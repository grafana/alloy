package file

// This code is adapted from loki/promtail. Last revision used to port changes to Alloy was a8d5815510bd959a6dd8c176a5d9fd9bbfc8f8b5.
// Decompressor implements the reader interface and is used to read compressed log files.
// It uses the Go stdlib's compress/* packages for decoding.

import (
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"compress/zlib" //TODO(ptodev): Replace this with https://github.com/klauspost/compress
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/go-kit/log"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"
	"golang.org/x/text/encoding"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func supportedCompressedFormats() map[string]struct{} {
	return map[string]struct{}{
		"gz":  {},
		"z":   {},
		"bz2": {},
		// TODO: add support for zip.
	}
}

type decompressor struct {
	metrics   *metrics
	logger    log.Logger
	receiver  loki.LogsReceiver
	positions positions.Positions

	key    positions.Entry
	labels model.LabelSet

	posAndSizeMtx sync.RWMutex

	onPositionsFileError OnPositionsFileError

	running *atomic.Bool

	decoder *encoding.Decoder

	position int64
	size     int64
	cfg      DecompressionConfig

	componentStopping func() bool
}

func newDecompressor(
	metrics *metrics,
	logger log.Logger,
	receiver loki.LogsReceiver,
	pos positions.Positions,
	componentStopping func() bool,
	opts sourceOptions,
) (*decompressor, error) {

	labelsStr := opts.labels.String()

	logger = log.With(logger, "component", "decompressor")

	position, err := pos.Get(opts.path, labelsStr)
	if err != nil {
		switch opts.onPositionsFileError {
		case OnPositionsFileErrorSkip:
			return nil, fmt.Errorf("failed to get file position: %w", err)
		case OnPositionsFileErrorRestartEnd:
			level.Warn(logger).Log("msg", "`restart_from_end` is not supported for compressed files, defaulting to `restart_from_beginning`")
			fallthrough
		default:
			level.Warn(logger).Log("msg", "unrecognized `on_positions_file_error` option, defaulting to `restart_from_beginning`", "option", opts.onPositionsFileError)
			fallthrough
		case OnPositionsFileErrorRestartBeginning:
			position = 0
			level.Info(logger).Log("msg", "reset position to start of file after positions error", "original_error", err)
		}
	}

	decoder, err := getDecoder(opts.encoding)
	if err != nil {
		return nil, fmt.Errorf("failed to get decoder: %w", err)
	}

	decompressor := &decompressor{
		metrics:              metrics,
		logger:               logger,
		receiver:             receiver,
		positions:            pos,
		key:                  positions.Entry{Path: opts.path, Labels: labelsStr},
		labels:               opts.labels,
		running:              atomic.NewBool(false),
		position:             position,
		decoder:              decoder,
		cfg:                  opts.decompressionConfig,
		onPositionsFileError: opts.onPositionsFileError,
		componentStopping:    componentStopping,
	}

	return decompressor, nil
}

// mountReader instantiate a reader ready to be used by the decompressor.
//
// The reader implementation is selected based on the given CompressionFormat.
// If the actual file format is incorrect, the reading of the header may fail and return an error - depending on the
// implementation of the underlying compression library. In any case, when a file is corrupted, the subsequent reading
// of lines will fail.
func mountReader(f *os.File, decoder *encoding.Decoder, logger log.Logger, format CompressionFormat) (reader io.Reader, err error) {
	var decompressLib string

	switch format.String() {
	case "gz":
		decompressLib = "compress/gzip"
		reader, err = gzip.NewReader(f)
	case "z":
		decompressLib = "compress/zlib"
		reader, err = zlib.NewReader(f)
	case "bz2":
		decompressLib = "bzip2"
		reader = bzip2.NewReader(f)
	}

	if err != nil && err != io.EOF {
		return nil, err
	}

	if reader == nil {
		supportedFormatsList := strings.Builder{}
		for format := range supportedCompressedFormats() {
			supportedFormatsList.WriteString(format)
		}
		return nil, fmt.Errorf("file %q has unsupported format, it has to be one of %q", f.Name(), supportedFormatsList.String())
	}

	level.Debug(logger).Log("msg", fmt.Sprintf("using %q to decompress file %q", decompressLib, f.Name()))

	// Use the appropriated decoder for the file encoding.
	// Otherwise the file may not be split into separate lines properly.
	if decoder != nil {
		reader = decoder.Reader(reader)
	}

	return reader, nil
}

func (d *decompressor) Run(ctx context.Context) {
	// Check if context was canceled between two calls to Run.
	select {
	case <-ctx.Done():
		return
	default:
	}

	labelsMiddleware := d.labels.Merge(model.LabelSet{labelFilename: model.LabelValue(d.key.Path)})
	handler := loki.AddLabelsMiddleware(labelsMiddleware).Wrap(loki.NewEntryHandler(d.receiver.Chan(), func() {}))
	defer handler.Stop()

	d.metrics.filesActive.Add(1.)

	done := make(chan struct{})
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		// readLines closes done on exit
		d.readLines(handler, done)
		cancel()
	}()

	d.running.Store(true)
	defer d.running.Store(false)

	<-ctx.Done()
	d.stop(done)
}

func (d *decompressor) updatePosition(posquit chan struct{}) {
	positionSyncPeriod := d.positions.SyncPeriod()
	positionWait := time.NewTicker(positionSyncPeriod)
	defer func() {
		positionWait.Stop()
		level.Info(d.logger).Log("msg", "position timer: exited", "path", d.key.Path)
		d.cleanupMetrics()
	}()

	for {
		select {
		case <-positionWait.C:
			if err := d.markPositionAndSize(); err != nil {
				level.Error(d.logger).Log("msg", "position timer: error getting position and/or size, stopping decompressor", "path", d.key.Path, "error", err)
				return
			}
		case <-posquit:
			return
		}
	}
}

// readLines read all existing lines of the given compressed file.
//
// It first decompresses the file as a whole using a reader and then it will iterate
// over its chunks, separated by '\n'.
// During each iteration, the parsed and decoded log line is then sent to the API with the current timestamp.
// done channel is closed when readlines exits.
func (d *decompressor) readLines(handler loki.EntryHandler, done chan struct{}) {
	level.Info(d.logger).Log("msg", "read lines routine: started", "path", d.key.Path)

	if d.cfg.InitialDelay > 0 {
		level.Info(d.logger).Log("msg", "sleeping before starting decompression", "path", d.key.Path, "duration", d.cfg.InitialDelay.String())
		time.Sleep(d.cfg.InitialDelay)
	}

	posquit, posdone := make(chan struct{}), make(chan struct{})
	go func() {
		d.updatePosition(posquit)
		close(posdone)
	}()

	defer func() {
		level.Info(d.logger).Log("msg", "read lines routine finished", "path", d.key.Path)
		close(posquit)
		<-posdone
		close(done)
	}()

	entries := handler.Chan()

	f, err := os.Open(d.key.Path)
	if err != nil {
		level.Error(d.logger).Log("msg", "error reading file", "path", d.key.Path, "error", err)
		return
	}
	defer f.Close()

	r, err := mountReader(f, d.decoder, d.logger, d.cfg.Format)
	if err != nil {
		level.Error(d.logger).Log("msg", "error mounting new reader", "err", err)
		return
	}

	level.Info(d.logger).Log("msg", "successfully mounted reader", "path", d.key.Path, "ext", filepath.Ext(d.key.Path))

	bufferSize := 4096
	buffer := make([]byte, bufferSize)
	maxLoglineSize := 2000000 // 2 MB
	scanner := bufio.NewScanner(r)
	scanner.Buffer(buffer, maxLoglineSize)
	for line := int64(1); ; line++ {
		if !scanner.Scan() {
			break
		}

		if scannerErr := scanner.Err(); scannerErr != nil {
			if scannerErr != io.EOF {
				level.Error(d.logger).Log("msg", "error scanning", "err", scannerErr)
			}

			break
		}

		d.posAndSizeMtx.RLock()
		if line <= d.position {
			// skip already seen lines.
			d.posAndSizeMtx.RUnlock()
			continue
		}
		d.posAndSizeMtx.RUnlock()

		text := scanner.Text()

		d.metrics.readLines.WithLabelValues(d.key.Path).Inc()

		// Trim Windows line endings
		text = strings.TrimSuffix(text, "\r")

		entries <- loki.Entry{
			// Allocate the expected size of labels. This matches the number of labels added by the middleware
			// as configured in Run().
			Labels: make(model.LabelSet, len(d.labels)+1),
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      text,
			},
		}

		d.posAndSizeMtx.Lock()
		d.size = int64(unsafe.Sizeof(text))
		d.position++
		d.posAndSizeMtx.Unlock()
	}
}

func (d *decompressor) markPositionAndSize() error {
	// Lock this update because it can be called in two different goroutines
	d.posAndSizeMtx.RLock()
	defer d.posAndSizeMtx.RUnlock()

	d.metrics.totalBytes.WithLabelValues(d.key.Path).Set(float64(d.size))
	d.metrics.readBytes.WithLabelValues(d.key.Path).Set(float64(d.position))
	d.positions.Put(d.key.Path, d.key.Labels, d.position)

	return nil
}

func (d *decompressor) stop(done chan struct{}) {
	// Wait for readLines() to consume all the remaining messages and exit when the channel is closed
	<-done

	level.Info(d.logger).Log("msg", "stopped decompressor", "path", d.key.Path)

	// If the component is not stopping, then it means that the target for this component is gone and that
	// we should clear the entry from the positions file.
	if !d.componentStopping() {
		d.positions.Remove(d.key.Path, d.key.Labels)
	} else {
		// Save the current position before shutting down reader
		if err := d.markPositionAndSize(); err != nil {
			level.Error(d.logger).Log("msg", "error marking file position when stopping decompressor", "path", d.key.Path, "error", err)
		}
	}
}

func (d *decompressor) Key() positions.Entry {
	return d.key
}

func (d *decompressor) DebugInfo() any {
	offset, _ := d.positions.Get(d.key.Path, d.key.Labels)
	return sourceDebugInfo{
		Path:       d.key.Path,
		Labels:     d.key.Labels,
		IsRunning:  d.running.Load(),
		ReadOffset: offset,
	}
}

// cleanupMetrics removes all metrics exported by this reader
func (d *decompressor) cleanupMetrics() {
	// When we stop tailing the file, un-export metrics related to the file.
	d.metrics.filesActive.Add(-1.)
	d.metrics.readLines.DeleteLabelValues(d.key.Path)
	d.metrics.readBytes.DeleteLabelValues(d.key.Path)
	d.metrics.totalBytes.DeleteLabelValues(d.key.Path)
}
