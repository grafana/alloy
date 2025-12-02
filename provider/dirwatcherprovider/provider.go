// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package dirwatcherprovider // import "go.opentelemetry.io/collector/confmap/provider/dirwatcherprovider"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"go.opentelemetry.io/collector/confmap"
)

const schemeName = "dirwatch"

// debouncePeriod is the default debounce period for file change events.
// It can be overridden in tests using SetDebouncePeriodForTest.
var debouncePeriod = 5 * time.Second

// SetDebouncePeriodForTest sets the debounce period and returns a function to restore it.
// This is intended for use in tests only.
func SetDebouncePeriodForTest(d time.Duration) func() {
	old := debouncePeriod
	debouncePeriod = d
	return func() {
		debouncePeriod = old
	}
}

type provider struct {
	logger *zap.Logger

	// mu protects the watchers map
	mu       sync.Mutex
	watchers map[*dirWatcher]struct{}
}

// dirWatcher handles watching a single directory for changes
type dirWatcher struct {
	dirPath     string
	watcherFunc confmap.WatcherFunc
	logger      *zap.Logger

	fsWatcher  *fsnotify.Watcher
	shutdownCh chan struct{}
	wg         sync.WaitGroup

	// Debouncing state
	debounceTimer *time.Timer
	debounceMu    sync.Mutex

	// Ensure stop is only called once
	stopOnce sync.Once
}

// NewFactory returns a factory for a confmap.Provider that reads and merges
// configuration from all YAML files in a directory and watches for changes.
//
// This Provider supports the "dirwatch" scheme, and can be called with a "uri" that follows:
//
//	dirwatch-uri    = "dirwatch:" dir-path
//	dir-path        = [ drive-letter ] directory-path
//	drive-letter    = ALPHA ":"
//
// The "directory-path" can be relative or absolute, and it can be any OS supported format.
//
// Examples:
// `dirwatch:path/to/dir` - relative path (unix, windows)
// `dirwatch:/path/to/dir` - absolute path (unix, windows)
// `dirwatch:c:/path/to/dir` - absolute path including drive-letter (windows)
// `dirwatch:c:\path\to\dir` - absolute path including drive-letter (windows)
//
// The provider reads all *.yaml and *.yml files in the directory, merges them
// in alphabetical order, and watches for file changes to trigger configuration reloads.
func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(newProvider)
}

func newProvider(ps confmap.ProviderSettings) confmap.Provider {
	return &provider{
		logger:   ps.Logger,
		watchers: make(map[*dirWatcher]struct{}),
	}
}

func (p *provider) Retrieve(_ context.Context, uri string, watcherFunc confmap.WatcherFunc) (*confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, schemeName+":") {
		return nil, fmt.Errorf("%q uri is not supported by %q provider", uri, schemeName)
	}

	dirPath := uri[len(schemeName)+1:]
	if dirPath == "" {
		return nil, fmt.Errorf("directory path cannot be empty")
	}

	// Clean the path
	dirPath = filepath.Clean(dirPath)

	// List all YAML files in the directory
	files, err := listYAMLFiles(dirPath)
	if err != nil {
		return nil, fmt.Errorf("unable to list YAML files in directory %q: %w", dirPath, err)
	}

	// Merge all YAML files
	merged, err := mergeYAMLFiles(files)
	if err != nil {
		return nil, fmt.Errorf("unable to merge YAML files in directory %q: %w", dirPath, err)
	}

	// If watcherFunc is provided, set up file watching
	var closeFunc confmap.CloseFunc
	if watcherFunc != nil {
		dw, err := p.startWatching(dirPath, watcherFunc)
		if err != nil {
			return nil, fmt.Errorf("unable to start watching directory %q: %w", dirPath, err)
		}
		closeFunc = func(ctx context.Context) error {
			return p.stopWatching(dw)
		}
	}

	return confmap.NewRetrieved(merged, confmap.WithRetrievedClose(closeFunc))
}

func (*provider) Scheme() string {
	return schemeName
}

func (p *provider) Shutdown(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for dw := range p.watchers {
		dw.stop()
		delete(p.watchers, dw)
	}

	return nil
}

// startWatching creates and starts a new directory watcher
func (p *provider) startWatching(dirPath string, watcherFunc confmap.WatcherFunc) (*dirWatcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	if err := fsWatcher.Add(dirPath); err != nil {
		fsWatcher.Close()
		return nil, fmt.Errorf("failed to add directory to watcher: %w", err)
	}

	dw := &dirWatcher{
		dirPath:     dirPath,
		watcherFunc: watcherFunc,
		logger:      p.logger,
		fsWatcher:   fsWatcher,
		shutdownCh:  make(chan struct{}),
	}

	p.mu.Lock()
	p.watchers[dw] = struct{}{}
	p.mu.Unlock()

	dw.wg.Add(1)
	go dw.handleEvents()

	return dw, nil
}

// stopWatching stops a specific directory watcher
func (p *provider) stopWatching(dw *dirWatcher) error {
	p.mu.Lock()
	delete(p.watchers, dw)
	p.mu.Unlock()

	dw.stop()
	return nil
}

// handleEvents processes fsnotify events with debouncing
func (dw *dirWatcher) handleEvents() {
	defer dw.wg.Done()
	defer dw.fsWatcher.Close()

	for {
		select {
		case <-dw.shutdownCh:
			return

		case event, ok := <-dw.fsWatcher.Events:
			if !ok {
				return
			}
			dw.handleEvent(event)

		case err, ok := <-dw.fsWatcher.Errors:
			if !ok {
				return
			}
			dw.logger.Error("Error watching directory", zap.Error(err))
			// Notify watcher of error
			dw.watcherFunc(&confmap.ChangeEvent{Error: err})
		}
	}
}

// handleEvent processes a single fsnotify event
func (dw *dirWatcher) handleEvent(event fsnotify.Event) {
	// Only care about YAML files
	if !isYAMLFile(event.Name) {
		return
	}

	// Handle Create, Write, and Rename events
	if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove) {
		dw.scheduleNotify()
	}
}

// scheduleNotify schedules a debounced notification to the watcher
func (dw *dirWatcher) scheduleNotify() {
	dw.debounceMu.Lock()
	defer dw.debounceMu.Unlock()

	// If there's already a pending timer, reset it
	if dw.debounceTimer != nil {
		dw.debounceTimer.Stop()
	}

	dw.debounceTimer = time.AfterFunc(debouncePeriod, func() {
		// Check if we've been shut down
		select {
		case <-dw.shutdownCh:
			return
		default:
		}

		dw.watcherFunc(&confmap.ChangeEvent{})
	})
}

// stop gracefully stops the directory watcher (idempotent)
func (dw *dirWatcher) stop() {
	dw.stopOnce.Do(func() {
		// Signal shutdown
		close(dw.shutdownCh)

		// Cancel any pending debounce timer
		dw.debounceMu.Lock()
		if dw.debounceTimer != nil {
			dw.debounceTimer.Stop()
			dw.debounceTimer = nil
		}
		dw.debounceMu.Unlock()

		// Wait for the event handler goroutine to finish
		dw.wg.Wait()
	})
}

// isYAMLFile checks if a file path is a YAML file
func isYAMLFile(path string) bool {
	name := filepath.Base(path)

	// Skip hidden files
	if strings.HasPrefix(name, ".") {
		return false
	}

	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yaml" || ext == ".yml"
}

// listYAMLFiles returns a sorted list of all YAML files (*.yaml and *.yml) in the given directory.
// Hidden files (starting with '.') are excluded.
// The returned paths are full paths (directory + filename).
func listYAMLFiles(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip hidden files
		if strings.HasPrefix(name, ".") {
			continue
		}

		// Only include .yaml and .yml files
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, filepath.Join(dirPath, name))
		}
	}

	// Sort alphabetically for deterministic merge order
	sort.Strings(files)

	return files, nil
}

// mergeYAMLFiles reads and merges multiple YAML files into a single map.
// Files are merged in order, with later files overriding earlier ones for conflicting keys.
// If any file contains invalid YAML, an error is returned.
func mergeYAMLFiles(files []string) (map[string]any, error) {
	result := make(map[string]any)

	for _, file := range files {
		content, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			return nil, fmt.Errorf("unable to read file %q: %w", file, err)
		}

		var fileConfig map[string]any
		if err := yaml.Unmarshal(content, &fileConfig); err != nil {
			return nil, fmt.Errorf("unable to parse YAML file %q: %w", file, err)
		}

		// Merge this file's config into the result
		result = mergeMaps(result, fileConfig)
	}

	return result, nil
}

// mergeMaps recursively merges src into dst.
// For conflicting keys:
// - If both values are maps, they are merged recursively
// - Otherwise, the src value overwrites the dst value
func mergeMaps(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = make(map[string]any)
	}

	for key, srcVal := range src {
		if dstVal, exists := dst[key]; exists {
			// Both have this key - check if we can merge recursively
			srcMap, srcIsMap := srcVal.(map[string]any)
			dstMap, dstIsMap := dstVal.(map[string]any)

			if srcIsMap && dstIsMap {
				// Both are maps - merge recursively
				dst[key] = mergeMaps(dstMap, srcMap)
			} else {
				// Not both maps - src overwrites dst
				dst[key] = srcVal
			}
		} else {
			// Key doesn't exist in dst - just add it
			dst[key] = srcVal
		}
	}

	return dst
}
