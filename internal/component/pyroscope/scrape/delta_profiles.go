package scrape

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/scrape/internal/fastdelta"

	"github.com/klauspost/compress/gzip"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

var deltaProfiles map[string][]fastdelta.ValueType = map[string][]fastdelta.ValueType{
	pprofMemory: {
		{Type: "alloc_objects", Unit: "count"},
		{Type: "alloc_space", Unit: "bytes"},
	},
	pprofMutex: {
		{Type: "contentions", Unit: "count"},
		{Type: "delay", Unit: "nanoseconds"},
	},
	pprofBlock: {
		{Type: "contentions", Unit: "count"},
		{Type: "delay", Unit: "nanoseconds"},
	},
}

type DeltaProfiler interface {
	Delta(p []byte, out io.Writer) error
}

func NewDeltaAppender(appender pyroscope.Appender, labels labels.Labels) pyroscope.Appender {
	types, ok := deltaProfiles[labels.Get(model.MetricNameLabel)]
	if !ok {
		// for profiles that we don't need to produce delta, just return the appender
		return appender
	}

	return newDeltaAppender(appender, types)
}

func newDeltaAppender(appender pyroscope.Appender, types []fastdelta.ValueType) *deltaAppender {
	delta := &deltaAppender{
		appender: appender,
		delta:    fastdelta.NewDeltaComputer(types...),
	}
	return delta
}

type deltaAppender struct {
	appender pyroscope.Appender
	delta    DeltaProfiler

	// true if we have seen at least one sample
	initialized bool
}

type gzipBuffer struct {
	gzr gzip.Reader
	gzw *gzip.Writer

	out          bytes.Buffer
	uncompressed bytes.Buffer
	in           *bytes.Reader
}

var gzipBufferPool = sync.Pool{
	New: func() any {
		return &gzipBuffer{
			gzw: gzip.NewWriter(nil),
			in:  bytes.NewReader(nil),
		}
	},
}

func getGzipBuffer() *gzipBuffer {
	buf := gzipBufferPool.Get().(*gzipBuffer)
	buf.reset()
	return buf
}

func putGzipBuffer(buf *gzipBuffer) {
	gzipBufferPool.Put(buf)
}

func (d *gzipBuffer) reset() io.Writer {
	d.out.Reset()
	d.gzw.Reset(&d.out)
	return d.gzw
}

func (d *gzipBuffer) uncompress(in []byte) ([]byte, error) {
	if !isGzipData(in) {
		return in, nil
	}
	d.in.Reset(in)
	if err := d.gzr.Reset(d.in); err != nil {
		return nil, err
	}
	d.uncompressed.Reset()
	d.uncompressed.Grow(uncompressedSize(in))
	_, err := d.uncompressed.ReadFrom(&d.gzr)
	if err != nil {
		return nil, fmt.Errorf("decompressing profile: %v", err)
	}
	return d.uncompressed.Bytes(), nil
}

func (d *deltaAppender) Append(ctx context.Context, lbs labels.Labels, samples []*pyroscope.RawSample) error {
	// Notify the server that this profile is a delta profile and we don't need to compute the delta again.
	lbsBuilder := labels.NewBuilder(lbs)
	lbsBuilder.Set(pyroscope.LabelNameDelta, "false")
	for _, sample := range samples {
		data, err := d.computeDelta(sample.RawProfile)
		if err != nil {
			return err
		}
		// The first sample should be skipped because we don't have a previous sample to compute delta with.
		if !d.initialized {
			d.initialized = true
			continue
		}
		if err := d.appender.Append(ctx, lbsBuilder.Labels(), []*pyroscope.RawSample{{RawProfile: data}}); err != nil {
			return err
		}
	}
	return nil
}

func (d *deltaAppender) AppendIngest(_ context.Context, _ *pyroscope.IncomingProfile) error {
	// No-op: AppendIngest is not used in deltaAppender
	return nil
}

// computeDelta computes the delta between the given profile and the last
// data is uncompressed if it is gzip compressed.
// The returned data is always gzip compressed.
func (d *deltaAppender) computeDelta(data []byte) (b []byte, err error) {
	gzipBuf := getGzipBuffer()
	defer putGzipBuffer(gzipBuf)

	data, err = gzipBuf.uncompress(data)
	if err != nil {
		return nil, err
	}

	if err = d.delta.Delta(data, gzipBuf.gzw); err != nil {
		return nil, fmt.Errorf("computing delta: %v", err)
	}
	if err := gzipBuf.gzw.Close(); err != nil {
		return nil, fmt.Errorf("closing gzip writer: %v", err)
	}
	// The returned slice will be retained in case the profile upload fails,
	// so we need to return a copy of the buffer's bytes to avoid a data
	// race.
	b = make([]byte, len(gzipBuf.out.Bytes()))
	copy(b, gzipBuf.out.Bytes())
	return b, nil
}

func isGzipData(data []byte) bool {
	return bytes.HasPrefix(data, []byte{0x1f, 0x8b})
}

func uncompressedSize(in []byte) int {
	last := len(in)
	if last < 4 {
		return -1
	}
	return int(binary.LittleEndian.Uint32(in[last-4 : last]))
}
