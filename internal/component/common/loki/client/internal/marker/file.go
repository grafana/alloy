package marker

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/natefinch/atomic"
)

const (
	markerFolderName = "remote"
	markerFileName   = "segment_marker"

	markerFolderMode        os.FileMode = 0o700
	markerWindowsFolderMode os.FileMode = 0o777
	markerFileMode          os.FileMode = 0o600
	markerWindowsFileMode   os.FileMode = 0o666
)

// baseFile holds the marker file logic shared by all platforms. Each platform
// defines a File type that embeds baseFile. On Windows, File adds a mutex to
// serialize file access. See file_windows.go and file_unix.go.
type baseFile struct {
	logger                    *slog.Logger
	lastMarkedSegmentDir      string
	lastMarkedSegmentFilePath string
}

func newBaseFile(logger *slog.Logger, dir string) (baseFile, error) {
	markerDir := filepath.Join(dir, markerFolderName)
	// attempt to create dir if doesn't exist
	if err := os.MkdirAll(markerDir, markerFolderMode); err != nil {
		return baseFile{}, fmt.Errorf("error creating segment marker folder %q: %w", markerDir, err)
	}

	return baseFile{
		logger:                    logger,
		lastMarkedSegmentDir:      filepath.Join(markerDir),
		lastMarkedSegmentFilePath: filepath.Join(markerDir, markerFileName),
	}, nil
}

// LastMarkedSegment implements wlog.Marker.
func (f *baseFile) LastMarkedSegment() int {
	bs, err := os.ReadFile(f.lastMarkedSegmentFilePath)
	if os.IsNotExist(err) {
		f.logger.Warn("marker segment file does not exist", "file", f.lastMarkedSegmentFilePath)
		return -1
	} else if err != nil {
		f.logger.Error("could not access segment marker file", "file", f.lastMarkedSegmentFilePath, "err", err)
		return -1
	}

	savedSegment, err := decodeV1(bs)
	if err != nil {
		f.logger.Error("could not decode segment marker file", "file", f.lastMarkedSegmentFilePath, "err", err)
		return -1
	}

	return int(savedSegment)
}

// MarkSegment stores segment as the last marked WAL segment.
func (f *baseFile) MarkSegment(segment int) {
	if err := f.atomicallyWriteMarker(encodeV1(uint64(segment))); err != nil {
		f.logger.Error("could not replace segment marker file", "file", f.lastMarkedSegmentFilePath, "err", err)
		return
	}

	f.logger.Debug("updated segment marker file", "file", f.lastMarkedSegmentFilePath, "segment", segment)
}

// atomicallyWriteMarker attempts to perform an atomic write of the marker contents. This is delegated to
// https://github.com/natefinch/atomic/blob/master/atomic.go, that first handles atomic file renaming for UNIX and
// Windows systems. Also, atomic.WriteFile will first write the contents to a temporal file, and then perform the atomic
// rename, swapping the marker, or not at all.
func (f *baseFile) atomicallyWriteMarker(bs []byte) error {
	return atomic.WriteFile(f.lastMarkedSegmentFilePath, bytes.NewReader(bs))
}
