//go:build unix

package reporter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/google/pprof/profile"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/symb/irsymcache"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/xsync"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/reporter/samples"
	"go.opentelemetry.io/ebpf-profiler/support"
)

type PPROF struct {
	Raw    []byte
	Labels labels.Labels
	Origin libpf.Origin
}

type PPROFConsumer func(ctx context.Context, p []PPROF)

type Config struct {
	ReportInterval            time.Duration
	SamplesPerSecond          int64
	Demangle                  string
	ReporterUnsymbolizedStubs bool
}
type PPROFReporter struct {
	cfg *Config
	log log.Logger

	consumer PPROFConsumer
	symbols  irsymcache.NativeSymbolResolver

	traceEvents xsync.RWMutex[samples.TraceEventsTree]

	sd discovery.TargetProducer

	wg              sync.WaitGroup
	cancelReporting context.CancelFunc
}

func NewPPROF(log log.Logger,
	cfg *Config,
	sd discovery.TargetProducer,
	symbols irsymcache.NativeSymbolResolver,
	consumer PPROFConsumer,
) *PPROFReporter {

	tree := make(samples.TraceEventsTree)
	return &PPROFReporter{
		cfg:         cfg,
		log:         log,
		traceEvents: xsync.NewRWMutex(tree),
		sd:          sd,
		consumer:    consumer,
		symbols:     symbols,
	}
}

var errUnknownOrigin = errors.New("unknown trace origin")

func (p *PPROFReporter) ReportTraceEvent(trace *libpf.Trace, meta *samples.TraceEventMeta) error {
	switch meta.Origin {
	case support.TraceOriginSampling:
	case support.TraceOriginOffCPU:
	case support.TraceOriginProbe:
	default:
		return fmt.Errorf("skip reporting trace for %d origin: %w", meta.Origin,
			errUnknownOrigin)
	}

	containerID := meta.ContainerID
	key := samples.TraceAndMetaKey{
		Hash:           trace.Hash,
		Comm:           meta.Comm,
		ProcessName:    meta.ProcessName,
		ExecutablePath: meta.ExecutablePath,
		ApmServiceName: meta.APMServiceName,
		Pid:            int64(meta.PID),
		Tid:            int64(meta.TID),
	}

	eventsTree := p.traceEvents.WLock()
	defer p.traceEvents.WUnlock(&eventsTree)

	if _, exists := (*eventsTree)[containerID]; !exists {
		(*eventsTree)[containerID] =
			make(map[libpf.Origin]samples.KeyToEventMapping)
	}

	if _, exists := (*eventsTree)[containerID][meta.Origin]; !exists {
		(*eventsTree)[containerID][meta.Origin] =
			make(samples.KeyToEventMapping)
	}

	if events, exists := (*eventsTree)[containerID][meta.Origin][key]; exists {
		events.Timestamps = append(events.Timestamps, uint64(meta.Timestamp))
		events.OffTimes = append(events.OffTimes, meta.OffTime)
		(*eventsTree)[containerID][meta.Origin][key] = events
		return nil
	}
	(*eventsTree)[containerID][meta.Origin][key] = &samples.TraceEvents{
		Frames:     trace.Frames,
		Timestamps: []uint64{uint64(meta.Timestamp)},
		OffTimes:   []int64{meta.OffTime},
		EnvVars:    meta.EnvVars,
		Labels:     trace.CustomLabels,
	}
	return nil
}

func (p *PPROFReporter) Start(ctx context.Context) error {
	ctx, cancelReporting := context.WithCancel(ctx)
	p.cancelReporting = cancelReporting
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()
		tick := time.NewTicker(p.cfg.ReportInterval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				p.reportProfile(ctx)
			}
		}
	}()

	return nil
}

func (p *PPROFReporter) Stop() {
	if p.cancelReporting != nil {
		p.cancelReporting()
	}
	p.wg.Wait()
}

func (p *PPROFReporter) reportProfile(ctx context.Context) {
	traceEventsPtr := p.traceEvents.WLock()
	reportedEvents := *traceEventsPtr
	newEvents := make(samples.TraceEventsTree)
	*traceEventsPtr = newEvents
	p.traceEvents.WUnlock(&traceEventsPtr)
	var profiles []PPROF
	for containerID, ts := range reportedEvents {
		for origin, events := range ts {
			pp := p.createProfile(containerID, origin, events)
			profiles = append(profiles, pp...)
		}
	}

	p.consumer(ctx, profiles)
	sz := 0
	for _, it := range profiles {
		sz += len(it.Raw)
	}
	_ = level.Debug(p.log).Log("msg", "pprof report successful", "count", len(profiles), "total-size", sz)
}

func (p *PPROFReporter) createProfile(containerID samples.ContainerID, origin libpf.Origin, events map[samples.TraceAndMetaKey]*samples.TraceEvents) []PPROF {
	defer func() {
		if p.symbols != nil {
			p.symbols.Cleanup()
		}
	}()

	bs := NewProfileBuilders(BuildersOptions{
		SampleRate:    p.cfg.SamplesPerSecond,
		PerPIDProfile: true,
		Origin:        origin,
	})

	for traceKey, traceInfo := range events {
		target := p.sd.FindTarget(uint32(traceKey.Pid), containerID.String())
		if target == nil {
			continue
		}
		b := bs.BuilderForSample(target, uint32(traceKey.Pid))
		fakeMapping := b.FakeMapping()

		s := b.NewSample(len(traceInfo.Frames))

		switch origin {
		case support.TraceOriginSampling:
			b.AddValue(int64(len(traceInfo.Timestamps)), s)
		case support.TraceOriginOffCPU:
			sum := int64(0)
			for _, t := range traceInfo.OffTimes {
				sum += t
			}
			b.AddValue(sum, s)
		case support.TraceOriginProbe:
			b.AddValue(int64(len(traceInfo.Timestamps)), s)
		}

		for i := range traceInfo.Frames {
			fr := traceInfo.Frames[i].Value()
			var (
				mapping  *profile.Mapping
				location *profile.Location
				fresh    bool
			)
			if fr.Mapping.Valid() {
				mappingData := fr.Mapping.Value()
				pfMapping := mappingData.File.Value()
				mapping, fresh = b.Mapping(mappingData.Start, mappingData.File)
				if fresh {
					mapping.Start = uint64(mappingData.Start)
					mapping.Limit = uint64(mappingData.End)
					mapping.Offset = mappingData.FileOffset
					mapping.File = pfMapping.FileName.String()
					mapping.BuildID = pfMapping.GnuBuildID
				}
			} else {
				mapping = fakeMapping
			}

			location, fresh = b.Location(mapping, fr.AddressOrLineno, fr.FunctionName, fr.SourceLine)
			if fresh {
				location.Mapping = mapping
				location.Address = uint64(fr.AddressOrLineno)
				switch fr.Type {
				case libpf.NativeFrame:
					p.symbolizeNativeFrame(b, location, fr)
					if fr.FunctionName == libpf.NullString {
						if location.Line == nil && p.cfg.ReporterUnsymbolizedStubs {
							p.symbolizeStub(b, location, fr)
						}
					} else {
						location.Line = []profile.Line{{
							Function: b.Function(
								p.demangle(fr.FunctionName),
								fr.SourceFile,
							),
						}}
						location.Mapping.HasFunctions = true
					}

				default:
					if fr.Type.IsAbort() {
						// Be explicit about unknown frames so that we do introduce unknown unknowns.
						location.Line = []profile.Line{{
							Line: 0,
							Function: b.Function(
								libpf.Intern("[unknown]"),
								libpf.Intern("[unknown]"),
							)},
						}
						location.Mapping.HasFunctions = true
					} else if fr.FunctionName != libpf.NullString {
						location.Line = []profile.Line{{
							Line: int64(fr.SourceLine),
							Function: b.Function(
								fr.FunctionName,
								fr.SourceFile,
							)},
						}
						location.Mapping.HasFunctions = true
						location.Mapping.HasLineNumbers = true
					}
				}
			}
			if fr.Type == libpf.PythonFrame && len(location.Line) == 1 && location.Line[0].Function.Name == "<interpreter trampoline>" {
				continue
			}
			s.Location = append(s.Location, location)
		}
	}
	res := make([]PPROF, 0, len(bs.Builders))
	for _, b := range bs.Builders {
		buf := bytes.NewBuffer(nil)
		_, err := b.Write(buf)
		if err != nil {
			_ = p.log.Log("err", err)
			continue
		}
		_, ls := b.Target.Labels()
		metric := discovery.MetricValueProcessCPU
		if origin == support.TraceOriginOffCPU {
			metric = discovery.MetricValueOffCPU
		}

		builder := labels.NewScratchBuilder(ls.Len() + 1)
		ls.Range(func(l labels.Label) {
			builder.Add(l.Name, l.Value)
		})
		builder.Add(model.MetricNameLabel, metric)
		builder.Sort()
		res = append(res, PPROF{
			Raw:    buf.Bytes(),
			Labels: builder.Labels(),
			Origin: origin,
		})
	}
	return res
}

func (p *PPROFReporter) symbolizeNativeFrame(
	b *ProfileBuilder,
	loc *profile.Location,
	fr libpf.Frame,
) {

	if !fr.Mapping.Valid() {
		return
	}
	mappingFile := fr.Mapping.Value().File.Value()
	if mappingFile.FileName == process.VdsoPathName {
		return
	}
	if p.symbols == nil {
		return
	}
	irsymcache.SymbolizeNativeFrame(p.symbols, mappingFile.FileName, fr.AddressOrLineno, mappingFile.FileID, func(si irsymcache.SourceInfo) {
		name := si.FunctionName
		if name == libpf.NullString && si.FilePath == libpf.NullString {
			return
		}
		name = p.demangle(name)
		loc.Mapping.HasFunctions = true
		line := profile.Line{Function: b.Function(name, si.FilePath)}
		loc.Line = append(loc.Line, line)
	})
}

func (p *PPROFReporter) symbolizeStub(b *ProfileBuilder, location *profile.Location, fr libpf.Frame) {
	if location.Mapping.File == "" {
		return
	}
	location.Line = []profile.Line{{
		Function: b.Function(
			libpf.Intern(fmt.Sprintf("$ %s + 0x%x", location.Mapping.File, fr.AddressOrLineno)),
			fr.SourceFile,
		),
	}}
	location.Mapping.HasFunctions = true
}
