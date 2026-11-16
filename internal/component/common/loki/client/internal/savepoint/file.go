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

	// beforeFirstSegment is the saved segment of an endpoint that starts with the first segment of the WAL. It can not
	// be -1, which is reported for an endpoint without a savepoint.
	beforeFirstSegment = -2
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

// StoreStart records that key starts reading the WAL at segment, unless key already has a savepoint. Without it an
// endpoint that was never marked has no savepoint and a restart would start at the newest segment, skipping what was
// written before the crash. A start at the first segment is stored as beforeFirstSegment, since -1 means no savepoint.
func (f *File) StoreStart(key string, segment int) {
	f.mut.Lock()
	defer f.mut.Unlock()

	f.ensureLoaded()

	if _, ok := f.entries[key]; ok {
		return
	}

	last := segment - 1
	if last < 0 {
		last = beforeFirstSegment
	}
	f.entries[key] = entry{Segment: last}

	if err := f.write(); err != nil {
		f.logger.Error("could not update savepoint file", "file", f.path, "err", err)
	}
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
