package internal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/agent/internal/component/common/loki/wal"
	"github.com/natefinch/atomic"
)

const (
	MarkerFolderName = "remote"
	MarkerFileName   = "segment_marker"

	MarkerFolderMode        os.FileMode = 0o700
	MarkerWindowsFolderMode os.FileMode = 0o777
	MarkerFileMode          os.FileMode = 0o600
	MarkerWindowsFileMode   os.FileMode = 0o666
)

// MarkerFileHandler is a file-backed wal.Marker, that also allows one to write to the backing store as particular
// segment number as the last one marked.
type MarkerFileHandler interface {
	wal.Marker

	// MarkSegment writes in the backing file-store that a particular segment is the last one marked.
	MarkSegment(segment int)
}

type markerFileHandler struct {
	logger                    log.Logger
	lastMarkedSegmentDir      string
	lastMarkedSegmentFilePath string
}

var (
	_ MarkerFileHandler = (*markerFileHandler)(nil)
)

// NewMarkerFileHandler creates a new markerFileHandler.
func NewMarkerFileHandler(logger log.Logger, walDir string) (MarkerFileHandler, error) {
	markerDir := filepath.Join(walDir, MarkerFolderName)
	// attempt to create dir if doesn't exist
	if err := os.MkdirAll(markerDir, MarkerFolderMode); err != nil {
		return nil, fmt.Errorf("error creating segment marker folder %q: %w", markerDir, err)
	}

	mfh := &markerFileHandler{
		logger:                    logger,
		lastMarkedSegmentDir:      filepath.Join(markerDir),
		lastMarkedSegmentFilePath: filepath.Join(markerDir, MarkerFileName),
	}

	return mfh, nil
}

// LastMarkedSegment implements wlog.Marker.
func (mfh *markerFileHandler) LastMarkedSegment() int {
	bs, err := os.ReadFile(mfh.lastMarkedSegmentFilePath)
	if os.IsNotExist(err) {
		level.Warn(mfh.logger).Log("msg", "marker segment file does not exist", "file", mfh.lastMarkedSegmentFilePath)
		return -1
	} else if err != nil {
		level.Error(mfh.logger).Log("msg", "could not access segment marker file", "file", mfh.lastMarkedSegmentFilePath, "err", err)
		return -1
	}

	savedSegment, err := DecodeMarkerV1(bs)
	if err != nil {
		level.Error(mfh.logger).Log("msg", "could not decode segment marker file", "file", mfh.lastMarkedSegmentFilePath, "err", err)
		return -1
	}

	return int(savedSegment)
}

// MarkSegment implements MarkerHandler.
func (mfh *markerFileHandler) MarkSegment(segment int) {
	encodedMarker, err := EncodeMarkerV1(uint64(segment))
	if err != nil {
		level.Error(mfh.logger).Log("msg", "failed to encode marker when marking segment", "err", err)
		return
	}

	if err := mfh.atomicallyWriteMarker(encodedMarker); err != nil {
		level.Error(mfh.logger).Log("msg", "could not replace segment marker file", "file", mfh.lastMarkedSegmentFilePath, "err", err)
		return
	}

	level.Debug(mfh.logger).Log("msg", "updated segment marker file", "file", mfh.lastMarkedSegmentFilePath, "segment", segment)
}

// atomicallyWriteMarker attempts to perform an atomic write of the marker contents. This is delegated to
// https://github.com/natefinch/atomic/blob/master/atomic.go, that first handles atomic file renaming for UNIX and
// Windows systems. Also, atomic.WriteFile will first write the contents to a temporal file, and then perform the atomic
// rename, swapping the marker, or not at all.
func (mfh *markerFileHandler) atomicallyWriteMarker(bs []byte) error {
	return atomic.WriteFile(mfh.lastMarkedSegmentFilePath, bytes.NewReader(bs))
}
