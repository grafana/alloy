// Package relabel provides in-profile label manipulation for pprof samples.
package relabel

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"sync"

	debuginfogrpc "buf.build/gen/go/parca-dev/parca/grpc/go/parca/debuginfo/v1alpha1/debuginfov1alpha1grpc"
	"github.com/google/pprof/profile"
	"github.com/grafana/alloy/internal/component"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	prom_relabel "github.com/prometheus/prometheus/model/relabel"
	"go.uber.org/atomic"
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.profile.relabel",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	// Where the relabeled profiles should be forwarded to.
	ForwardTo []pyroscope.Appendable `alloy:"forward_to,attr"`
	// Relabeling rules to apply to pprof sample labels.
	RelabelConfigs []*alloy_relabel.Config `alloy:"rule,block,optional"`
	// The maximum number of items to hold in the component's LRU cache.
	MaxCacheSize int `alloy:"max_cache_size,attr,optional"`
}

// DefaultArguments provides default arguments for the pyroscope.profile.relabel component.
var DefaultArguments = Arguments{
	MaxCacheSize: 10_000,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Exports holds values exported by the pyroscope.profile.relabel component.
type Exports struct {
	Receiver pyroscope.Appendable `alloy:"receiver,attr"`
	Rules    alloy_relabel.Rules  `alloy:"rules,attr"`
}

// Component implements the pyroscope.profile.relabel component.
type Component struct {
	opts         component.Options
	metrics      *metrics
	mut          sync.RWMutex
	rcs          []*prom_relabel.Config
	fanout       *pyroscope.Fanout
	cache        *lru.Cache[model.Fingerprint, []cacheItem]
	maxCacheSize int
	exited       atomic.Bool
}

var _ component.Component = (*Component)(nil)

type cacheItem struct {
	original  model.LabelSet
	relabeled model.LabelSet
	keep      bool
}

// New creates a new pyroscope.profile.relabel component.
func New(o component.Options, args Arguments) (*Component, error) {
	cache, err := lru.New[model.Fingerprint, []cacheItem](args.MaxCacheSize)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:         o,
		metrics:      newMetrics(o.Registerer),
		cache:        cache,
		maxCacheSize: args.MaxCacheSize,
	}

	c.fanout = pyroscope.NewFanout(args.ForwardTo, o.ID, o.Registerer)

	o.OnStateChange(Exports{
		Receiver: c,
		Rules:    args.RelabelConfigs,
	})

	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	defer c.exited.Store(true)
	<-ctx.Done()
	return nil
}

func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)
	newRCS := alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelConfigs)

	if relabelingChanged(c.rcs, newRCS) {
		level.Debug(c.opts.Logger).Log("msg", "received new relabel configs, purging cache")
		c.cache.Purge()
		c.metrics.cacheSize.Set(0)
	}

	if newArgs.MaxCacheSize != c.maxCacheSize {
		evicted := c.cache.Resize(newArgs.MaxCacheSize)
		if evicted > 0 {
			level.Debug(c.opts.Logger).Log("msg", "resizing cache led to evicting items", "evicted_count", evicted)
		}
		c.metrics.cacheSize.Set(float64(c.cache.Len()))
		c.maxCacheSize = newArgs.MaxCacheSize
	}

	c.rcs = newRCS
	c.fanout.UpdateChildren(newArgs.ForwardTo)

	c.opts.OnStateChange(Exports{
		Receiver: c,
		Rules:    newArgs.RelabelConfigs,
	})

	return nil
}

func (c *Component) Append(ctx context.Context, lbls labels.Labels, samples []*pyroscope.RawSample) error {
	if c.exited.Load() {
		return fmt.Errorf("%s has exited", c.opts.ID)
	}

	c.mut.RLock()
	defer c.mut.RUnlock()

	if len(samples) == 0 {
		return c.fanout.Appender().Append(ctx, lbls, samples)
	}

	c.metrics.profilesProcessed.Add(float64(len(samples)))

	out := make([]*pyroscope.RawSample, 0, len(samples))
	for _, sample := range samples {
		transformed := c.transformRawProfile(sample.RawProfile)
		c.metrics.samplesProcessed.Add(float64(transformed.samplesProcessed))
		c.metrics.samplesOutgoing.Add(float64(transformed.samplesWritten))
		c.metrics.samplesDropped.Add(float64(transformed.samplesDropped))

		if !transformed.forward {
			c.metrics.profilesDropped.Inc()
			continue
		}

		out = append(out, &pyroscope.RawSample{
			ID:         sample.ID,
			RawProfile: transformed.raw,
		})
		c.metrics.profilesOutgoing.Inc()
	}

	if len(out) == 0 {
		return nil
	}

	return c.fanout.Appender().Append(ctx, lbls, out)
}

func (c *Component) AppendIngest(ctx context.Context, profile *pyroscope.IncomingProfile) error {
	if c.exited.Load() {
		return fmt.Errorf("%s has exited", c.opts.ID)
	}

	c.mut.RLock()
	defer c.mut.RUnlock()

	c.metrics.profilesProcessed.Inc()

	transformed := c.transformRawProfile(profile.RawBody)
	c.metrics.samplesProcessed.Add(float64(transformed.samplesProcessed))
	c.metrics.samplesOutgoing.Add(float64(transformed.samplesWritten))
	c.metrics.samplesDropped.Add(float64(transformed.samplesDropped))

	if !transformed.forward {
		c.metrics.profilesDropped.Inc()
		return nil
	}

	profile.RawBody = transformed.raw
	c.metrics.profilesOutgoing.Inc()
	return c.fanout.Appender().AppendIngest(ctx, profile)
}

func (c *Component) Appender() pyroscope.Appender {
	return c
}

type transformResult struct {
	raw              []byte
	forward          bool
	samplesProcessed int
	samplesWritten   int
	samplesDropped   int
}

func (c *Component) transformRawProfile(raw []byte) transformResult {
	result := transformResult{raw: raw, forward: true}

	if len(c.rcs) == 0 {
		return result
	}

	pprofProfile, err := profile.ParseData(raw)
	if err != nil {
		c.metrics.pprofParseFailures.Inc()
		level.Debug(c.opts.Logger).Log("msg", "unable to parse pprof profile, forwarding unchanged", "err", err)
		return result
	}

	if len(pprofProfile.Sample) == 0 {
		return result
	}

	result.samplesProcessed = len(pprofProfile.Sample)

	filteredSamples := pprofProfile.Sample[:0]
	for _, sample := range pprofProfile.Sample {
		newLabels, keep := c.relabelSampleLabels(toPromLabels(sample.Label))
		if !keep {
			result.samplesDropped++
			continue
		}

		sample.Label = toSampleLabelMap(newLabels)
		if len(sample.Label) == 0 && !hasNumericLabels(sample) {
			result.samplesDropped++
			continue
		}
		filteredSamples = append(filteredSamples, sample)
		result.samplesWritten++
	}

	if len(filteredSamples) == 0 {
		result.forward = false
		return result
	}

	pprofProfile.Sample = filteredSamples

	var buf bytes.Buffer
	if err := pprofProfile.Write(&buf); err != nil {
		c.metrics.pprofWriteFailures.Inc()
		level.Debug(c.opts.Logger).Log("msg", "unable to write pprof profile, forwarding unchanged", "err", err)
		return result
	}

	result.raw = buf.Bytes()
	return result
}

func (c *Component) relabelSampleLabels(lbls labels.Labels) (labels.Labels, bool) {
	labelSet := toModelLabelSet(lbls)
	hash := labelSet.Fingerprint()

	if result, keep, found := c.getCacheEntry(hash, labelSet); found {
		return result, keep
	}

	builder := labels.NewBuilder(lbls)
	keep := prom_relabel.ProcessBuilder(builder, c.rcs...)
	if !keep {
		c.addToCache(hash, labelSet, labels.EmptyLabels(), false)
		return labels.EmptyLabels(), false
	}

	newLabels := builder.Labels()
	c.addToCache(hash, labelSet, newLabels, true)
	return newLabels, true
}

func hasNumericLabels(sample *profile.Sample) bool {
	return len(sample.NumLabel) > 0 || len(sample.NumUnit) > 0
}

func (c *Component) getCacheEntry(hash model.Fingerprint, labelSet model.LabelSet) (labels.Labels, bool, bool) {
	if val, ok := c.cache.Get(hash); ok {
		for _, item := range val {
			if labelSet.Equal(item.original) {
				c.metrics.cacheHits.Inc()
				if !item.keep {
					return labels.EmptyLabels(), false, true
				}
				return toLabelsLabels(item.relabeled), true, true
			}
		}
	}

	c.metrics.cacheMisses.Inc()
	return labels.EmptyLabels(), false, false
}

func (c *Component) addToCache(hash model.Fingerprint, original model.LabelSet, relabeled labels.Labels, keep bool) {
	var cacheValue []cacheItem
	if val, exists := c.cache.Get(hash); exists {
		cacheValue = val
	}

	cacheValue = append(cacheValue, cacheItem{
		original:  original,
		relabeled: toModelLabelSet(relabeled),
		keep:      keep,
	})
	c.cache.Add(hash, cacheValue)
	c.metrics.cacheSize.Set(float64(c.cache.Len()))
}

func relabelingChanged(prev, next []*prom_relabel.Config) bool {
	if len(prev) != len(next) {
		return true
	}
	for i := range prev {
		if !reflect.DeepEqual(prev[i], next[i]) {
			return true
		}
	}
	return false
}

func toPromLabels(sampleLabels map[string][]string) labels.Labels {
	if len(sampleLabels) == 0 {
		return labels.EmptyLabels()
	}

	builder := labels.NewScratchBuilder(len(sampleLabels))
	for key, values := range sampleLabels {
		if len(values) == 0 {
			continue
		}
		builder.Add(key, values[0])
	}
	builder.Sort()
	return builder.Labels()
}

func toSampleLabelMap(lbls labels.Labels) map[string][]string {
	if lbls.IsEmpty() {
		return nil
	}

	result := make(map[string][]string, lbls.Len())
	lbls.Range(func(l labels.Label) {
		result[l.Name] = []string{l.Value}
	})
	return result
}

func toModelLabelSet(lbls labels.Labels) model.LabelSet {
	labelSet := make(model.LabelSet, lbls.Len())
	lbls.Range(func(l labels.Label) {
		labelSet[model.LabelName(l.Name)] = model.LabelValue(l.Value)
	})
	return labelSet
}

func toLabelsLabels(ls model.LabelSet) labels.Labels {
	result := labels.NewScratchBuilder(len(ls))
	for name, value := range ls {
		result.Add(string(name), string(value))
	}
	result.Sort()
	return result.Labels()
}

func (c *Component) Upload(j debuginfo.UploadJob) {
	c.fanout.Upload(j)
}

func (c *Component) Client() debuginfogrpc.DebuginfoServiceClient {
	return c.fanout.Client()
}
