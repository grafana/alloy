package reporter

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/pprof/profile"
	"github.com/klauspost/compress/gzip"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/discovery"
	"go.opentelemetry.io/ebpf-profiler/support"
)

var (
	gzipWriterPool = sync.Pool{
		New: func() any {
			res, err := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
			if err != nil {
				panic(err)
			}
			return res
		},
	}
)

type BuildersOptions struct {
	SampleRate    int64
	PerPIDProfile bool
}

type builderHashKey struct {
	labelsHash uint64
	pid        uint32
	sampleType libpf.Origin
}

type ProfileBuilders struct {
	Builders map[builderHashKey]*ProfileBuilder
	opt      BuildersOptions

	samples   batch[profile.Sample]
	functions batch[profile.Function]
	locations batch[profile.Location]
}

func NewProfileBuilders(options BuildersOptions) *ProfileBuilders {
	return &ProfileBuilders{Builders: make(map[builderHashKey]*ProfileBuilder), opt: options}
}

func (b *ProfileBuilders) BuilderForSample(
	target *discovery.Target,
	pid uint32,
	st libpf.Origin,
) *ProfileBuilder {
	labelsHash, _ := target.Labels()

	k := builderHashKey{labelsHash: labelsHash, sampleType: st}
	if b.opt.PerPIDProfile {
		k.pid = pid
	}
	res := b.Builders[k]
	if res != nil {
		return res
	}

	var sampleType []*profile.ValueType
	var periodType *profile.ValueType
	var period int64
	if st == support.TraceOriginSampling {
		sampleType = []*profile.ValueType{{Type: "cpu", Unit: "nanoseconds"}}
		periodType = &profile.ValueType{Type: "cpu", Unit: "nanoseconds"}
		period = time.Second.Nanoseconds() / b.opt.SampleRate
	} else if st == support.TraceOriginOffCPU {
		sampleType = []*profile.ValueType{{Type: "offcpu", Unit: "nanoseconds"}}
		period = 1
	} else {
		panic(fmt.Sprintf("unknown sample type %v", sampleType))
	}
	dummyMapping := &profile.Mapping{
		ID: 1,
	}
	builder := &ProfileBuilder{
		p:         b,
		locations: make(map[libpf.FrameID]*profile.Location),
		functions: make(map[functionsKey]*profile.Function),
		Target:    target,
		Profile: &profile.Profile{
			Mapping: []*profile.Mapping{
				dummyMapping,
			},
			SampleType: sampleType,
			Period:     period,
			PeriodType: periodType,
			TimeNanos:  time.Now().UnixNano(),
		},
		dummyMapping:    dummyMapping,
		fileIDtoMapping: make(map[libpf.FileID]*profile.Mapping),
	}
	res = builder
	b.Builders[k] = res
	return res
}

type functionsKey struct {
	name string
	file string
}
type ProfileBuilder struct {
	p         *ProfileBuilders
	locations map[libpf.FrameID]*profile.Location

	functions map[functionsKey]*profile.Function

	Profile *profile.Profile
	Target  *discovery.Target

	dummyMapping    *profile.Mapping
	fileIDtoMapping map[libpf.FileID]*profile.Mapping
}

func (p *ProfileBuilder) Mapping(fid libpf.FileID) (*profile.Mapping, bool) {
	if tmpMappingIndex, exists := p.fileIDtoMapping[fid]; exists {
		return tmpMappingIndex, false
	}
	mid := uint64(len(p.Profile.Mapping) + 1)
	mapping := &profile.Mapping{
		ID: mid,
	}
	p.fileIDtoMapping[fid] = mapping
	p.Profile.Mapping = append(p.Profile.Mapping, mapping)
	return mapping, true
}

func (p *ProfileBuilder) Function(function, file string) *profile.Function {
	k := functionsKey{name: function, file: file}
	f, ok := p.functions[k]
	if ok {
		return f
	}

	id := uint64(len(p.Profile.Function) + 1)
	f = p.p.functions.pop()
	f.ID = id
	f.Name = function

	p.Profile.Function = append(p.Profile.Function, f)
	p.functions[k] = f
	return f
}

func (p *ProfileBuilder) Write(dst io.Writer) (int64, error) {
	gzipWriter := gzipWriterPool.Get().(*gzip.Writer)
	gzipWriter.Reset(dst)
	defer func() {
		gzipWriter.Reset(io.Discard)
		gzipWriterPool.Put(gzipWriter)
	}()
	err := p.Profile.WriteUncompressed(gzipWriter)
	if err != nil {
		return 0, fmt.Errorf("ebpf profile encode %w", err)
	}
	err = gzipWriter.Close()
	if err != nil {
		return 0, fmt.Errorf("ebpf profile encode %w", err)
	}
	return 0, nil
}

func (p *ProfileBuilder) NewSample(locSize int) *profile.Sample {
	sample := p.p.samples.pop()
	sample.Value = []int64{0}
	sample.Location = make([]*profile.Location, 0, locSize)
	p.Profile.Sample = append(p.Profile.Sample, sample)
	return sample
}

func (p *ProfileBuilder) AddValue(v int64, sample *profile.Sample) {
	sample.Value[0] += v * p.Profile.Period
}

func (p *ProfileBuilder) Location(frameID libpf.FrameID) (*profile.Location, bool) {
	loc, ok := p.locations[frameID]
	if ok {
		return loc, false
	}
	loc = p.p.locations.pop()
	loc.ID = uint64(len(p.Profile.Location) + 1)
	loc.Mapping = p.dummyMapping
	p.locations[frameID] = loc
	p.Profile.Location = append(p.Profile.Location, loc)
	return loc, true
}

type batch[T any] struct {
	items []T
}

func (b *batch[T]) pop() *T {
	if len(b.items) == 0 {
		b.items = make([]T, 128)
	}
	res := &b.items[0]
	b.items = b.items[1:]
	return res
}
