package scrape

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/alloy/logging/level"
	"github.com/grafana/alloy/internal/component"
	component_config "github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/useragent"
	client_prometheus "github.com/prometheus/client_golang/prometheus"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
)

func init() {
	scrape.UserAgent = useragent.Get()

	component.Register(component.Registration{
		Name:      "prometheus.scrape",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the prometheus.scrape
// component.
type Arguments struct {
	Targets   []discovery.Target   `alloy:"targets,attr"`
	ForwardTo []storage.Appendable `alloy:"forward_to,attr"`

	// The job name to override the job label with.
	JobName string `alloy:"job_name,attr,optional"`
	// Indicator whether the scraped metrics should remain unmodified.
	HonorLabels bool `alloy:"honor_labels,attr,optional"`
	// Indicator whether the scraped timestamps should be respected.
	HonorTimestamps bool `alloy:"honor_timestamps,attr,optional"`
	// Indicator whether to track the staleness of the scraped timestamps.
	TrackTimestampsStaleness bool `alloy:"track_timestamps_staleness,attr,optional"`
	// A set of query parameters with which the target is scraped.
	Params url.Values `alloy:"params,attr,optional"`
	// Whether to scrape a classic histogram that is also exposed as a native histogram.
	ScrapeClassicHistograms bool `alloy:"scrape_classic_histograms,attr,optional"`
	// How frequently to scrape the targets of this scrape config.
	ScrapeInterval time.Duration `alloy:"scrape_interval,attr,optional"`
	// The timeout for scraping targets of this config.
	ScrapeTimeout time.Duration `alloy:"scrape_timeout,attr,optional"`
	// The HTTP resource path on which to fetch metrics from targets.
	MetricsPath string `alloy:"metrics_path,attr,optional"`
	// The URL scheme with which to fetch metrics from targets.
	Scheme string `alloy:"scheme,attr,optional"`
	// An uncompressed response body larger than this many bytes will cause the
	// scrape to fail. 0 means no limit.
	BodySizeLimit units.Base2Bytes `alloy:"body_size_limit,attr,optional"`
	// More than this many samples post metric-relabeling will cause the scrape
	// to fail.
	SampleLimit uint `alloy:"sample_limit,attr,optional"`
	// More than this many targets after the target relabeling will cause the
	// scrapes to fail.
	TargetLimit uint `alloy:"target_limit,attr,optional"`
	// More than this many labels post metric-relabeling will cause the scrape
	// to fail.
	LabelLimit uint `alloy:"label_limit,attr,optional"`
	// More than this label name length post metric-relabeling will cause the
	// scrape to fail.
	LabelNameLengthLimit uint `alloy:"label_name_length_limit,attr,optional"`
	// More than this label value length post metric-relabeling will cause the
	// scrape to fail.
	LabelValueLengthLimit uint `alloy:"label_value_length_limit,attr,optional"`

	HTTPClientConfig component_config.HTTPClientConfig `alloy:",squash"`

	// Scrape Options
	ExtraMetrics              bool `alloy:"extra_metrics,attr,optional"`
	EnableProtobufNegotiation bool `alloy:"enable_protobuf_negotiation,attr,optional"`

	Clustering cluster.ComponentBlock `alloy:"clustering,block,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (arg *Arguments) SetToDefault() {
	*arg = Arguments{
		MetricsPath:              "/metrics",
		Scheme:                   "http",
		HonorLabels:              false,
		HonorTimestamps:          true,
		TrackTimestampsStaleness: false,
		HTTPClientConfig:         component_config.DefaultHTTPClientConfig,
		ScrapeInterval:           1 * time.Minute,  // From config.DefaultGlobalConfig
		ScrapeTimeout:            10 * time.Second, // From config.DefaultGlobalConfig
	}
}

// Validate implements syntax.Validator.
func (arg *Arguments) Validate() error {
	if arg.ScrapeTimeout > arg.ScrapeInterval {
		return fmt.Errorf("scrape_timeout (%s) greater than scrape_interval (%s) for scrape config with job name %q", arg.ScrapeTimeout, arg.ScrapeInterval, arg.JobName)
	}

	// We must explicitly Validate because HTTPClientConfig is squashed and it won't run otherwise
	return arg.HTTPClientConfig.Validate()
}

// Component implements the prometheus.scrape component.
type Component struct {
	opts    component.Options
	cluster cluster.Cluster

	reloadTargets chan struct{}

	mut          sync.RWMutex
	args         Arguments
	scraper      *scrape.Manager
	appendable   *prometheus.Fanout
	targetsGauge client_prometheus.Gauge
}

var (
	_ component.Component = (*Component)(nil)
)

// New creates a new prometheus.scrape component.
func New(o component.Options, args Arguments) (*Component, error) {
	data, err := o.GetServiceData(http.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get information about HTTP server: %w", err)
	}
	httpData := data.(http.Data)

	data, err = o.GetServiceData(cluster.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get information about cluster: %w", err)
	}
	clusterData := data.(cluster.Cluster)

	service, err := o.GetServiceData(labelstore.ServiceName)
	if err != nil {
		return nil, err
	}
	ls := service.(labelstore.LabelStore)

	alloyAppendable := prometheus.NewFanout(args.ForwardTo, o.ID, o.Registerer, ls)
	scrapeOptions := &scrape.Options{
		ExtraMetrics: args.ExtraMetrics,
		HTTPClientOptions: []config_util.HTTPClientOption{
			config_util.WithDialContextFunc(httpData.DialFunc),
		},
		EnableProtobufNegotiation: args.EnableProtobufNegotiation,
	}
	scraper := scrape.NewManager(scrapeOptions, o.Logger, alloyAppendable)

	targetsGauge := client_prometheus.NewGauge(client_prometheus.GaugeOpts{
		Name: "prometheus_scrape_targets_gauge",
		Help: "Number of targets this component is configured to scrape"})
	err = o.Registerer.Register(targetsGauge)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:          o,
		cluster:       clusterData,
		reloadTargets: make(chan struct{}, 1),
		scraper:       scraper,
		appendable:    alloyAppendable,
		targetsGauge:  targetsGauge,
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
		err := c.scraper.Run(targetSetsChan)
		level.Info(c.opts.Logger).Log("msg", "scrape manager stopped")
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "scrape manager failed", "err", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.reloadTargets:
			c.mut.RLock()
			var (
				targets = c.args.Targets
				jobName = c.opts.ID
			)
			if c.args.JobName != "" {
				jobName = c.args.JobName
			}
			c.mut.RUnlock()

			promTargets := c.distTargets(c.args.Clustering.Enabled, targets, jobName)

			select {
			case targetSetsChan <- promTargets:
				level.Debug(c.opts.Logger).Log("msg", "passed new targets to scrape manager")
			case <-ctx.Done():
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

	sc := getPromScrapeConfigs(c.opts.ID, newArgs)
	err := c.scraper.ApplyConfig(&config.Config{
		ScrapeConfigs: []*config.ScrapeConfig{sc},
	})
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

// Helper function to bridge the in-house configuration with the Prometheus
// scrape_config.
// As explained in the Config struct, the following fields are purposefully
// missing out, as they're being implemented by another components.
// - RelabelConfigs
// - MetricsRelabelConfigs
// - ServiceDiscoveryConfigs
func getPromScrapeConfigs(jobName string, c Arguments) *config.ScrapeConfig {
	dec := config.DefaultScrapeConfig
	if c.JobName != "" {
		dec.JobName = c.JobName
	} else {
		dec.JobName = jobName
	}
	dec.HonorLabels = c.HonorLabels
	dec.HonorTimestamps = c.HonorTimestamps
	dec.TrackTimestampsStaleness = c.TrackTimestampsStaleness
	dec.Params = c.Params
	dec.ScrapeClassicHistograms = c.ScrapeClassicHistograms
	dec.ScrapeInterval = model.Duration(c.ScrapeInterval)
	dec.ScrapeTimeout = model.Duration(c.ScrapeTimeout)
	dec.MetricsPath = c.MetricsPath
	dec.Scheme = c.Scheme
	dec.BodySizeLimit = c.BodySizeLimit
	dec.SampleLimit = c.SampleLimit
	dec.TargetLimit = c.TargetLimit
	dec.LabelLimit = c.LabelLimit
	dec.LabelNameLengthLimit = c.LabelNameLengthLimit
	dec.LabelValueLengthLimit = c.LabelValueLengthLimit

	// HTTP scrape client settings
	dec.HTTPClientConfig = *c.HTTPClientConfig.Convert()
	return &dec
}

func (c *Component) distTargets(
	clusteringEnabled bool,
	targets []discovery.Target,
	jobName string,
) map[string][]*targetgroup.Group {
	// NOTE(@tpaschalis) First approach, manually building the
	// 'clustered' targets implementation every time.
	dt := discovery.NewDistributedTargets(clusteringEnabled, c.cluster, targets)
	alloyTargets := dt.LocalTargets()
	c.targetsGauge.Set(float64(len(alloyTargets)))
	promTargets := c.componentTargetsToProm(jobName, alloyTargets)
	return promTargets
}

// ScraperStatus reports the status of the scraper's jobs.
type ScraperStatus struct {
	TargetStatus []TargetStatus `alloy:"target,block,optional"`
}

// TargetStatus reports on the status of the latest scrape for a target.
type TargetStatus struct {
	JobName            string            `alloy:"job,attr"`
	URL                string            `alloy:"url,attr"`
	Health             string            `alloy:"health,attr"`
	Labels             map[string]string `alloy:"labels,attr"`
	LastError          string            `alloy:"last_error,attr,optional"`
	LastScrape         time.Time         `alloy:"last_scrape,attr"`
	LastScrapeDuration time.Duration     `alloy:"last_scrape_duration,attr,optional"`
}

// BuildTargetStatuses transforms the targets from a scrape manager into our internal status type for debug info.
func BuildTargetStatuses(targets map[string][]*scrape.Target) []TargetStatus {
	var res []TargetStatus

	for job, stt := range targets {
		for _, st := range stt {
			var lastError string
			if st.LastError() != nil {
				lastError = st.LastError().Error()
			}
			if st != nil {
				res = append(res, TargetStatus{
					JobName:            job,
					URL:                st.URL().String(),
					Health:             string(st.Health()),
					Labels:             st.Labels().Map(),
					LastError:          lastError,
					LastScrape:         st.LastScrape(),
					LastScrapeDuration: st.LastScrapeDuration(),
				})
			}
		}
	}
	return res
}

// DebugInfo implements component.DebugComponent
func (c *Component) DebugInfo() interface{} {
	return ScraperStatus{
		TargetStatus: BuildTargetStatuses(c.scraper.TargetsActive()),
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
