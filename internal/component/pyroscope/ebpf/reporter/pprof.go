//go:build linux && (arm64 || amd64) && pyroscope_ebpf

package reporter

import (
	"bytes"
	"context"
	"maps"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/runtime/logging/level"

	"github.com/elastic/go-freelru"
	lru "github.com/elastic/go-freelru"
	"github.com/go-kit/log"
	"github.com/google/pprof/profile"
	"github.com/prometheus/prometheus/model/labels"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/xsync"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/discovery"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/symb/irsymcache"
	reporter2 "go.opentelemetry.io/ebpf-profiler/reporter"
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
	CGroupCacheElements      uint32
	ReportInterval           time.Duration
	SamplesPerSecond         int64
	ExecutablesCacheElements uint32
	FramesCacheElements      uint32

	ExtraNativeSymbolResolver samples.NativeSymbolResolver
	Consumer                  PPROFConsumer
}
type PPROFReporter struct {
	cfg         *Config
	log         log.Logger
	cgroups     freelru.Cache[libpf.PID, string]
	traceEvents xsync.RWMutex[map[libpf.Origin]samples.KeyToEventMapping]
	Executables *freelru.SyncedLRU[libpf.FileID, samples.ExecInfo]
	Frames      *lru.SyncedLRU[libpf.FrameID, samples.SourceInfo]

	sd              discovery.TargetProducer
	wg              sync.WaitGroup
	cancelReporting context.CancelFunc
}

func NewPPROF(
	log log.Logger,
	cgroups freelru.Cache[libpf.PID, string],
	cfg *Config,
	sd discovery.TargetProducer,
) (*PPROFReporter, error) {
	// Set a lifetime to reduce risk of invalid data in case of PID reuse.
	cgroups.SetLifetime(90 * time.Second)

	originsMap := make(map[libpf.Origin]samples.KeyToEventMapping, 2)
	for _, origin := range []libpf.Origin{support.TraceOriginSampling,
		support.TraceOriginOffCPU} {
		originsMap[origin] = make(samples.KeyToEventMapping)
	}
	executables, err :=
		freelru.NewSynced[libpf.FileID, samples.ExecInfo](
			cfg.ExecutablesCacheElements,
			libpf.FileID.Hash32,
		)
	if err != nil {
		return nil, err
	}
	executables.SetLifetime(ExecutableCacheLifetime)
	executables.SetOnEvict(func(f libpf.FileID, ei samples.ExecInfo) {
		log.Log("msg", "evicting executable", "f", f.StringNoQuotes(), "n", ei.FileName, "id", ei.GnuBuildID)
	})
	frames, err := freelru.NewSynced[
		libpf.FrameID,
		samples.SourceInfo,
	](
		cfg.FramesCacheElements, libpf.FrameID.Hash32)
	if err != nil {
		return nil, err
	}
	frames.SetLifetime(FramesCacheLifetime)

	return &PPROFReporter{
		cfg:         cfg,
		log:         log,
		cgroups:     cgroups,
		traceEvents: xsync.NewRWMutex(originsMap),
		Executables: executables,
		Frames:      frames,
		sd:          sd,
	}, nil
}

func (p *PPROFReporter) ReportTraceEvent(trace *libpf.Trace, meta *samples.TraceEventMeta) error {
	if meta.Origin != support.TraceOriginSampling && meta.Origin != support.TraceOriginOffCPU {
		return nil
	}

	key := samples.TraceAndMetaKey{
		Hash:           trace.Hash,
		Comm:           "",
		ProcessName:    "",
		ExecutablePath: "",
		ApmServiceName: "",
		ContainerID:    "",
		Pid:            int64(meta.PID),
		ExtraMeta:      nil,
	}

	traceEventsMap := p.traceEvents.WLock()
	defer p.traceEvents.WUnlock(&traceEventsMap)

	if events, exists := (*traceEventsMap)[meta.Origin][key]; exists {
		events.Timestamps = append(events.Timestamps, uint64(meta.Timestamp))
		events.OffTimes = append(events.OffTimes, meta.OffTime)
		(*traceEventsMap)[meta.Origin][key] = events
		return nil
	}

	(*traceEventsMap)[meta.Origin][key] = &samples.TraceEvents{
		Files:              trace.Files,
		Linenos:            trace.Linenos,
		FrameTypes:         trace.FrameTypes,
		MappingStarts:      trace.MappingStart,
		MappingEnds:        trace.MappingEnd,
		MappingFileOffsets: trace.MappingFileOffsets,
		Timestamps:         []uint64{uint64(meta.Timestamp)},
		OffTimes:           []int64{meta.OffTime},
	}
	return nil
}

func (p *PPROFReporter) SupportsReportTraceEvent() bool {
	return true
}

func (p *PPROFReporter) ExecutableKnown(fileID libpf.FileID) bool {
	_, known := p.Executables.GetAndRefresh(fileID, ExecutableCacheLifetime)
	return known
}

func (p *PPROFReporter) ExecutableMetadata(args *reporter2.ExecutableMetadataArgs) {
	lt := ExecutableCacheLifetime
	p.Executables.AddWithLifetime(args.FileID, samples.ExecInfo{
		FileName:   args.FileName,
		GnuBuildID: args.GnuBuildID,
	}, lt)
}

func (p *PPROFReporter) FrameKnown(frameID libpf.FrameID) bool {
	_, ok := p.Frames.GetAndRefresh(frameID, FramesCacheLifetime)
	return ok
}

func (p *PPROFReporter) FrameMetadata(args *reporter2.FrameMetadataArgs) {
	si := samples.SourceInfo{
		Frames: []samples.SourceInfoFrame{
			{
				LineNumber:   args.SourceLine,
				FilePath:     args.SourceFile,
				FunctionName: args.FunctionName,
			},
		},
	}
	p.Frames.Add(args.FrameID, si)
}

func (p *PPROFReporter) ReportHostMetadata(_ map[string]string) {
}

func (p *PPROFReporter) ReportHostMetadataBlocking(
	_ context.Context,
	_ map[string]string,
	_ int,
	_ time.Duration,
) error {
	return nil
}

func (p *PPROFReporter) ReportMetrics(
	_ uint32,
	_ []uint32,
	_ []int64,
) {

}

func (p *PPROFReporter) Start(ctx context.Context) error {
	ctx, cancelReporting := context.WithCancel(ctx)
	p.cancelReporting = cancelReporting
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()
		tick := time.NewTicker(p.cfg.ReportInterval)
		defer tick.Stop()
		purgeTick := time.NewTicker(5 * time.Minute)
		defer purgeTick.Stop()
		purge := false
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				p.reportProfile(ctx)
				if purge {
					p.Executables.PurgeExpired()
					p.Frames.PurgeExpired()
					p.cgroups.PurgeExpired()
					purge = false
				}
			case <-purgeTick.C:
				purge = true
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
	traceEvents := p.traceEvents.WLock()
	events := make(map[libpf.Origin]samples.KeyToEventMapping, 2)
	for _, origin := range []libpf.Origin{support.TraceOriginSampling,
		support.TraceOriginOffCPU} {
		events[origin] = maps.Clone((*traceEvents)[origin])
		clear((*traceEvents)[origin])
	}
	p.traceEvents.WUnlock(&traceEvents)

	var profiles []PPROF
	for _, origin := range []libpf.Origin{support.TraceOriginSampling,
		support.TraceOriginOffCPU} {
		originEvents := events[origin]
		if len(originEvents) == 0 {
			continue
		}
		pp := p.createProfile(origin, originEvents)
		profiles = append(profiles, pp...)
	}

	p.cfg.Consumer.ConsumePprofProfiles(ctx, profiles)
	sz := 0
	for _, it := range profiles {
		sz += len(it.Raw)
	}
	_ = level.Debug(p.log).Log("msg", "pprof report successful", "count", len(profiles), "total-size", sz)
}

func (p *PPROFReporter) createProfile(
	origin libpf.Origin,
	events map[samples.TraceAndMetaKey]*samples.TraceEvents,
) []PPROF {
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
		target := p.sd.FindTarget(uint32(traceKey.Pid))
		if target == nil {
			continue
		}
		b := bs.BuilderForSample(target, uint32(traceKey.Pid))

		s := b.NewSample(len(traceInfo.FrameTypes))

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

		// Walk every frame of the trace.
		for i := range traceInfo.FrameTypes {
			fileID := traceInfo.Files[i]
			addrOrLineNo := traceInfo.Linenos[i]
			frameID := libpf.NewFrameID(fileID, addrOrLineNo)
			location, locationFresh := b.Location(frameID)
			if locationFresh {
				location.Address = uint64(addrOrLineNo)
				switch frameKind := traceInfo.FrameTypes[i]; frameKind {
				case libpf.NativeFrame:
					mapping, mappingFresh := b.Mapping(traceInfo.Files[i])
					if mappingFresh {
						ei, exists := p.Executables.GetAndRefresh(traceInfo.Files[i],
							ExecutableCacheLifetime)

						var fileName = "UNKNOWN"
						if exists {
							fileName = ei.FileName
						}
						mapping.Start = uint64(traceInfo.MappingStarts[i])
						mapping.Limit = uint64(traceInfo.MappingEnds[i])
						mapping.Offset = traceInfo.MappingFileOffsets[i]
						mapping.File = fileName
						mapping.BuildID = ei.GnuBuildID
					}
					location.Mapping = mapping
					p.symbolizeNativeFrame(b, location, traceInfo, i)
				case libpf.AbortFrame:
					// Next step: Figure out how the OTLP protocol
					// could handle artificial frames, like AbortFrame,
					// that are not originated from a native or interpreted
					// program.
				default:
					var funcName string
					var filePath string
					var lineNo int64
					if si, exists := p.Frames.GetAndRefresh(frameID, FramesCacheLifetime); exists {
						if len(si.Frames) == 1 {
							fr := si.Frames[0]
							funcName = fr.FunctionName
							filePath = fr.FilePath
							lineNo = int64(fr.LineNumber)
						} else {
							funcName = "UNRESOLVED2"
						}
					} else {
						funcName = "UNRESOLVED"
					}
					location.Line = []profile.Line{{
						Line:     lineNo,
						Function: b.Function(funcName, filePath)},
					}
					location.Mapping.HasFunctions = true
				}
			}
			if traceInfo.FrameTypes[i] == libpf.PythonFrame && len(location.Line) == 1 && location.Line[0].Function.Name == "<interpreter trampoline>" {
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
	traceInfo *samples.TraceEvents,
	i int,
) {
	if loc.Mapping.File == process.VdsoPathName {
		return
	}
	if p.cfg.ExtraNativeSymbolResolver == nil {
		return
	}
	fileID := traceInfo.Files[i]
	addr := traceInfo.Linenos[i]
	frameID := libpf.NewFrameID(fileID, addr)

	irsymcache.SymbolizeNativeFrame(p.cfg.ExtraNativeSymbolResolver, p.Frames, loc.Mapping.File, frameID, func(si samples.SourceInfo) {
		if len(si.Frames) > 0 {
			loc.Mapping.HasFunctions = true
		}
		for _, fn := range si.Frames {
			line := profile.Line{Function: b.Function(fn.FunctionName, fn.FilePath)}
			line.Line = int64(fn.LineNumber)
			loc.Line = append(loc.Line, line)
		}
	})
}

const (
	ExecutableCacheLifetime = 1 * time.Hour
	FramesCacheLifetime     = 1 * time.Hour
)
