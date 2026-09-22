//go:build !windows

package marker

import "log/slog"

// File tracks the last marked WAL segment on disk.
//
// On non-Windows systems an atomic rename works while other handles are open
// on the destination file. Reads and writes need no serialization, so File
// uses baseFile directly.
type File struct {
	baseFile
}

// NewFile creates a new marker File.
func NewFile(logger *slog.Logger, dir string) (*File, error) {
	base, err := newBaseFile(logger, dir)
	if err != nil {
		return nil, err
	}
	return &File{baseFile: base}, nil
}
