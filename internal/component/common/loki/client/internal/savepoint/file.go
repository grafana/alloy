package savepoint

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/natefinch/atomic"
)

const (
	folderName = "remote"
	fileName   = "savepoint.json"

	folderMode os.FileMode = 0o700
)

// File holds the savepoints of every endpoint of a single loki.write component in one file.
type File struct {
	logger *slog.Logger
	path   string

	initialLoad sync.Once
	mut         sync.Mutex
	entries     map[string]entry
}

// NewFile returns the savepoint file in dir, creating its folder if needed. Keys
// without a stored savepoint report -1.
func NewFile(logger *slog.Logger, dir string) (*File, error) {
	folder := filepath.Join(dir, folderName)
	if err := os.MkdirAll(folder, folderMode); err != nil {
		return nil, fmt.Errorf("error creating savepoint folder %q: %w", folder, err)
	}

	return &File{
		logger: logger,
		path:   filepath.Join(folder, fileName),
	}, nil
}

// LastStoredSegment returns key's last consumed segment, or -1 if it has
// none.
func (f *File) LastStoredSegment(key string) int {
	f.mut.Lock()
	defer f.mut.Unlock()

	f.ensureLoaded()

	e, ok := f.entries[key]
	if !ok {
		return noSegment
	}
	return e.Segment
}

// StoreSegment stores segment as key's last consumed segment.
func (f *File) StoreSegment(key string, segment int) {
	f.mut.Lock()
	defer f.mut.Unlock()

	f.ensureLoaded()

	f.entries[key] = entry{Segment: segment}

	// A savepoint that cannot be persisted costs a replay on the next restart,
	// so it must not stop the endpoint from sending.
	if err := f.write(); err != nil {
		f.logger.Error("could not update savepoint file", "file", f.path, "err", err)
		return
	}

	f.logger.Debug("updated savepoint file", "file", f.path, "endpoint", key, "segment", segment)
}

// ensureLoaded reads the stored savepoints on first use. Callers must hold mut.
func (f *File) ensureLoaded() {
	f.initialLoad.Do(func() {
		file, err := os.ReadFile(f.path)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				f.logger.Error("could not read savepoint file, continuing without savepoints", "file", f.path, "err", err)
			}
			f.entries = make(map[string]entry)
			return
		}

		entries, err := decode(file)
		if err != nil {
			f.logger.Error("could not decode savepoint file, continuing without savepoints", "file", f.path, "err", err)
			f.entries = make(map[string]entry)
			return
		}
		f.entries = entries
	})
}

// write persists all savepoints. Callers must hold mut.
func (f *File) write() error {
	bs, err := encode(f.entries)
	if err != nil {
		return fmt.Errorf("error encoding savepoints: %w", err)
	}

	if err := atomic.WriteFile(f.path, bytes.NewReader(bs)); err != nil {
		return fmt.Errorf("error writing savepoint file %q: %w", f.path, err)
	}

	return nil
}
