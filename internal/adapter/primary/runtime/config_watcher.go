// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"io"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ConfigWatcher watches a configuration file for changes and calls a reload
// callback when the file is modified. It watches the parent directory to handle
// editors that delete and recreate files (e.g. vim). Events are debounced to
// coalesce rapid editor save patterns.
type ConfigWatcher struct {
	watcher *fsnotify.Watcher
	done    chan struct{}
}

// NewConfigWatcher creates a ConfigWatcher that monitors the file at path and
// calls reload when the file changes. The reload function receives the file
// path as its argument. Pass a nil logger to discard log output.
func NewConfigWatcher(path string, reload func(string) error, logger *slog.Logger) (*ConfigWatcher, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(absPath)
	base := filepath.Base(absPath)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := watcher.Add(dir); err != nil {
		_ = watcher.Close()
		return nil, err
	}

	cw := &ConfigWatcher{
		watcher: watcher,
		done:    make(chan struct{}),
	}

	go cw.loop(absPath, base, reload, logger)
	return cw, nil
}

func (cw *ConfigWatcher) loop(absPath, base string, reload func(string) error, logger *slog.Logger) {
	defer close(cw.done)

	const debounce = 200 * time.Millisecond
	var timer *time.Timer

	for {
		select {
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}
			if filepath.Base(event.Name) != base {
				continue
			}
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) && !event.Has(fsnotify.Rename) {
				continue
			}

			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounce, func() {
				logger.Info("config file changed, reloading", "path", absPath)
				if err := reload(absPath); err != nil {
					logger.Error("config reload failed", "error", err)
				}
			})

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			logger.Error("config watcher error", "error", err)
		}
	}
}

// Stop stops the config watcher and waits for the event loop to finish.
func (cw *ConfigWatcher) Stop() {
	_ = cw.watcher.Close()
	<-cw.done
}
