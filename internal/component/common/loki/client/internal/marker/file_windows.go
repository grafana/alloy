//go:build windows

package marker

import (
	"log/slog"
	"sync"
)

// File tracks the last marked WAL segment on disk.
//
// On Windows an atomic rename fails with "Access is denied" if another handle
// is open on the destination file. File serializes reads and the atomic write
// with mu, so a concurrent read never overlaps the write.
type File struct {
	baseFile
	mu sync.Mutex
}

// NewFile creates a new marker File.
func NewFile(logger *slog.Logger, dir string) (*File, error) {
	base, err := newBaseFile(logger, dir)
	if err != nil {
		return nil, err
	}
	return &File{baseFile: base}, nil
}

// LastMarkedSegment implements wlog.Marker.
func (f *File) LastMarkedSegment() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.baseFile.LastMarkedSegment()
}

// MarkSegment stores segment as the last marked WAL segment.
func (f *File) MarkSegment(segment int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.baseFile.MarkSegment(segment)
}
