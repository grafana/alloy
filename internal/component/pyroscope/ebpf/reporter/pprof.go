//go:build linux && (arm64 || amd64)

package reporter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/runtime/logging/level"
	"go.opentelemetry.io/ebpf-profiler/host"

	"github.com/go-kit/log"
	"github.com/google/pprof/profile"
	"github.com/prometheus/prometheus/model/labels"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/xsync"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/discovery"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/symb/irsymcache"
	"go.opentelemetry.io/ebpf-profiler/reporter/samples"
	"go.opentelemetry.io/ebpf-profiler/support"
)

type PPROF struct {
	Raw    []byte
	Labels labels.Labels
	Origin libpf.Origin
}
type PPROFConsumer interface {
	ConsumePprofProfiles(ctx context.Context, p []PPROF)
}

type PPROFConsumerFunc func(ctx context.Context, p []PPROF)

func (f PPROFConsumerFunc) ConsumePprofProfiles(ctx context.Context, p []PPROF) {
	f(ctx, p)
}

type Config struct {
	ReportInterval   time.Duration
	SamplesPerSecond int64

	ExtraNativeSymbolResolver samples.NativeSymbolResolver
	Consumer                  PPROFConsumer
}
type PPROFReporter struct {
	cfg *Config
	log log.Logger

	traceEvents xsync.RWMutex[samples.TraceEventsTree]

	sd              discovery.TargetProducer
	wg              sync.WaitGroup
	cancelReporting context.CancelFunc
}

func NewPPROF(
	log log.Logger,
	cfg *Config,
	sd discovery.TargetProducer,
) *PPROFReporter {
	tree := make(samples.TraceEventsTree)
	return &PPROFReporter{
		cfg:         cfg,
		log:         log,
		traceEvents: xsync.NewRWMutex(tree),
		sd:          sd,
	}
}

var errUnknownOrigin = errors.New("unknown trace origin")

func (p *PPROFReporter) ReportTraceEvent(trace *libpf.Trace, meta *samples.TraceEventMeta) error {
	switch meta.Origin {
	case support.TraceOriginSampling:
	case support.TraceOriginOffCPU:
	case support.TraceOriginUProbe:
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
		ContainerID:    containerID,
		Pid:            int64(meta.PID),
		Tid:            int64(meta.TID),
	}

	eventsTree := p.traceEvents.WLock()
	defer p.traceEvents.WUnlock(&eventsTree)

	if _, exists := (*eventsTree)[samples.ContainerID(containerID)]; !exists {
		(*eventsTree)[samples.ContainerID(containerID)] =
			make(map[libpf.Origin]samples.KeyToEventMapping)
	}

	if _, exists := (*eventsTree)[samples.ContainerID(containerID)][meta.Origin]; !exists {
		(*eventsTree)[samples.ContainerID(containerID)][meta.Origin] =
			make(samples.KeyToEventMapping)
	}

	if events, exists := (*eventsTree)[samples.ContainerID(containerID)][meta.Origin][key]; exists {
		events.Timestamps = append(events.Timestamps, uint64(meta.Timestamp))
		events.OffTimes = append(events.OffTimes, meta.OffTime)
		(*eventsTree)[samples.ContainerID(containerID)][meta.Origin][key] = events
		return nil
	}
	(*eventsTree)[samples.ContainerID(containerID)][meta.Origin][key] = &samples.TraceEvents{
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
	for cid, ts := range reportedEvents {
		for origin, events := range ts {
			pp := p.createProfile(origin, events, cid)
			profiles = append(profiles, pp...)
		}
	}

	p.cfg.Consumer.ConsumePprofProfiles(ctx, profiles)
	sz := 0
	for _, it := range profiles {
		sz += len(it.Raw)
	}
	_ = level.Debug(p.log).Log("msg", "pprof report successful", "count", len(profiles), "total-size", sz)
}

func (p *PPROFReporter) createProfile(origin libpf.Origin, events map[samples.TraceAndMetaKey]*samples.TraceEvents, cid samples.ContainerID) []PPROF {
	defer func() {
		if p.cfg.ExtraNativeSymbolResolver != nil {
			p.cfg.ExtraNativeSymbolResolver.Cleanup()
		}
	}()

	bs := NewProfileBuilders(BuildersOptions{
		SampleRate:    p.cfg.SamplesPerSecond,
		PerPIDProfile: true,
		Origin:        origin,
	})

	for traceKey, traceInfo := range events {
		target := p.sd.FindTarget(uint32(traceKey.Pid), traceKey.ContainerID)
		if target == nil {
			continue
		}
		b := bs.BuilderForSample(target, uint32(traceKey.Pid))
		fakeMapping, _ := b.Mapping(0, libpf.FrameMappingFile{})

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
		}

		for i := range traceInfo.Frames {
			fr := traceInfo.Frames[i].Value()
			pfMapping := fr.MappingFile.Value()
			pfFileID := pfMapping.FileID
			pfAddrOrLineNo := fr.AddressOrLineno
			location, locationFresh := b.Location(pfFileID, pfAddrOrLineNo)

			if locationFresh {
				if fr.MappingFile.Valid() {
					mapping, mappingFresh := b.Mapping(fr.MappingStart, fr.MappingFile)
					if mappingFresh {
						mapping.Start = uint64(fr.MappingStart)
						mapping.Limit = uint64(fr.MappingEnd)
						mapping.Offset = fr.MappingFileOffset
						mapping.File = pfMapping.FileName.String()
						mapping.BuildID = pfMapping.GnuBuildID
					}
					location.Mapping = mapping
				} else {
					location.Mapping = fakeMapping
				}
				location.Address = uint64(pfAddrOrLineNo)
				switch fr.Type {
				case libpf.NativeFrame:
					p.symbolizeNativeFrame(b, location, fr)
				case libpf.AbortFrame:
					// Next step: Figure out how the OTLP protocol
					// could handle artificial frames, like AbortFrame,
					// that are not originated from a native or interpreted
					// program.
				default:
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
		labelsWithMetric := make([]labels.Label, 0, len(ls)+1)
		labelsWithMetric = append(labelsWithMetric, ls...)
		labelsWithMetric = append(labelsWithMetric, labels.Label{
			Name:  labels.MetricName,
			Value: metric,
		})
		res = append(res, PPROF{
			Raw:    buf.Bytes(),
			Labels: labelsWithMetric,
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
	mappingFile := fr.MappingFile.Value()
	if mappingFile.FileName == process.VdsoPathName {
		return
	}
	if p.cfg.ExtraNativeSymbolResolver == nil {
		return
	}
	addr := fr.AddressOrLineno
	hostFrame := host.Frame{
		File:          host.FileIDFromLibpf(mappingFile.FileID),
		Lineno:        addr,
		Type:          fr.Type,
		ReturnAddress: false,
	}
	irsymcache.SymbolizeNativeFrame(p.cfg.ExtraNativeSymbolResolver, mappingFile.FileName, hostFrame, func(si samples.SourceInfo) {
		if si.FunctionName == libpf.NullString && si.FilePath == libpf.NullString {
			return
		}
		loc.Mapping.HasFunctions = true
		line := profile.Line{Function: b.Function(si.FunctionName, si.FilePath)}
		loc.Line = append(loc.Line, line)
	})
}
