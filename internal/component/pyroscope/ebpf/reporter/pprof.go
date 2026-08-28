//go:build unix

package reporter

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/google/pprof/profile"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/reporter/args"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/symb/irsymcache"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/xsync"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/reporter/samples"
	"go.opentelemetry.io/ebpf-profiler/traceutil"
)

var vdsoPathName = libpf.Intern(process.VdsoPathName)

type PPROF struct {
	Raw    []byte
	Labels labels.Labels
}

type PPROFConsumer func(ctx context.Context, p []PPROF)

type Config struct {
	ReportInterval            time.Duration
	SamplesPerSecond          int64
	Demangle                  string
	ReporterUnsymbolizedStubs bool
	PIDLabel                  bool
	CommMode                  args.CommMode
	KernelFrames              bool
}
type PPROFReporter struct {
	cfg *Config
	log *slog.Logger

	consumer PPROFConsumer
	symbols  irsymcache.NativeSymbolResolver

	traceEvents xsync.RWMutex[samples.TraceEventsTree]

	sd discovery.TargetProducer

	wg              sync.WaitGroup
	cancelReporting context.CancelFunc
}

func NewPPROF(log *slog.Logger,
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

func (p *PPROFReporter) ReportTraceEvent(trace *libpf.Trace, meta *samples.TraceEventMeta) error {
	if meta.ProfileType == nil {
		return fmt.Errorf("skip reporting trace: profile type is nil")
	}

	key := samples.ResourceKey{
		APMServiceName: meta.APMServiceName,
		ContainerID:    meta.ContainerID,
		PID:            int64(meta.PID),
		ExecutablePath: meta.ExecutablePath,
	}

	eventsTree := p.traceEvents.WLock()
	defer p.traceEvents.WUnlock(&eventsTree)

	if _, exists := (*eventsTree)[key]; !exists {
		(*eventsTree)[key] = samples.ResourceToProfiles{
			EnvVars: meta.EnvVars,
			Events:  make(map[*samples.TypeMetadata]samples.SampleToEvents),
		}
	}

	rtp := (*eventsTree)[key]
	if _, exists := rtp.Events[meta.ProfileType]; !exists {
		rtp.Events[meta.ProfileType] = make(samples.SampleToEvents)
	}

	sampleKey := samples.SampleKey{
		Hash:    traceutil.HashTrace(trace),
		Comm:    meta.Comm,
		TID:     int64(meta.TID),
		CPU:     int64(meta.CPU),
		SpanID:  meta.SpanID,
		TraceID: meta.TraceID,
	}
	if events, exists := rtp.Events[meta.ProfileType][sampleKey]; exists {
		events.Timestamps = append(events.Timestamps, uint64(meta.Timestamp))
		events.Values = append(events.Values, meta.Value)
		return nil
	}

	rtp.Events[meta.ProfileType][sampleKey] = &samples.TraceEvents{
		Frames:     trace.Frames,
		Timestamps: []uint64{uint64(meta.Timestamp)},
		Values:     []int64{meta.Value},
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
	for resourceKey, rtp := range reportedEvents {
		for profileType, events := range rtp.Events {
			pp := p.createProfile(resourceKey, profileType, events)
			profiles = append(profiles, pp...)
		}
	}

	p.consumer(ctx, profiles)
	sz := 0
	for _, it := range profiles {
		sz += len(it.Raw)
	}
	p.log.Debug("pprof report successful", "count", len(profiles), "total-size", sz)
}

func (p *PPROFReporter) createProfile(resourceKey samples.ResourceKey, profileType *samples.TypeMetadata, events map[samples.SampleKey]*samples.TraceEvents) []PPROF {
	defer func() {
		if p.symbols != nil {
			p.symbols.Cleanup()
		}
	}()

	bs := NewProfileBuilders(BuildersOptions{
		SampleRate:    p.cfg.SamplesPerSecond,
		PerPIDProfile: true,
		ProfileType:   profileType,
	})

	for sampleKey, traceInfo := range events {
		target := p.sd.FindTarget(uint32(resourceKey.PID), resourceKey.ContainerID.String())
		if target == nil {
			continue
		}
		b, err := bs.BuilderForSample(target, uint32(resourceKey.PID))
		if err != nil {
			p.log.Error("failed to create profile builder", "err", err)
			return nil
		}
		fakeMapping := b.FakeMapping()

		s := b.NewSample(len(traceInfo.Frames))
		comm := sampleKey.Comm.String()
		sampleLabels := map[string][]string{}
		if comm != "" && p.cfg.CommMode.Label() {
			sampleLabels["comm"] = []string{comm}
		}
		if p.cfg.PIDLabel {
			sampleLabels["pid"] = []string{strconv.FormatInt(resourceKey.PID, 10)}
		}
		if sampleKey.SpanID != libpf.InvalidAPMSpanID {
			sampleLabels["span_id"] = []string{hex.EncodeToString(sampleKey.SpanID[:])}
		}
		if sampleKey.TraceID != libpf.InvalidAPMTraceID {
			sampleLabels["trace_id"] = []string{hex.EncodeToString(sampleKey.TraceID[:])}
		}
		if len(sampleLabels) > 0 {
			s.Label = sampleLabels
		}

		if profileType.ReportValues {
			sum := int64(0)
			for _, t := range traceInfo.Values {
				sum += t
			}
			b.AddValue(sum, s)
		} else {
			b.AddValue(int64(len(traceInfo.Timestamps)), s)
		}

		for i := range traceInfo.Frames {
			fr := traceInfo.Frames[i].Value()
			if !p.cfg.KernelFrames && fr.Type == libpf.KernelFrame {
				continue
			}
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
		if comm != "" && p.cfg.CommMode.Stackframe() {
			s.Location = append(s.Location, b.CommLocation(comm))
		}
	}
	res := make([]PPROF, 0, len(bs.Builders))
	for _, b := range bs.Builders {
		buf := bytes.NewBuffer(nil)
		_, err := b.Write(buf)
		if err != nil {
			p.log.Error("failed to encode profile", "err", err)
			continue
		}
		_, ls := b.Target.Labels()
		metric := discovery.MetricValueProcessCPU
		if profileType.ReportValues {
			metric = discovery.MetricValueOffCPU
		}

		builder := labels.NewBuilder(ls)
		builder.Set(model.MetricNameLabel, metric)
		pyroscope.AddScopeLabels(builder, pyroscope.ScopeNameEBPF)
		res = append(res, PPROF{
			Raw:    buf.Bytes(),
			Labels: builder.Labels(),
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
	if mappingFile.FileName == vdsoPathName {
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
