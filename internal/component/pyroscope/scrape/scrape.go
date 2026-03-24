package scrape

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"

	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/http"

	"github.com/grafana/alloy/internal/component"
	component_config "github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/scrape"
)

const (
	pprofMemory              string        = "memory"
	pprofBlock               string        = "block"
	pprofGoroutine           string        = "goroutine"
	pprofMutex               string        = "mutex"
	pprofProcessCPU          string        = "process_cpu"
	pprofFgprof              string        = "fgprof"
	pprofGoDeltaProfMemory   string        = "godeltaprof_memory"
	pprofGoDeltaProfBlock    string        = "godeltaprof_block"
	pprofGoDeltaProfMutex    string        = "godeltaprof_mutex"
	defaultScrapeInterval    time.Duration = 15 * time.Second
	defaultProfilingDuration time.Duration = defaultScrapeInterval - 1*time.Second
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.scrape",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the pprof.scrape
// component.
type Arguments struct {
	Targets   []discovery.Target     `alloy:"targets,attr"`
	ForwardTo []pyroscope.Appendable `alloy:"forward_to,attr"`

	// The job name to override the job label with.
	JobName string `alloy:"job_name,attr,optional"`
	// A set of query parameters with which the target is scraped.
	Params url.Values `alloy:"params,attr,optional"`
	// How frequently to scrape the targets of this scrape config.
	ScrapeInterval time.Duration `alloy:"scrape_interval,attr,optional"`
	// The timeout for scraping targets of this config.
	ScrapeTimeout time.Duration `alloy:"scrape_timeout,attr,optional"`
	// The URL scheme with which to fetch metrics from targets.
	Scheme string `alloy:"scheme,attr,optional"`
	// The duration for a profile to be scrapped.
	DeltaProfilingDuration time.Duration `alloy:"delta_profiling_duration,attr,optional"`

	// todo(ctovena): add support for limits.
	// // An uncompressed response body larger than this many bytes will cause the
	// // scrape to fail. 0 means no limit.
	// BodySizeLimit units.Base2Bytes `alloy:"body_size_limit,attr,optional"`
	// // More than this many targets after the target relabeling will cause the
	// // scrapes to fail.
	// TargetLimit uint `alloy:"target_limit,attr,optional"`
	// // More than this many labels post metric-relabeling will cause the scrape
	// // to fail.
	// LabelLimit uint `alloy:"label_limit,attr,optional"`
	// // More than this label name length post metric-relabeling will cause the
	// // scrape to fail.
	// LabelNameLengthLimit uint `alloy:"label_name_length_limit,attr,optional"`
	// // More than this label value length post metric-relabeling will cause the
	// // scrape to fail.
	// LabelValueLengthLimit uint `alloy:"label_value_length_limit,attr,optional"`

	HTTPClientConfig component_config.HTTPClientConfig `alloy:",squash"`

	ProfilingConfig ProfilingConfig `alloy:"profiling_config,block,optional"`

	Clustering cluster.ComponentBlock `alloy:"clustering,block,optional"`
}

type ProfilingConfig struct {
	Memory            ProfilingTarget         `alloy:"profile.memory,block,optional"`
	Block             ProfilingTarget         `alloy:"profile.block,block,optional"`
	Goroutine         ProfilingTarget         `alloy:"profile.goroutine,block,optional"`
	Mutex             ProfilingTarget         `alloy:"profile.mutex,block,optional"`
	ProcessCPU        ProfilingTarget         `alloy:"profile.process_cpu,block,optional"`
	FGProf            ProfilingTarget         `alloy:"profile.fgprof,block,optional"`
	GoDeltaProfMemory ProfilingTarget         `alloy:"profile.godeltaprof_memory,block,optional"`
	GoDeltaProfMutex  ProfilingTarget         `alloy:"profile.godeltaprof_mutex,block,optional"`
	GoDeltaProfBlock  ProfilingTarget         `alloy:"profile.godeltaprof_block,block,optional"`
	Custom            []CustomProfilingTarget `alloy:"profile.custom,block,optional"`

	PathPrefix string `alloy:"path_prefix,attr,optional"`
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

// SetToDefault implements syntax.Defaulter.
func (cfg *ProfilingConfig) SetToDefault() {
	*cfg = DefaultProfilingConfig
}

type ProfilingTarget struct {
	Enabled bool   `alloy:"enabled,attr,optional"`
	Path    string `alloy:"path,attr,optional"`
	Delta   bool   `alloy:"delta,attr,optional"`
}

type CustomProfilingTarget struct {
	Enabled bool   `alloy:"enabled,attr"`
	Path    string `alloy:"path,attr"`
	Delta   bool   `alloy:"delta,attr,optional"`
	Name    string `alloy:",label"`
}

var DefaultArguments = NewDefaultArguments()

// NewDefaultArguments create the default settings for a scrape job.
func NewDefaultArguments() Arguments {
	return Arguments{
		Scheme:                 "http",
		HTTPClientConfig:       component_config.DefaultHTTPClientConfig,
		ScrapeInterval:         15 * time.Second,
		ScrapeTimeout:          10 * time.Second,
		ProfilingConfig:        DefaultProfilingConfig,
		DeltaProfilingDuration: defaultProfilingDuration,
	}
}

// SetToDefault implements syntax.Defaulter.
func (arg *Arguments) SetToDefault() {
	*arg = NewDefaultArguments()
}

// Validate implements syntax.Validator.
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
		if target.Enabled && target.Delta {
			if arg.DeltaProfilingDuration.Seconds() <= 1 {
				return fmt.Errorf("delta_profiling_duration must be larger than 1 second when using delta profiling")
			}
			if arg.DeltaProfilingDuration.Seconds() > arg.ScrapeInterval.Seconds()-1 {
				return fmt.Errorf("delta_profiling_duration must be at least 1 second smaller than scrape_interval when using delta profiling")
			}
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
	serviceData, err := o.GetServiceData(http.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get information about HTTP server: %w", err)
	}
	httpData := serviceData.(http.Data)

	data, err := o.GetServiceData(cluster.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get info about cluster service: %w", err)
	}
	clusterData := data.(cluster.Cluster)

	alloyAppendable := pyroscope.NewFanout(args.ForwardTo, o.ID, o.Registerer)
	scrapeHttpOptions := Options{
		HTTPClientOptions: []config_util.HTTPClientOption{
			config_util.WithDialContextFunc(httpData.DialFunc),
		},
	}
	scraper, err := NewManager(scrapeHttpOptions, args, alloyAppendable, o.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create scraper manager: %w", err)
	}
	c := &Component{
		opts:          o,
		cluster:       clusterData,
		reloadTargets: make(chan struct{}, 1),
		scraper:       scraper,
		appendable:    alloyAppendable,
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

	targetSetsChan := make(chan []*targetgroup.Group)

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
				tgs               = c.args.Targets
				jobName           = c.opts.ID
				clusteringEnabled = c.args.Clustering.Enabled
			)
			if c.args.JobName != "" {
				jobName = c.args.JobName
			}
			c.mut.RUnlock()

			ct := discovery.NewDistributedTargets(clusteringEnabled, c.cluster, tgs)
			promTargets := discovery.ComponentTargetsToPromTargetGroupsForSingleJob(jobName, ct.LocalTargets())

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

// DebugInfo implements component.DebugComponent.
func (c *Component) DebugInfo() any {
	var res []scrape.TargetStatus

	for _, st := range c.scraper.TargetsActive() {
		var lastError string
		if st.LastError() != nil {
			lastError = st.LastError().Error()
		}
		if st != nil {
			res = append(res, scrape.TargetStatus{
				JobName:            c.args.JobName,
				URL:                st.URL(),
				Health:             string(st.Health()),
				Labels:             st.allLabels.Map(),
				LastError:          lastError,
				LastScrape:         st.LastScrape(),
				LastScrapeDuration: st.LastScrapeDuration(),
			})
		}
	}

	return scrape.ScraperStatus{TargetStatus: res}
}
