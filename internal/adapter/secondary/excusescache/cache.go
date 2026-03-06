// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package excusescache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	"github.com/ulikunitz/xz"
	"go.etcd.io/bbolt"
)

var _ port.ExcusesCache = (*Cache)(nil)

const metaBucket = "meta"

// Cache implements port.ExcusesCache using bbolt for normalized records and raw files on disk.
type Cache struct {
	baseDir string
	db      *bbolt.DB
	client  *http.Client
	logger  *slog.Logger
}

type trackerMeta struct {
	URL         string    `json:"url"`
	LastUpdated time.Time `json:"last_updated"`
}

// NewCache creates a new excuses cache rooted at baseDir.
func NewCache(baseDir string, logger *slog.Logger) (*Cache, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating excuses cache dir: %w", err)
	}
	dbPath := filepath.Join(baseDir, "excuses.db")
	db, err := bbolt.Open(dbPath, 0o600, &bbolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("opening excuses cache db: %w", err)
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Cache{
		baseDir: baseDir,
		db:      db,
		client:  &http.Client{Timeout: 2 * time.Minute},
		logger:  logger,
	}, nil
}

// Close releases resources held by the cache.
func (c *Cache) Close() error { return c.db.Close() }

// CacheDir returns the base cache directory.
func (c *Cache) CacheDir() string { return c.baseDir }

// RemoveAll deletes the entire excuses cache.
func (c *Cache) RemoveAll() error {
	if err := c.db.Close(); err != nil {
		return fmt.Errorf("closing db before removal: %w", err)
	}
	return os.RemoveAll(c.baseDir)
}

// Remove deletes cached data for a specific tracker.
func (c *Cache) Remove(tracker string) error {
	return c.db.Update(func(tx *bbolt.Tx) error {
		_ = tx.DeleteBucket(recordsBucketName(tracker))
		b := tx.Bucket([]byte(metaBucket))
		if b != nil {
			_ = b.Delete([]byte(tracker))
		}
		return os.RemoveAll(c.rawDir(tracker))
	})
}

// Update downloads, parses, and indexes one excuses tracker.
func (c *Cache) Update(ctx context.Context, source dto.ExcusesSource) error {
	c.logger.Info("downloading excuses", "tracker", source.Tracker, "url", source.URL)
	rawData, err := c.download(ctx, source.URL)
	if err != nil {
		return err
	}
	if err := c.storeRaw(source, rawData); err != nil {
		return err
	}

	decoded, err := decodeRaw(source, rawData)
	if err != nil {
		return err
	}
	excuses, err := parseExcusesYAML(decoded, source)
	if err != nil {
		return err
	}
	if source.TeamURL != "" {
		teamSource := dto.ExcusesSource{
			Tracker:     source.Tracker,
			URL:         source.TeamURL,
			Compression: source.TeamCompression,
		}
		c.logger.Info("downloading excuses team mapping", "tracker", source.Tracker, "url", teamSource.URL)
		teamRaw, err := c.download(ctx, teamSource.URL)
		if err != nil {
			return err
		}
		if err := c.storeRaw(teamSource, teamRaw); err != nil {
			return err
		}
		teamDecoded, err := decodeRaw(teamSource, teamRaw)
		if err != nil {
			return err
		}
		exactTeams, packageTeams, err := parseUbuntuExcusesByTeamYAML(teamDecoded)
		if err != nil {
			return err
		}
		applyExcuseTeams(excuses, exactTeams, packageTeams)
	}

	if err := c.db.Update(func(tx *bbolt.Tx) error {
		_ = tx.DeleteBucket(recordsBucketName(source.Tracker))
		b, err := tx.CreateBucket(recordsBucketName(source.Tracker))
		if err != nil {
			return fmt.Errorf("creating records bucket for %s: %w", source.Tracker, err)
		}
		for _, excuse := range excuses {
			data, err := marshalJSON(excuse)
			if err != nil {
				return fmt.Errorf("marshalling excuse %s: %w", excuse.Package, err)
			}
			if err := b.Put([]byte(recordKey(excuse.Package, excuse.Version)), data); err != nil {
				return fmt.Errorf("storing excuse %s: %w", excuse.Package, err)
			}
		}

		metaBucket, err := tx.CreateBucketIfNotExists([]byte(metaBucket))
		if err != nil {
			return fmt.Errorf("creating meta bucket: %w", err)
		}
		meta, err := marshalJSON(trackerMeta{URL: source.URL, LastUpdated: time.Now().UTC()})
		if err != nil {
			return fmt.Errorf("marshalling tracker meta: %w", err)
		}
		return metaBucket.Put([]byte(source.Tracker), meta)
	}); err != nil {
		return err
	}

	return nil
}

func (c *Cache) download(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating excuses request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading excuses: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading excuses: HTTP %d", resp.StatusCode)
	}

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading excuses response: %w", err)
	}
	return rawData, nil
}

// List returns excuses matching the given query options.
func (c *Cache) List(_ context.Context, opts dto.ExcuseQueryOpts) ([]dto.PackageExcuseSummary, error) {
	var nameRe *regexp.Regexp
	var err error
	if opts.Name != "" {
		nameRe, err = regexp.Compile("(?i)" + opts.Name)
		if err != nil {
			return nil, fmt.Errorf("invalid name regex: %w", err)
		}
	}

	trackers := opts.Trackers
	if len(trackers) == 0 {
		for _, source := range dto.KnownExcusesSources() {
			trackers = append(trackers, source.Tracker)
		}
	}

	var results []dto.PackageExcuseSummary
	err = c.db.View(func(tx *bbolt.Tx) error {
		for _, tracker := range trackers {
			b := tx.Bucket(recordsBucketName(tracker))
			if b == nil {
				continue
			}
			if err := b.ForEach(func(_, v []byte) error {
				var excuse dto.PackageExcuse
				if err := unmarshalJSON(v, &excuse); err != nil {
					return err
				}
				if !matchesQuery(excuse, opts, nameRe) {
					return nil
				}
				results = append(results, excuse.PackageExcuseSummary)
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing excuses: %w", err)
	}

	sort.Slice(results, func(i, j int) bool {
		if opts.Reverse {
			if results[i].AgeDays == results[j].AgeDays {
				if results[i].Tracker == results[j].Tracker {
					return results[i].Package < results[j].Package
				}
				return results[i].Tracker < results[j].Tracker
			}
			return results[i].AgeDays > results[j].AgeDays
		}
		if results[i].AgeDays == results[j].AgeDays {
			if results[i].Tracker == results[j].Tracker {
				return results[i].Package < results[j].Package
			}
			return results[i].Tracker < results[j].Tracker
		}
		return results[i].AgeDays < results[j].AgeDays
	})

	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}
	return results, nil
}

// Get returns one excuse by tracker, package, and optional version.
func (c *Cache) Get(_ context.Context, tracker, name, version string) (*dto.PackageExcuse, error) {
	trackers := []string{tracker}
	if tracker == "" {
		sources := dto.KnownExcusesSources()
		trackers = make([]string, 0, len(sources))
		for _, source := range sources {
			trackers = append(trackers, source.Tracker)
		}
	}

	var found *dto.PackageExcuse
	err := c.db.View(func(tx *bbolt.Tx) error {
		for _, currentTracker := range trackers {
			b := tx.Bucket(recordsBucketName(currentTracker))
			if b == nil {
				continue
			}
			if version != "" {
				v := b.Get([]byte(recordKey(name, version)))
				if v == nil {
					continue
				}
				var excuse dto.PackageExcuse
				if err := unmarshalJSON(v, &excuse); err != nil {
					return err
				}
				found = &excuse
				return nil
			}
			if err := b.ForEach(func(_, v []byte) error {
				var excuse dto.PackageExcuse
				if err := unmarshalJSON(v, &excuse); err != nil {
					return err
				}
				if excuse.Package != name {
					return nil
				}
				found = &excuse
				return nil
			}); err != nil {
				return err
			}
			if found != nil {
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("getting excuse %s: %w", name, err)
	}
	if found == nil {
		if tracker != "" {
			return nil, fmt.Errorf("excuse %s not found in %s tracker", name, tracker)
		}
		return nil, fmt.Errorf("excuse %s not found", name)
	}
	return found, nil
}

// Status reports one status entry per cached tracker.
func (c *Cache) Status() ([]dto.ExcusesCacheStatus, error) {
	var statuses []dto.ExcusesCacheStatus
	err := c.db.View(func(tx *bbolt.Tx) error {
		meta := tx.Bucket([]byte(metaBucket))
		for _, source := range dto.KnownExcusesSources() {
			status := dto.ExcusesCacheStatus{
				Tracker:  source.Tracker,
				URL:      source.URL,
				DiskSize: dirSize(c.rawDir(source.Tracker)),
			}
			if b := tx.Bucket(recordsBucketName(source.Tracker)); b != nil {
				_ = b.ForEach(func(_, _ []byte) error {
					status.EntryCount++
					return nil
				})
			}
			if meta != nil {
				if data := meta.Get([]byte(source.Tracker)); data != nil {
					var tracker trackerMeta
					if err := unmarshalJSON(data, &tracker); err != nil {
						return err
					}
					status.LastUpdated = tracker.LastUpdated
					if tracker.URL != "" {
						status.URL = tracker.URL
					}
				}
			}
			statuses = append(statuses, status)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading excuses status: %w", err)
	}
	return statuses, nil
}

func matchesQuery(excuse dto.PackageExcuse, opts dto.ExcuseQueryOpts, nameRe *regexp.Regexp) bool {
	if nameRe != nil && !nameRe.MatchString(excuse.Package) {
		return false
	}
	if opts.Component != "" && excuse.Component != opts.Component {
		return false
	}
	if opts.Team != "" && !strings.EqualFold(excuse.Team, opts.Team) {
		return false
	}
	if opts.FTBFS && !excuse.FTBFS {
		return false
	}
	if opts.Autopkgtest && len(excuse.Autopkgtests) == 0 {
		return false
	}
	if opts.BlockedBy != "" {
		found := false
		for _, pkg := range excuse.BlockedBy {
			if pkg == opts.BlockedBy {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if opts.Bugged && excuse.Bug == "" {
		return false
	}
	if opts.MinAge > 0 && excuse.AgeDays < opts.MinAge {
		return false
	}
	if opts.MaxAge > 0 && excuse.AgeDays > opts.MaxAge {
		return false
	}
	return true
}

func decodeRaw(source dto.ExcusesSource, data []byte) ([]byte, error) {
	switch source.Compression {
	case "", dto.ExcusesCompressionNone:
		return data, nil
	case dto.ExcusesCompressionXZ:
		reader, err := xz.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decompressing xz excuses: %w", err)
		}
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("reading decompressed excuses: %w", err)
		}
		return decoded, nil
	default:
		return nil, fmt.Errorf("unsupported excuses compression %q", source.Compression)
	}
}

func (c *Cache) storeRaw(source dto.ExcusesSource, data []byte) error {
	dir := c.rawDir(source.Tracker)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating raw excuses dir: %w", err)
	}
	path := filepath.Join(dir, filepath.Base(source.URL))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing raw excuses file: %w", err)
	}
	return nil
}

func (c *Cache) rawDir(tracker string) string {
	return filepath.Join(c.baseDir, "raw", tracker)
}

func recordsBucketName(tracker string) []byte {
	return []byte("records:" + tracker)
}

func recordKey(pkg, version string) string {
	return pkg + "\x00" + version
}

func dirSize(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info == nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}
