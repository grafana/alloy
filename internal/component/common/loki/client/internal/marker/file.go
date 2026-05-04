package marker

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
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

type File struct {
	logger                    log.Logger
	lastMarkedSegmentDir      string
	lastMarkedSegmentFilePath string
}

// NewFile creates a new marker File.
func NewFile(logger log.Logger, dir string) (*File, error) {
	markerDir := filepath.Join(dir, markerFolderName)
	// attempt to create dir if doesn't exist
	if err := os.MkdirAll(markerDir, markerFolderMode); err != nil {
		return nil, fmt.Errorf("error creating segment marker folder %q: %w", markerDir, err)
	}

	return &File{
		logger:                    logger,
		lastMarkedSegmentDir:      filepath.Join(markerDir),
		lastMarkedSegmentFilePath: filepath.Join(markerDir, markerFileName),
	}, nil
}

// LastMarkedSegment implements wlog.Marker.
func (f *File) LastMarkedSegment() int {
	bs, err := os.ReadFile(f.lastMarkedSegmentFilePath)
	if os.IsNotExist(err) {
		level.Warn(f.logger).Log("msg", "marker segment file does not exist", "file", f.lastMarkedSegmentFilePath)
		return -1
	} else if err != nil {
		level.Error(f.logger).Log("msg", "could not access segment marker file", "file", f.lastMarkedSegmentFilePath, "err", err)
		return -1
	}

	savedSegment, err := decodeV1(bs)
	if err != nil {
		level.Error(f.logger).Log("msg", "could not decode segment marker file", "file", f.lastMarkedSegmentFilePath, "err", err)
		return -1
	}

	return int(savedSegment)
}

// MarkSegment implements MarkerHandler.
func (f *File) MarkSegment(segment int) {
	encodedMarker, err := encodeV1(uint64(segment))
	if err != nil {
		level.Error(f.logger).Log("msg", "failed to encode marker when marking segment", "err", err)
		return
	}

	if err := f.atomicallyWriteMarker(encodedMarker); err != nil {
		level.Error(f.logger).Log("msg", "could not replace segment marker file", "file", f.lastMarkedSegmentFilePath, "err", err)
		return
	}

	level.Debug(f.logger).Log("msg", "updated segment marker file", "file", f.lastMarkedSegmentFilePath, "segment", segment)
}

// atomicallyWriteMarker attempts to perform an atomic write of the marker contents. This is delegated to
// https://github.com/natefinch/atomic/blob/master/atomic.go, that first handles atomic file renaming for UNIX and
// Windows systems. Also, atomic.WriteFile will first write the contents to a temporal file, and then perform the atomic
// rename, swapping the marker, or not at all.
func (f *File) atomicallyWriteMarker(bs []byte) error {
	return atomic.WriteFile(f.lastMarkedSegmentFilePath, bytes.NewReader(bs))
}
