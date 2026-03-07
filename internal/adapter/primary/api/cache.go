// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"

	"github.com/danielgtaylor/huma/v2"

	frontend "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/frontend"
	"github.com/gboutry/sunbeam-watchtower/internal/app"
	pkg "github.com/gboutry/sunbeam-watchtower/internal/core/service/package"
	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
)

// --- Request / Response types ------------------------------------------------

// CacheEntry describes a single cached directory entry.
type CacheEntry struct {
	Name string `json:"name"`
	Size string `json:"size"`
}

// CacheSyncGitInput is the request body for POST /api/v1/cache/sync/git.
type CacheSyncGitInput struct {
	Body struct {
		Project string `json:"project" doc:"Sync only this project (empty = all)"`
	}
}

// CacheSyncGitOutput is the response for POST /api/v1/cache/sync/git.
type CacheSyncGitOutput struct {
	Body struct {
		Synced   int      `json:"synced" doc:"Number of repos synced"`
		Warnings []string `json:"warnings" doc:"Non-fatal warnings"`
	}
}

// CacheSyncUpstreamOutput is the response for POST /api/v1/cache/sync/upstream.
type CacheSyncUpstreamOutput struct {
	Body struct {
		Status string `json:"status" example:"ok"`
	}
}

// CacheDeleteInput is the request for DELETE /api/v1/cache/{type}.
type CacheDeleteInput struct {
	Type     string   `path:"type" doc:"Cache type to clear (git, packages-index, upstream-repos, bugs, excuses, releases)"`
	Project  string   `query:"project" required:"false" doc:"Clear only this project (git/bugs types only)"`
	Trackers []string `query:"tracker" required:"false" doc:"Clear only these excuses trackers (excuses type only)"`
}

// CacheDeleteOutput is the response for DELETE /api/v1/cache/{type}.
type CacheDeleteOutput struct {
	Body struct {
		Status string `json:"status" example:"ok"`
	}
}

// CacheSyncBugsInput is the request body for POST /api/v1/cache/sync/bugs.
type CacheSyncBugsInput struct {
	Body struct {
		Project string `json:"project" doc:"Sync only this project (empty = all configured)"`
	}
}

// CacheSyncBugsOutput is the response for POST /api/v1/cache/sync/bugs.
type CacheSyncBugsOutput struct {
	Body struct {
		Synced int `json:"synced" doc:"Number of bug tasks synced"`
	}
}

// CacheSyncExcusesInput is the request body for POST /api/v1/cache/sync/excuses.
type CacheSyncExcusesInput struct {
	Body struct {
		Trackers []string `json:"trackers,omitempty" required:"false" doc:"Excuses trackers to sync (default: all configured trackers)"`
	}
}

// CacheSyncExcusesOutput is the response for POST /api/v1/cache/sync/excuses.
type CacheSyncExcusesOutput struct {
	Body struct {
		Status string `json:"status" example:"ok"`
	}
}

// CacheSyncReleasesOutput is the response for POST /api/v1/cache/sync/releases.
type CacheSyncReleasesOutput struct {
	Body dto.ReleaseSyncResult
}

// CacheStatusOutput is the response for GET /api/v1/cache/status.
type CacheStatusOutput struct {
	Body struct {
		Git struct {
			Directory string       `json:"directory"`
			Repos     []CacheEntry `json:"repos"`
		} `json:"git"`
		Packages struct {
			Directory string            `json:"directory"`
			Sources   []dto.CacheStatus `json:"sources"`
			Error     string            `json:"error,omitempty"`
		} `json:"packages"`
		Upstream struct {
			Directory string       `json:"directory"`
			Repos     []CacheEntry `json:"repos"`
		} `json:"upstream"`
		Bugs struct {
			Directory string               `json:"directory"`
			Entries   []dto.BugCacheStatus `json:"entries"`
			Error     string               `json:"error,omitempty"`
		} `json:"bugs"`
		Excuses struct {
			Directory string                   `json:"directory"`
			Entries   []dto.ExcusesCacheStatus `json:"entries"`
			Error     string                   `json:"error,omitempty"`
		} `json:"excuses"`
		Releases struct {
			Directory string                   `json:"directory"`
			Entries   []dto.ReleaseCacheStatus `json:"entries"`
			Error     string                   `json:"error,omitempty"`
		} `json:"releases"`
	}
}

// --- Route registration ------------------------------------------------------

// RegisterCacheAPI registers all cache-related endpoints on the given huma API.
func RegisterCacheAPI(api huma.API, application *app.App) {
	// POST /api/v1/cache/sync/git
	huma.Register(api, huma.Operation{
		OperationID: "cache-sync-git",
		Method:      http.MethodPost,
		Path:        "/api/v1/cache/sync/git",
		Summary:     "Sync git caches for configured projects",
		Tags:        []string{"cache"},
	}, func(ctx context.Context, input *CacheSyncGitInput) (*CacheSyncGitOutput, error) {
		cache, err := application.GitCache()
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to open git cache: %v", err))
		}

		cfg := application.Config
		if cfg == nil {
			return nil, huma.Error500InternalServerError("no configuration loaded")
		}

		forgeClients, err := application.BuildForgeClients()
		if err != nil {
			application.Logger.Warn("could not build forge clients for MR sync", "error", err)
		}

		var warnings []string
		synced := 0
		project := input.Body.Project

		for _, proj := range cfg.Projects {
			if project != "" && proj.Name != project {
				continue
			}

			cloneURL, err := proj.Code.CloneURL()
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("%s: %v", proj.Name, err))
				continue
			}

			refSpecs := app.MRRefSpecs(proj.Code.Forge)
			var syncOpts *dto.SyncOptions
			if len(refSpecs) > 0 {
				syncOpts = &dto.SyncOptions{ExtraRefSpecs: refSpecs}
			}

			if _, err := cache.EnsureRepo(ctx, cloneURL, syncOpts); err != nil {
				warnings = append(warnings, fmt.Sprintf("%s: %v", proj.Name, err))
			} else {
				synced++
			}

			if fc, ok := forgeClients[proj.Name]; ok {
				mrs, mrErr := fc.Forge.ListMergeRequests(ctx, fc.ProjectID, forge.ListMergeRequestsOpts{})
				if mrErr != nil {
					application.Logger.Warn("MR metadata fetch failed", "project", proj.Name, "error", mrErr)
				} else if len(mrs) > 0 {
					metadata := app.ConvertToMRMetadata(mrs, proj.Code.Forge)
					if storeErr := cache.StoreMRMetadata(cloneURL, metadata); storeErr != nil {
						application.Logger.Warn("storing MR metadata failed", "project", proj.Name, "error", storeErr)
					}
				}
			}
		}

		out := &CacheSyncGitOutput{}
		out.Body.Synced = synced
		out.Body.Warnings = warnings
		return out, nil
	})

	// POST /api/v1/cache/sync/upstream
	huma.Register(api, huma.Operation{
		OperationID: "cache-sync-upstream",
		Method:      http.MethodPost,
		Path:        "/api/v1/cache/sync/upstream",
		Summary:     "Sync upstream repos (releases, requirements)",
		Tags:        []string{"cache"},
	}, func(ctx context.Context, _ *struct{}) (*CacheSyncUpstreamOutput, error) {
		cfg := application.Config
		if cfg == nil {
			return nil, huma.Error500InternalServerError("no configuration loaded")
		}
		if cfg.Packages.Upstream == nil {
			out := &CacheSyncUpstreamOutput{}
			out.Body.Status = "skipped: upstream not configured"
			return out, nil
		}

		upDir, err := app.UpstreamCacheDir()
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("resolving upstream cache dir: %v", err))
		}
		if err := os.MkdirAll(upDir, 0o755); err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("creating upstream cache dir: %v", err))
		}

		repos := map[string]string{}
		if u := cfg.Packages.Upstream.ReleasesRepo; u != "" {
			repos["releases"] = u
		}
		if u := cfg.Packages.Upstream.RequirementsRepo; u != "" {
			repos["requirements"] = u
		}

		for label, repoURL := range repos {
			localPath := app.UpstreamRepoPath(upDir, repoURL)
			if _, err := os.Stat(localPath); err == nil {
				gitCmd := exec.CommandContext(ctx, "git", "-C", localPath, "fetch", "--all")
				if err := gitCmd.Run(); err != nil {
					application.Logger.Warn("fetch upstream failed", "repo", label, "error", err)
				}
			} else {
				gitCmd := exec.CommandContext(ctx, "git", "clone", "--bare", repoURL, localPath)
				if err := gitCmd.Run(); err != nil {
					application.Logger.Warn("clone upstream failed", "repo", label, "error", err)
				}
			}
		}

		out := &CacheSyncUpstreamOutput{}
		out.Body.Status = "ok"
		return out, nil
	})

	// POST /api/v1/cache/sync/bugs
	huma.Register(api, huma.Operation{
		OperationID: "cache-sync-bugs",
		Method:      http.MethodPost,
		Path:        "/api/v1/cache/sync/bugs",
		Summary:     "Sync bug caches for configured projects",
		Tags:        []string{"cache"},
	}, func(ctx context.Context, input *CacheSyncBugsInput) (*CacheSyncBugsOutput, error) {
		synced, err := application.SyncBugCache(ctx, input.Body.Project)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("bug cache sync failed: %v", err))
		}
		out := &CacheSyncBugsOutput{}
		out.Body.Synced = synced
		return out, nil
	})

	// POST /api/v1/cache/sync/excuses
	huma.Register(api, huma.Operation{
		OperationID: "cache-sync-excuses",
		Method:      http.MethodPost,
		Path:        "/api/v1/cache/sync/excuses",
		Summary:     "Sync package migration excuses caches",
		Tags:        []string{"cache"},
	}, func(ctx context.Context, input *CacheSyncExcusesInput) (*CacheSyncExcusesOutput, error) {
		trackers := input.Body.Trackers
		sources := application.ExcusesSources()
		if err := dto.ValidateExcusesTrackers(sources, trackers); err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		sources = dto.FilterExcusesSources(sources, trackers)
		if len(sources) == 0 {
			return nil, huma.Error400BadRequest("no excuses trackers selected")
		}

		cache, err := application.ExcusesCache()
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to open excuses cache: %v", err))
		}
		svc := pkg.NewExcusesService(cache, application.Logger)
		if err := svc.UpdateCache(ctx, sources); err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("excuses cache sync failed: %v", err))
		}
		out := &CacheSyncExcusesOutput{}
		out.Body.Status = "ok"
		return out, nil
	})

	// POST /api/v1/cache/sync/releases
	huma.Register(api, huma.Operation{
		OperationID: "cache-sync-releases",
		Method:      http.MethodPost,
		Path:        "/api/v1/cache/sync/releases",
		Summary:     "Sync published snap and charm release caches",
		Tags:        []string{"cache", "releases"},
	}, func(ctx context.Context, _ *struct{}) (*CacheSyncReleasesOutput, error) {
		result, err := frontend.NewServerFacade(application).Releases().SyncCache(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("release cache sync failed: %v", err))
		}
		out := &CacheSyncReleasesOutput{}
		out.Body = *result
		return out, nil
	})

	// DELETE /api/v1/cache/{type}
	huma.Register(api, huma.Operation{
		OperationID: "cache-delete",
		Method:      http.MethodDelete,
		Path:        "/api/v1/cache/{type}",
		Summary:     "Clear a specific cache type",
		Tags:        []string{"cache"},
	}, func(ctx context.Context, input *CacheDeleteInput) (*CacheDeleteOutput, error) {
		switch input.Type {
		case "git":
			cache, err := application.GitCache()
			if err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to open git cache: %v", err))
			}
			if input.Project == "" {
				if err := cache.RemoveAll(); err != nil {
					return nil, huma.Error500InternalServerError(fmt.Sprintf("clearing git cache: %v", err))
				}
			} else {
				cfg := application.Config
				if cfg == nil {
					return nil, huma.Error500InternalServerError("no configuration loaded")
				}
				found := false
				for _, proj := range cfg.Projects {
					if proj.Name != input.Project {
						continue
					}
					found = true
					cloneURL, err := proj.Code.CloneURL()
					if err != nil {
						return nil, huma.Error500InternalServerError(fmt.Sprintf("%s: %v", proj.Name, err))
					}
					if err := cache.Remove(cloneURL); err != nil {
						return nil, huma.Error500InternalServerError(fmt.Sprintf("removing git cache: %v", err))
					}
					break
				}
				if !found {
					return nil, huma.Error404NotFound(fmt.Sprintf("project %q not found in config", input.Project))
				}
			}

		case "packages-index":
			cache, err := application.DistroCache()
			if err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to open distro cache: %v", err))
			}
			if err := cache.RemoveAll(); err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("clearing packages index: %v", err))
			}

		case "upstream-repos":
			upDir, err := app.UpstreamCacheDir()
			if err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("resolving upstream cache dir: %v", err))
			}
			if err := os.RemoveAll(upDir); err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("removing upstream cache: %v", err))
			}

		case "bugs":
			cache, err := application.BugCache()
			if err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to open bug cache: %v", err))
			}
			if input.Project == "" {
				if err := cache.RemoveAll(ctx); err != nil {
					return nil, huma.Error500InternalServerError(fmt.Sprintf("clearing bug cache: %v", err))
				}
			} else {
				cfg := application.Config
				if cfg == nil {
					return nil, huma.Error500InternalServerError("no configuration loaded")
				}
				found := false
				for _, proj := range cfg.Projects {
					for _, b := range proj.Bugs {
						if b.Project == input.Project {
							found = true
							forgeType := app.ForgeTypeFromConfig(b.Forge)
							if err := cache.Remove(ctx, forgeType, b.Project); err != nil {
								return nil, huma.Error500InternalServerError(fmt.Sprintf("removing bug cache: %v", err))
							}
							break
						}
					}
					if found {
						break
					}
				}
				if !found {
					return nil, huma.Error404NotFound(fmt.Sprintf("bug project %q not found in config", input.Project))
				}
			}

		case "excuses":
			cache, err := application.ExcusesCache()
			if err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to open excuses cache: %v", err))
			}
			if len(input.Trackers) == 0 {
				if err := cache.RemoveAll(); err != nil {
					return nil, huma.Error500InternalServerError(fmt.Sprintf("clearing excuses cache: %v", err))
				}
			} else {
				if err := dto.ValidateExcusesTrackers(application.ExcusesSources(), input.Trackers); err != nil {
					return nil, huma.Error400BadRequest(err.Error())
				}
				for _, tracker := range input.Trackers {
					if err := cache.Remove(tracker); err != nil {
						return nil, huma.Error500InternalServerError(fmt.Sprintf("removing excuses cache for %s: %v", tracker, err))
					}
				}
			}

		case "releases":
			cache, err := application.ReleaseCache()
			if err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to open release cache: %v", err))
			}
			if err := cache.RemoveAll(); err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("clearing release cache: %v", err))
			}

		default:
			return nil, huma.Error400BadRequest(
				fmt.Sprintf("unknown cache type %q (valid: git, packages-index, upstream-repos, bugs, excuses, releases)", input.Type))
		}

		out := &CacheDeleteOutput{}
		out.Body.Status = "ok"
		return out, nil
	})

	// GET /api/v1/cache/status
	huma.Register(api, huma.Operation{
		OperationID: "cache-status",
		Method:      http.MethodGet,
		Path:        "/api/v1/cache/status",
		Summary:     "Full cache status (git + packages + upstream)",
		Tags:        []string{"cache"},
	}, func(ctx context.Context, _ *struct{}) (*CacheStatusOutput, error) {
		out := &CacheStatusOutput{}

		// Git repos status.
		gitCache, err := application.GitCache()
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to open git cache: %v", err))
		}
		cacheDir := gitCache.CacheDir()
		out.Body.Git.Directory = cacheDir
		if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
			_ = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() || !isBareGitRepo(path) {
					return nil
				}
				rel, _ := filepath.Rel(cacheDir, path)
				size, _ := dirSize(path)
				out.Body.Git.Repos = append(out.Body.Git.Repos, CacheEntry{Name: rel, Size: formatSize(size)})
				return filepath.SkipDir
			})
		}

		// Packages index status.
		distroCache, err := application.DistroCache()
		if err != nil {
			out.Body.Packages.Error = err.Error()
		} else {
			out.Body.Packages.Directory = distroCache.CacheDir()
			svc := pkg.NewService(distroCache, application.Logger)
			statuses, sErr := svc.CacheStatus()
			if sErr != nil {
				out.Body.Packages.Error = sErr.Error()
			} else {
				out.Body.Packages.Sources = statuses
			}
		}

		// Upstream repos status.
		upDir, err := app.UpstreamCacheDir()
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("resolving upstream cache dir: %v", err))
		}
		out.Body.Upstream.Directory = upDir
		if _, err := os.Stat(upDir); !os.IsNotExist(err) {
			entries, rErr := os.ReadDir(upDir)
			if rErr == nil {
				for _, e := range entries {
					if !e.IsDir() {
						continue
					}
					size, _ := dirSize(filepath.Join(upDir, e.Name()))
					out.Body.Upstream.Repos = append(out.Body.Upstream.Repos, CacheEntry{Name: e.Name(), Size: formatSize(size)})
				}
			}
		}

		// Bug cache status.
		bugCache, err := application.BugCache()
		if err != nil {
			out.Body.Bugs.Error = err.Error()
		} else {
			out.Body.Bugs.Directory = bugCache.CacheDir()
			bugStatuses, bErr := bugCache.Status(ctx)
			if bErr != nil {
				out.Body.Bugs.Error = bErr.Error()
			} else {
				out.Body.Bugs.Entries = bugStatuses
			}
		}

		// Excuses cache status.
		excusesCache, err := application.ExcusesCache()
		if err != nil {
			out.Body.Excuses.Error = err.Error()
		} else {
			out.Body.Excuses.Directory = excusesCache.CacheDir()
			svc := pkg.NewExcusesService(excusesCache, application.Logger)
			statuses, sErr := svc.CacheStatus()
			if sErr != nil {
				out.Body.Excuses.Error = sErr.Error()
			} else {
				out.Body.Excuses.Entries = statuses
			}
		}

		// Published release cache status.
		releaseCache, err := application.ReleaseCache()
		if err != nil {
			out.Body.Releases.Error = err.Error()
		} else {
			out.Body.Releases.Directory = releaseCache.CacheDir()
			statuses, sErr := frontend.NewServerFacade(application).Releases().CacheStatus(ctx)
			if sErr != nil {
				out.Body.Releases.Error = sErr.Error()
			} else {
				out.Body.Releases.Entries = statuses
			}
		}

		return out, nil
	})
}

// --- Helpers -----------------------------------------------------------------

func isBareGitRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, "HEAD"))
	return err == nil
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		size += info.Size()
		return nil
	})
	return size, err
}

func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
