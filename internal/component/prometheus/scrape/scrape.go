package scrape

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/alecthomas/units"
	client_prometheus "github.com/prometheus/client_golang/prometheus"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	promlogging "github.com/prometheus/prometheus/util/logging"

	"github.com/grafana/alloy/internal/component"
	component_config "github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/useragent"
	"github.com/grafana/alloy/internal/util"
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

var (
	defaultScrapeProtocols                = convertScrapeProtocols(config.DefaultScrapeProtocols)
	defaultNativeHistogramScrapeProtocols = convertScrapeProtocols(config.DefaultProtoFirstScrapeProtocols)
)

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
	// Whether to scrape native histograms.
	ScrapeNativeHistograms bool `alloy:"scrape_native_histograms,attr,optional"`
	// File to which scrape failures are logged.
	ScrapeFailureLogFile string `alloy:"scrape_failure_log_file,attr,optional"`
	// How frequently to scrape the targets of this scrape config.
	ScrapeInterval time.Duration `alloy:"scrape_interval,attr,optional"`
	// The timeout for scraping targets of this config.
	ScrapeTimeout time.Duration `alloy:"scrape_timeout,attr,optional"`
	// The protocols to negotiate during a scrape. It tells clients what
	// protocol are accepted by Prometheus and with what order of preference.
	// Supported values (case sensitive): PrometheusProto, OpenMetricsText0.0.1,
	// OpenMetricsText1.0.0, PrometheusText0.0.4.
	ScrapeProtocols []string `alloy:"scrape_protocols,attr,optional"`
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
	ExtraMetrics bool `alloy:"extra_metrics,attr,optional"`
	// Deprecated: Use ScrapeProtocols instead. For backwards-compatibility, if this option is set to true, the
	// ScrapeProtocols will be set to [PrometheusProto, OpenMetricsText1.0.0, OpenMetricsText0.0.1, PrometheusText0.0.4].
	// It is invalid to set both EnableProtobufNegotiation and ScrapeProtocols.
	// TODO: https://github.com/grafana/alloy/issues/878: Remove this option.
	EnableProtobufNegotiation bool `alloy:"enable_protobuf_negotiation,attr,optional"`
	// The validation scheme to use for metric names.
	MetricNameValidationScheme string `alloy:"metric_name_validation_scheme,attr,optional"`
	// The escaping scheme to use for metric names.
	MetricNameEscapingScheme string `alloy:"metric_name_escaping_scheme,attr,optional"`
	// The fallback protocol to use if the target does not provide a valid Content-Type header.
	ScrapeFallbackProtocol string `alloy:"scrape_fallback_protocol,attr,optional"`
	// Whether to convert classic histograms with buckets to native histograms
	// with custom buckets (NHCB). False by default.
	ConvertClassicHistogramsToNHCB bool `alloy:"convert_classic_histograms_to_nhcb,attr,optional"`
	// Whether compression is enabled for the scrape. True by default.
	EnableCompression bool `alloy:"enable_compression,attr,optional"`
	// If there are more than this many buckets in a native histogram,
	// buckets will be merged to stay within the limit. Disabled when set to zero.
	NativeHistogramBucketLimit uint `alloy:"native_histogram_bucket_limit,attr,optional"`
	// If the growth factor of one bucket to the next is smaller than this,
	// buckets will be merged to stay within the limit. Disabled when set zero.
	NativeHistogramMinBucketFactor float64 `alloy:"native_histogram_min_bucket_factor,attr,optional"`
	// Whether the metric metadata should be passed to the downstream components.
	HonorMetadata bool `alloy:"honor_metadata,attr,optional"`

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
		ScrapeProtocols:          slices.Clone(defaultScrapeProtocols),
		ScrapeFallbackProtocol:   string(config.PrometheusText0_0_4), // Use the same fallback protocol as Prometheus v2
		ScrapeNativeHistograms:   false,
		// NOTE: the MetricNameEscapingScheme depends on this, so its default must be set in Validate() function.
		MetricNameValidationScheme:     model.LegacyValidation.String(),
		ConvertClassicHistogramsToNHCB: false,
		EnableCompression:              true,
		NativeHistogramBucketLimit:     0,
		NativeHistogramMinBucketFactor: 0,
	}
}

// Validate implements syntax.Validator.
func (arg *Arguments) Validate() error {
	if arg.ScrapeTimeout > arg.ScrapeInterval {
		return fmt.Errorf("scrape_timeout (%s) greater than scrape_interval (%s) for scrape config with job name %q", arg.ScrapeTimeout, arg.ScrapeInterval, arg.JobName)
	}

	if arg.EnableProtobufNegotiation {
		// Check if scrape_protocols is set to anything other than default and error if it is. We do not allow combining
		// the enable_protobuf_negotiation and scrape_protocols options.
		if !reflect.DeepEqual(arg.ScrapeProtocols, defaultScrapeProtocols) {
			return fmt.Errorf("both enable_protobuf_negotiation and scrape_protocols are set, only one can be set at a time")
		}
		// For backwards-compatibility, if EnableProtobufNegotiation is set to true, the ScrapeProtocols are set to
		// [PrometheusProto, OpenMetricsText1.0.0, OpenMetricsText0.0.1, PrometheusText0.0.4].
		arg.ScrapeProtocols = slices.Clone(defaultNativeHistogramScrapeProtocols)
		// In previous Prometheus versions, EnableProtobufNegotiation would also enable native histogram scraping.
		// This is no longer the case, so we need to explicitly enable it here.
		arg.ScrapeNativeHistograms = true
	}

	if arg.ScrapeNativeHistograms {
		// When scrape_native_histograms is set to true, the default scrape protocols are overridden to
		// Proto-first scrape protocols, like in upstream Prometheus.
		if reflect.DeepEqual(arg.ScrapeProtocols, defaultScrapeProtocols) {
			arg.ScrapeProtocols = slices.Clone(defaultNativeHistogramScrapeProtocols)
		}

		if !slices.Contains(arg.ScrapeProtocols, string(config.PrometheusProto)) {
			return fmt.Errorf("scrape_native_histograms is set to true, but PrometheusProto is not in scrape_protocols")
		}
	}

	// Validate scrape protocols
	existing := make(map[string]struct{})
	for _, p := range arg.ScrapeProtocols {
		if _, ok := existing[p]; ok {
			return fmt.Errorf("duplicate scrape protocol %q: make sure the scrape protocols provided are unique", p)
		}
		promSP := config.ScrapeProtocol(p)
		if err := promSP.Validate(); err != nil {
			return fmt.Errorf("invalid scrape protocol %q: %w", p, err)
		}
		existing[p] = struct{}{}
	}

	if arg.ScrapeFallbackProtocol != "" {
		promSP := config.ScrapeProtocol(arg.ScrapeFallbackProtocol)
		if err := promSP.Validate(); err != nil {
			return fmt.Errorf("invalid scrape_fallback_protocol %q: %w", arg.ScrapeFallbackProtocol, err)
		}
	}

	switch arg.MetricNameValidationScheme {
	case model.UTF8Validation.String(), model.LegacyValidation.String():
	default:
		return fmt.Errorf("invalid metric_name_validation_scheme %q: must be either %q or %q", arg.MetricNameValidationScheme, model.UTF8Validation.String(), model.LegacyValidation.String())
	}

	switch arg.MetricNameEscapingScheme {
	case "":
		if arg.MetricNameValidationScheme == model.LegacyValidation.String() {
			arg.MetricNameEscapingScheme = model.EscapeUnderscores
		} else {
			arg.MetricNameEscapingScheme = model.AllowUTF8
		}
	case model.AllowUTF8, model.EscapeUnderscores, model.EscapeDots, model.EscapeValues:
	default:
		supportedValues := []string{model.AllowUTF8, model.EscapeUnderscores, model.EscapeDots, model.EscapeValues}
		return fmt.Errorf("invalid metric_name_escaping_scheme: %q, supported values: %v", arg.MetricNameEscapingScheme, supportedValues)
	}

	if arg.MetricNameEscapingScheme == model.AllowUTF8 && arg.MetricNameValidationScheme != model.UTF8Validation.String() {
		return fmt.Errorf("metric_name_escaping_scheme cannot be set to 'allow-utf-8' while metric_name_validation_scheme is not set to 'utf8'")
	}

	// We must explicitly Validate because HTTPClientConfig is squashed and it won't run otherwise
	return arg.HTTPClientConfig.Validate()
}

func convertScrapeProtocols(promProtocols []config.ScrapeProtocol) []string {
	protocols := make([]string, 0, len(promProtocols))
	for _, p := range promProtocols {
		protocols = append(protocols, string(p))
	}
	return protocols
}

// Component implements the prometheus.scrape component.
type Component struct {
	opts    component.Options
	cluster cluster.Cluster

	reloadTargets       chan struct{}
	targetsGauge        client_prometheus.Gauge
	movedTargetsCounter client_prometheus.Counter
	unregisterer        util.Unregisterer

	mut             sync.RWMutex
	args            Arguments
	scraper         *scrape.Manager
	appendable      *prometheus.Fanout
	firstUpdateDone bool

	dtMutex            sync.Mutex
	distributedTargets *discovery.DistributedTargets

	debugDataPublisher livedebugging.DebugDataPublisher
}

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// New creates a new prometheus.scrape component.
func New(o component.Options, args Arguments) (*Component, error) {
	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

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

	if args.HonorMetadata && !o.MinStability.Permits(featuregate.StabilityExperimental) {
		return nil, fmt.Errorf("honor_metadata is an experimental feature, and must be enabled by setting the stability.level flag to experimental")
	}

	alloyAppendable := prometheus.NewFanout(args.ForwardTo, o.Registerer, ls)
	scrapeOptions := &scrape.Options{
		// NOTE: This is not Update()-able.
		ExtraMetrics: args.ExtraMetrics,
		HTTPClientOptions: []config_util.HTTPClientOption{
			config_util.WithDialContextFunc(httpData.DialFunc),
		},
		// NOTE: This is not Update()-able.
		EnableNativeHistogramsIngestion: args.ScrapeNativeHistograms,
		AppendMetadata:                  args.HonorMetadata,
		// otelcol.receiver.prometheus gets metadata from context
		PassMetadataInContext: args.HonorMetadata,
	}

	unregisterer := util.WrapWithUnregisterer(o.Registerer)

	targetsGauge := client_prometheus.NewGauge(client_prometheus.GaugeOpts{
		Name: "prometheus_scrape_targets_gauge",
		Help: "Number of targets this component is configured to scrape"})
	err = o.Registerer.Register(targetsGauge)
	if err != nil {
		return nil, err
	}

	movedTargetsCounter := client_prometheus.NewCounter(client_prometheus.CounterOpts{
		Name: "prometheus_scrape_targets_moved_total",
		Help: "Number of targets that have moved from this cluster node to another one"})
	err = o.Registerer.Register(movedTargetsCounter)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:                o,
		cluster:             clusterData,
		reloadTargets:       make(chan struct{}, 1),
		debugDataPublisher:  debugDataPublisher.(livedebugging.DebugDataPublisher),
		appendable:          alloyAppendable,
		targetsGauge:        targetsGauge,
		movedTargetsCounter: movedTargetsCounter,
		unregisterer:        unregisterer,
	}

	interceptor := NewInterceptor(livedebugging.ComponentID(o.ID), ls, c.debugDataPublisher, alloyAppendable)

	scraper, err := scrape.NewManager(
		scrapeOptions,
		slog.New(logging.NewSlogGoKitHandler(c.opts.Logger)),
		func(s string) (*promlogging.JSONFileLogger, error) { return promlogging.NewJSONFileLogger(s) },
		interceptor,
		unregisterer)
	if err != nil {
		return nil, fmt.Errorf("failed to create scrape manager: %w", err)
	}
	c.scraper = scraper

	// Call to Update() to set the receivers and targets once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	if args.EnableProtobufNegotiation {
		level.Warn(o.Logger).Log("msg", "enable_protobuf_negotiation is deprecated and will be removed in a future major release, use scrape_protocols instead")
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer c.scraper.Stop()
	defer c.unregisterer.UnregisterAll()

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
				args    = c.args
			)
			c.mut.RUnlock()

			if args.JobName != "" {
				jobName = c.args.JobName
			}

			newTargetGroups, movedTargets := c.distributeTargets(targets, jobName, args)

			// Make sure the targets that moved to another instance are NOT marked as stale. This is specific to how
			// Prometheus handles marking series as stale: it is the client's responsibility to inject the
			// staleness markers. In our case, for targets that moved to another instance in the cluster, we hand
			// over this responsibility to the new owning instance. We must not inject staleness marker here.
			c.scraper.DisableEndOfRunStalenessMarkers(jobName, movedTargets)

			select {
			case targetSetsChan <- newTargetGroups:
				level.Debug(c.opts.Logger).Log("msg", "passed new targets to scrape manager")
			case <-ctx.Done():
			}
		}
	}
}

func (c *Component) distributeTargets(
	targets []discovery.Target,
	jobName string,
	args Arguments,
) (map[string][]*targetgroup.Group, []*scrape.Target) {

	var (
		newDistTargets        = discovery.NewDistributedTargets(args.Clustering.Enabled, c.cluster, targets)
		oldDistributedTargets *discovery.DistributedTargets
	)

	c.dtMutex.Lock()
	oldDistributedTargets, c.distributedTargets = c.distributedTargets, newDistTargets
	c.dtMutex.Unlock()

	newLocalTargets := newDistTargets.LocalTargets()
	c.targetsGauge.Set(float64(len(newLocalTargets)))
	promNewTargets := discovery.ComponentTargetsToPromTargetGroups(jobName, newLocalTargets)

	movedTargets := newDistTargets.MovedToRemoteInstance(oldDistributedTargets)
	c.movedTargetsCounter.Add(float64(len(movedTargets)))
	// For moved targets, we need to populate prom labels in the same way as the scraper does, so that they match
	// the currently running scrape loop's targets. This is not needed for new targets, as they will be populated
	// by the scrape loop itself during the sync.
	promMovedTargets := c.populatePromLabels(movedTargets, jobName, args)

	return promNewTargets, promMovedTargets
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()

	// Some fields are not updateable at runtime - only allow them when Update()
	// is called for the first time from New().
	if !c.firstUpdateDone {
		c.firstUpdateDone = true
	} else {
		if c.args.ScrapeNativeHistograms != newArgs.ScrapeNativeHistograms {
			return fmt.Errorf("scrape_native_histograms cannot be updated at runtime")
		}
		if c.args.ExtraMetrics != newArgs.ExtraMetrics {
			return fmt.Errorf("extra_metrics cannot be updated at runtime")
		}
	}

	c.args = newArgs

	c.appendable.UpdateChildren(newArgs.ForwardTo)

	promConfig, err := config.Load("", slog.New(logging.NewSlogGoKitHandler(c.opts.Logger)))
	if err != nil {
		return fmt.Errorf("error loading blank prometheus config: %w", err)
	}
	sc := getPromScrapeConfigs(c.opts.ID, newArgs)
	promConfig.ScrapeConfigs = []*config.ScrapeConfig{sc}
	err = c.scraper.ApplyConfig(promConfig)
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
	copyScrapeClassicHistograms := c.ScrapeClassicHistograms // make a copy as Prometheus wants a pointer.
	dec.HonorLabels = c.HonorLabels
	dec.HonorTimestamps = c.HonorTimestamps
	dec.TrackTimestampsStaleness = c.TrackTimestampsStaleness
	dec.Params = c.Params
	dec.AlwaysScrapeClassicHistograms = &copyScrapeClassicHistograms
	dec.ScrapeInterval = model.Duration(c.ScrapeInterval)
	dec.ScrapeTimeout = model.Duration(c.ScrapeTimeout)
	dec.ScrapeFailureLogFile = c.ScrapeFailureLogFile
	dec.MetricsPath = c.MetricsPath
	dec.Scheme = c.Scheme
	dec.BodySizeLimit = c.BodySizeLimit
	dec.SampleLimit = c.SampleLimit
	dec.TargetLimit = c.TargetLimit
	dec.LabelLimit = c.LabelLimit
	dec.LabelNameLengthLimit = c.LabelNameLengthLimit
	dec.LabelValueLengthLimit = c.LabelValueLengthLimit

	// Scrape protocols
	scrapeProtocols := make([]config.ScrapeProtocol, 0, len(c.ScrapeProtocols))
	for _, p := range c.ScrapeProtocols {
		scrapeProtocols = append(scrapeProtocols, config.ScrapeProtocol(p))
	}
	dec.ScrapeProtocols = scrapeProtocols

	// HTTP scrape client settings
	dec.HTTPClientConfig = *c.HTTPClientConfig.Convert()

	validationScheme := model.UnsetValidation
	if model.LegacyValidation.String() == c.MetricNameValidationScheme {
		validationScheme = model.LegacyValidation
	} else if model.UTF8Validation.String() == c.MetricNameValidationScheme {
		validationScheme = model.UTF8Validation
	}
	dec.MetricNameValidationScheme = validationScheme
	dec.MetricNameEscapingScheme = c.MetricNameEscapingScheme
	dec.ScrapeFallbackProtocol = config.ScrapeProtocol(c.ScrapeFallbackProtocol)
	convertToNHCB := c.ConvertClassicHistogramsToNHCB
	dec.ConvertClassicHistogramsToNHCB = &convertToNHCB
	dec.EnableCompression = c.EnableCompression
	dec.NativeHistogramBucketLimit = c.NativeHistogramBucketLimit
	dec.NativeHistogramMinBucketFactor = c.NativeHistogramMinBucketFactor

	return &dec
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
				lb := labels.NewBuilder(labels.EmptyLabels())
				res = append(res, TargetStatus{
					JobName:            job,
					URL:                st.URL().String(),
					Health:             string(st.Health()),
					Labels:             st.Labels(lb).Map(),
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

func (c *Component) populatePromLabels(targets []discovery.Target, jobName string, args Arguments) []*scrape.Target {
	// We need to call scrape.TargetsFromGroup to reuse the rather complex logic of populating labels on targets.
	allTargets := make([]*scrape.Target, 0, len(targets))
	groups := discovery.ComponentTargetsToPromTargetGroups(jobName, targets)
	for _, tgs := range groups {
		for _, tg := range tgs {
			promTargets, errs := scrape.TargetsFromGroup(
				tg,
				getPromScrapeConfigs(jobName, args),
				make([]*scrape.Target, len(targets)), /* targets slice to reuse */
				labels.NewBuilder(labels.EmptyLabels()),
			)
			for _, err := range errs {
				level.Warn(c.opts.Logger).Log("msg", "error while populating labels of targets using prom config", "err", err)
			}
			allTargets = append(allTargets, promTargets...)
		}
	}

	return allTargets
}

func (c *Component) LiveDebugging() {}
