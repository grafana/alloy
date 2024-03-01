package scrape

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/grafana/agent/internal/component/pyroscope"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/flow/logging/level"
	"github.com/grafana/agent/internal/service/cluster"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"

	"github.com/grafana/agent/internal/component"
	component_config "github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/prometheus/scrape"
)

const (
	pprofMemory            string = "memory"
	pprofBlock             string = "block"
	pprofGoroutine         string = "goroutine"
	pprofMutex             string = "mutex"
	pprofProcessCPU        string = "process_cpu"
	pprofFgprof            string = "fgprof"
	pprofGoDeltaProfMemory string = "godeltaprof_memory"
	pprofGoDeltaProfBlock  string = "godeltaprof_block"
	pprofGoDeltaProfMutex  string = "godeltaprof_mutex"
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.scrape",
		Stability: featuregate.StabilityBeta,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the pprof.scrape
// component.
type Arguments struct {
	Targets   []discovery.Target     `river:"targets,attr"`
	ForwardTo []pyroscope.Appendable `river:"forward_to,attr"`

	// The job name to override the job label with.
	JobName string `river:"job_name,attr,optional"`
	// A set of query parameters with which the target is scraped.
	Params url.Values `river:"params,attr,optional"`
	// How frequently to scrape the targets of this scrape config.
	ScrapeInterval time.Duration `river:"scrape_interval,attr,optional"`
	// The timeout for scraping targets of this config.
	ScrapeTimeout time.Duration `river:"scrape_timeout,attr,optional"`
	// The URL scheme with which to fetch metrics from targets.
	Scheme string `river:"scheme,attr,optional"`

	// todo(ctovena): add support for limits.
	// // An uncompressed response body larger than this many bytes will cause the
	// // scrape to fail. 0 means no limit.
	// BodySizeLimit units.Base2Bytes `river:"body_size_limit,attr,optional"`
	// // More than this many targets after the target relabeling will cause the
	// // scrapes to fail.
	// TargetLimit uint `river:"target_limit,attr,optional"`
	// // More than this many labels post metric-relabeling will cause the scrape
	// // to fail.
	// LabelLimit uint `river:"label_limit,attr,optional"`
	// // More than this label name length post metric-relabeling will cause the
	// // scrape to fail.
	// LabelNameLengthLimit uint `river:"label_name_length_limit,attr,optional"`
	// // More than this label value length post metric-relabeling will cause the
	// // scrape to fail.
	// LabelValueLengthLimit uint `river:"label_value_length_limit,attr,optional"`

	HTTPClientConfig component_config.HTTPClientConfig `river:",squash"`

	ProfilingConfig ProfilingConfig `river:"profiling_config,block,optional"`

	Clustering cluster.ComponentBlock `river:"clustering,block,optional"`
}

type ProfilingConfig struct {
	Memory            ProfilingTarget         `river:"profile.memory,block,optional"`
	Block             ProfilingTarget         `river:"profile.block,block,optional"`
	Goroutine         ProfilingTarget         `river:"profile.goroutine,block,optional"`
	Mutex             ProfilingTarget         `river:"profile.mutex,block,optional"`
	ProcessCPU        ProfilingTarget         `river:"profile.process_cpu,block,optional"`
	FGProf            ProfilingTarget         `river:"profile.fgprof,block,optional"`
	GoDeltaProfMemory ProfilingTarget         `river:"profile.godeltaprof_memory,block,optional"`
	GoDeltaProfMutex  ProfilingTarget         `river:"profile.godeltaprof_mutex,block,optional"`
	GoDeltaProfBlock  ProfilingTarget         `river:"profile.godeltaprof_block,block,optional"`
	Custom            []CustomProfilingTarget `river:"profile.custom,block,optional"`

	PprofPrefix string `river:"path_prefix,attr,optional"`
}

// AllTargets returns the set of all standard and custom profiling targets,
// regardless of whether they're enabled. The key in the map indicates the name
// of the target.
func (cfg *ProfilingConfig) AllTargets() map[string]ProfilingTarget {
	targets := map[string]ProfilingTarget{
		pprofMemory:            cfg.Memory,
		pprofBlock:             cfg.Block,
		pprofGoroutine:         cfg.Goroutine,
		pprofMutex:             cfg.Mutex,
		pprofProcessCPU:        cfg.ProcessCPU,
		pprofFgprof:            cfg.FGProf,
		pprofGoDeltaProfMemory: cfg.GoDeltaProfMemory,
		pprofGoDeltaProfMutex:  cfg.GoDeltaProfMutex,
		pprofGoDeltaProfBlock:  cfg.GoDeltaProfBlock,
	}

	for _, custom := range cfg.Custom {
		targets[custom.Name] = ProfilingTarget{
			Enabled: custom.Enabled,
			Path:    custom.Path,
			Delta:   custom.Delta,
		}
	}

	return targets
}

var DefaultProfilingConfig = ProfilingConfig{
	Memory: ProfilingTarget{
		Enabled: true,
		Path:    "/debug/pprof/allocs",
	},
	Block: ProfilingTarget{
		Enabled: true,
		Path:    "/debug/pprof/block",
	},
	Goroutine: ProfilingTarget{
		Enabled: true,
		Path:    "/debug/pprof/goroutine",
	},
	Mutex: ProfilingTarget{
		Enabled: true,
		Path:    "/debug/pprof/mutex",
	},
	ProcessCPU: ProfilingTarget{
		Enabled: true,
		Path:    "/debug/pprof/profile",
		Delta:   true,
	},
	FGProf: ProfilingTarget{
		Enabled: false,
		Path:    "/debug/fgprof",
		Delta:   true,
	},
	// https://github.com/grafana/godeltaprof/blob/main/http/pprof/pprof.go#L21
	GoDeltaProfMemory: ProfilingTarget{
		Enabled: false,
		Path:    "/debug/pprof/delta_heap",
	},
	GoDeltaProfMutex: ProfilingTarget{
		Enabled: false,
		Path:    "/debug/pprof/delta_mutex",
	},
	GoDeltaProfBlock: ProfilingTarget{
		Enabled: false,
		Path:    "/debug/pprof/delta_block",
	},
}

// SetToDefault implements river.Defaulter.
func (cfg *ProfilingConfig) SetToDefault() {
	*cfg = DefaultProfilingConfig
}

type ProfilingTarget struct {
	Enabled bool   `river:"enabled,attr,optional"`
	Path    string `river:"path,attr,optional"`
	Delta   bool   `river:"delta,attr,optional"`
}

type CustomProfilingTarget struct {
	Enabled bool   `river:"enabled,attr"`
	Path    string `river:"path,attr"`
	Delta   bool   `river:"delta,attr,optional"`
	Name    string `river:",label"`
}

var DefaultArguments = NewDefaultArguments()

// NewDefaultArguments create the default settings for a scrape job.
func NewDefaultArguments() Arguments {
	return Arguments{
		Scheme:           "http",
		HTTPClientConfig: component_config.DefaultHTTPClientConfig,
		ScrapeInterval:   15 * time.Second,
		ScrapeTimeout:    10 * time.Second,
		ProfilingConfig:  DefaultProfilingConfig,
	}
}

// SetToDefault implements river.Defaulter.
func (arg *Arguments) SetToDefault() {
	*arg = NewDefaultArguments()
}

// Validate implements river.Validator.
func (arg *Arguments) Validate() error {
	if arg.ScrapeTimeout.Seconds() <= 0 {
		return fmt.Errorf("scrape_timeout must be greater than 0")
	}

	// ScrapeInterval must be at least 2 seconds, because if
	// ProfilingTarget.Delta is true the ScrapeInterval - 1s is propagated in
	// the `seconds` parameter and it must be >= 1.
	for _, target := range arg.ProfilingConfig.AllTargets() {
		if target.Enabled && target.Delta && arg.ScrapeInterval.Seconds() < 2 {
			return fmt.Errorf("scrape_interval must be at least 2 seconds when using delta profiling")
		}
	}

	// We must explicitly Validate because HTTPClientConfig is squashed and it won't run otherwise
	return arg.HTTPClientConfig.Validate()
}

// Component implements the pprof.scrape component.
type Component struct {
	opts    component.Options
	cluster cluster.Cluster

	reloadTargets chan struct{}

	mut        sync.RWMutex
	args       Arguments
	scraper    *Manager
	appendable *pyroscope.Fanout
}

var _ component.Component = (*Component)(nil)

// New creates a new pprof.scrape component.
func New(o component.Options, args Arguments) (*Component, error) {
	data, err := o.GetServiceData(cluster.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get info about cluster service: %w", err)
	}
	clusterData := data.(cluster.Cluster)

	flowAppendable := pyroscope.NewFanout(args.ForwardTo, o.ID, o.Registerer)
	scraper := NewManager(flowAppendable, o.Logger)
	c := &Component{
		opts:          o,
		cluster:       clusterData,
		reloadTargets: make(chan struct{}, 1),
		scraper:       scraper,
		appendable:    flowAppendable,
	}

	// Call to Update() to set the receivers and targets once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer c.scraper.Stop()

	targetSetsChan := make(chan map[string][]*targetgroup.Group)

	go func() {
		c.scraper.Run(targetSetsChan)
		level.Info(c.opts.Logger).Log("msg", "scrape manager stopped")
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.reloadTargets:
			c.mut.RLock()
			var (
				tgs        = c.args.Targets
				jobName    = c.opts.ID
				clustering = c.args.Clustering.Enabled
			)
			if c.args.JobName != "" {
				jobName = c.args.JobName
			}
			c.mut.RUnlock()

			// NOTE(@tpaschalis) First approach, manually building the
			// 'clustered' targets implementation every time.
			ct := discovery.NewDistributedTargets(clustering, c.cluster, tgs)
			promTargets := c.componentTargetsToProm(jobName, ct.Get())

			select {
			case targetSetsChan <- promTargets:
				level.Debug(c.opts.Logger).Log("msg", "passed new targets to scrape manager")
			case <-ctx.Done():
				return nil
			}
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()
	c.args = newArgs

	c.appendable.UpdateChildren(newArgs.ForwardTo)

	err := c.scraper.ApplyConfig(newArgs)
	if err != nil {
		return fmt.Errorf("error applying scrape configs: %w", err)
	}
	level.Debug(c.opts.Logger).Log("msg", "scrape config was updated")

	select {
	case c.reloadTargets <- struct{}{}:
	default:
	}

	return nil
}

// NotifyClusterChange implements component.ClusterComponent.
func (c *Component) NotifyClusterChange() {
	c.mut.RLock()
	defer c.mut.RUnlock()

	if !c.args.Clustering.Enabled {
		return // no-op
	}

	// Schedule a reload so targets get redistributed.
	select {
	case c.reloadTargets <- struct{}{}:
	default:
	}
}

func (c *Component) componentTargetsToProm(jobName string, tgs []discovery.Target) map[string][]*targetgroup.Group {
	promGroup := &targetgroup.Group{Source: jobName}
	for _, tg := range tgs {
		promGroup.Targets = append(promGroup.Targets, convertLabelSet(tg))
	}

	return map[string][]*targetgroup.Group{jobName: {promGroup}}
}

func convertLabelSet(tg discovery.Target) model.LabelSet {
	lset := make(model.LabelSet, len(tg))
	for k, v := range tg {
		lset[model.LabelName(k)] = model.LabelValue(v)
	}
	return lset
}

// DebugInfo implements component.DebugComponent.
func (c *Component) DebugInfo() interface{} {
	var res []scrape.TargetStatus

	for job, stt := range c.scraper.TargetsActive() {
		for _, st := range stt {
			var lastError string
			if st.LastError() != nil {
				lastError = st.LastError().Error()
			}
			if st != nil {
				res = append(res, scrape.TargetStatus{
					JobName:            job,
					URL:                st.URL(),
					Health:             string(st.Health()),
					Labels:             st.discoveredLabels.Map(),
					LastError:          lastError,
					LastScrape:         st.LastScrape(),
					LastScrapeDuration: st.LastScrapeDuration(),
				})
			}
		}
	}

	return scrape.ScraperStatus{TargetStatus: res}
}
