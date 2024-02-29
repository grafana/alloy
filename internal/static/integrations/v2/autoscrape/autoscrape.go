// Package autoscrape implements a scraper for integrations.
package autoscrape

import (
	"context"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/agent/internal/static/metrics"
	"github.com/grafana/agent/internal/static/metrics/instance"
	"github.com/grafana/agent/internal/static/server"
	"github.com/oklog/run"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_config "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
)

// DefaultGlobal holds default values for Global.
var DefaultGlobal = Global{
	Enable:          true,
	MetricsInstance: "default",
}

// Global holds default settings for metrics integrations that support
// autoscraping. Integrations may override their settings.
type Global struct {
	Enable          bool           `yaml:"enable,omitempty"`           // Whether self-scraping should be enabled.
	MetricsInstance string         `yaml:"metrics_instance,omitempty"` // Metrics instance name to send metrics to.
	ScrapeInterval  model.Duration `yaml:"scrape_interval,omitempty"`  // Self-scraping frequency.
	ScrapeTimeout   model.Duration `yaml:"scrape_timeout,omitempty"`   // Self-scraping timeout.
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (g *Global) UnmarshalYAML(f func(interface{}) error) error {
	*g = DefaultGlobal
	type global Global
	return f((*global)(g))
}

// Config configure autoscrape for an individual integration. Override defaults.
type Config struct {
	Enable          *bool          `yaml:"enable,omitempty"`           // Whether self-scraping should be enabled.
	MetricsInstance string         `yaml:"metrics_instance,omitempty"` // Metrics instance name to send metrics to.
	ScrapeInterval  model.Duration `yaml:"scrape_interval,omitempty"`  // Self-scraping frequency.
	ScrapeTimeout   model.Duration `yaml:"scrape_timeout,omitempty"`   // Self-scraping timeout.

	RelabelConfigs       []*relabel.Config `yaml:"relabel_configs,omitempty"`        // Relabel the autoscrape job
	MetricRelabelConfigs []*relabel.Config `yaml:"metric_relabel_configs,omitempty"` // Relabel individual autoscrape metrics
}

// InstanceStore is used to find instances to send metrics to. It is a subset
// of the pkg/metrics/instance.Manager interface.
type InstanceStore interface {
	// GetInstance retrieves a ManagedInstance by name.
	GetInstance(name string) (instance.ManagedInstance, error)
}

// ScrapeConfig bind a Prometheus scrape config with an instance to send
// scraped metrics to.
type ScrapeConfig struct {
	Instance string
	Config   prom_config.ScrapeConfig
}

// Scraper is a metrics autoscraper.
type Scraper struct {
	ctx    context.Context
	cancel context.CancelFunc

	log log.Logger
	is  InstanceStore

	// Prometheus doesn't pass contextual information at scrape time that could
	// be used to change the behavior of generating an appender. This means that
	// it's not yet possible for us to just run a single SD + scrape manager for
	// all of our integrations, and we instead need to launch a pair of each for
	// every instance we're writing to.

	iscrapersMut sync.RWMutex
	iscrapers    map[string]*instanceScraper
	dialerFunc   server.DialContextFunc
}

// NewScraper creates a new autoscraper. Scraper will run until Stop is called.
// Instances to send scraped metrics to will be looked up via im. Scraping will
// use the provided dialerFunc to make connections if non-nil.
func NewScraper(l log.Logger, is InstanceStore, dialerFunc server.DialContextFunc) *Scraper {
	l = log.With(l, "component", "autoscraper")

	ctx, cancel := context.WithCancel(context.Background())

	s := &Scraper{
		ctx:    ctx,
		cancel: cancel,

		log:        l,
		is:         is,
		iscrapers:  map[string]*instanceScraper{},
		dialerFunc: dialerFunc,
	}
	return s
}

// ApplyConfig will apply the given jobs. An error will be returned for any
// jobs that failed to be applied.
func (s *Scraper) ApplyConfig(jobs []*ScrapeConfig) error {
	s.iscrapersMut.Lock()
	defer s.iscrapersMut.Unlock()

	var firstError error
	saveError := func(e error) {
		if firstError == nil {
			firstError = e
		}
	}

	// Shard our jobs by target instance.
	shardedJobs := map[string][]*prom_config.ScrapeConfig{}
	for _, j := range jobs {
		_, err := s.is.GetInstance(j.Instance)
		if err != nil {
			level.Error(s.log).Log("msg", "cannot autoscrape integration", "name", j.Config.JobName, "err", err)
			saveError(err)
			continue
		}

		shardedJobs[j.Instance] = append(shardedJobs[j.Instance], &j.Config)
	}

	// Then pass the jobs to instanceScraper, creating them if we need to.
	for instance, jobs := range shardedJobs {
		is, ok := s.iscrapers[instance]
		if !ok {
			is = newInstanceScraper(s.ctx, s.log, s.is, instance, config_util.DialContextFunc(s.dialerFunc))
			s.iscrapers[instance] = is
		}
		if err := is.ApplyConfig(jobs); err != nil {
			// Not logging here; is.ApplyConfig already logged the errors.
			saveError(err)
		}
	}

	// Garbage collect: If there's a key in s.scrapers that wasn't in
	// shardedJobs, stop that unused scraper.
	for instance, is := range s.iscrapers {
		_, current := shardedJobs[instance]
		if !current {
			is.Stop()
			delete(s.iscrapers, instance)
		}
	}

	return firstError
}

// TargetsActive returns the set of active scrape targets for all target
// instances.
func (s *Scraper) TargetsActive() map[string]metrics.TargetSet {
	s.iscrapersMut.RLock()
	defer s.iscrapersMut.RUnlock()

	allTargets := make(map[string]metrics.TargetSet, len(s.iscrapers))
	for instance, is := range s.iscrapers {
		allTargets[instance] = is.sm.TargetsActive()
	}
	return allTargets
}

// Stop stops the Scraper.
func (s *Scraper) Stop() {
	s.iscrapersMut.Lock()
	defer s.iscrapersMut.Unlock()

	for instance, is := range s.iscrapers {
		is.Stop()
		delete(s.iscrapers, instance)
	}

	s.cancel()
}

// instanceScraper is a Scraper which always sends to the same instance.
type instanceScraper struct {
	log log.Logger

	sd     *discovery.Manager
	sm     *scrape.Manager
	cancel context.CancelFunc
	exited chan struct{}
}

// newInstanceScraper runs a new instanceScraper. Must be stopped by calling
// Stop.
func newInstanceScraper(
	ctx context.Context,
	l log.Logger,
	s InstanceStore,
	instanceName string,
	dialerFunc config_util.DialContextFunc,
) *instanceScraper {

	ctx, cancel := context.WithCancel(ctx)
	l = log.With(l, "target_instance", instanceName)

	sdOpts := []func(*discovery.Manager){
		discovery.Name("autoscraper/" + instanceName),
		discovery.HTTPClientOptions(
			// If dialerFunc is nil, scrape.NewManager will use Go's default dialer.
			config_util.WithDialContextFunc(dialerFunc),
		),
	}
	sd := discovery.NewManager(ctx, l, sdOpts...)
	sm := scrape.NewManager(&scrape.Options{
		HTTPClientOptions: []config_util.HTTPClientOption{
			// If dialerFunc is nil, scrape.NewManager will use Go's default dialer.
			config_util.WithDialContextFunc(dialerFunc),
		},
	}, l, &agentAppender{
		inst: instanceName,
		is:   s,
	})

	is := &instanceScraper{
		log: l,

		sd:     sd,
		sm:     sm,
		cancel: cancel,
		exited: make(chan struct{}),
	}

	go is.run()
	return is
}

type agentAppender struct {
	inst string
	is   InstanceStore
}

func (aa *agentAppender) Appender(ctx context.Context) storage.Appender {
	mi, err := aa.is.GetInstance(aa.inst)
	if err != nil {
		return &failedAppender{instanceName: aa.inst}
	}
	return mi.Appender(ctx)
}

func (is *instanceScraper) run() {
	defer close(is.exited)
	var rg run.Group

	rg.Add(func() error {
		// Service discovery will stop whenever our parent context is canceled or
		// if is.cancel is called.
		err := is.sd.Run()
		if err != nil {
			level.Error(is.log).Log("msg", "autoscrape service discovery exited with error", "err", err)
		}
		return err
	}, func(_ error) {
		is.cancel()
	})

	rg.Add(func() error {
		err := is.sm.Run(is.sd.SyncCh())
		if err != nil {
			level.Error(is.log).Log("msg", "autoscrape scrape manager exited with error", "err", err)
		}
		return err
	}, func(_ error) {
		is.sm.Stop()
	})

	_ = rg.Run()
}

func (is *instanceScraper) ApplyConfig(jobs []*prom_config.ScrapeConfig) error {
	var firstError error
	saveError := func(e error) {
		if firstError == nil && e != nil {
			firstError = e
		}
	}

	var (
		scrapeConfigs = make([]*prom_config.ScrapeConfig, 0, len(jobs))
		sdConfigs     = make(map[string]discovery.Configs, len(jobs))
	)
	for _, job := range jobs {
		sdConfigs[job.JobName] = job.ServiceDiscoveryConfigs
		scrapeConfigs = append(scrapeConfigs, job)
	}
	if err := is.sd.ApplyConfig(sdConfigs); err != nil {
		level.Error(is.log).Log("msg", "error when applying SD to autoscraper", "err", err)
		saveError(err)
	}
	if err := is.sm.ApplyConfig(&prom_config.Config{ScrapeConfigs: scrapeConfigs}); err != nil {
		level.Error(is.log).Log("msg", "error when applying jobs to scraper", "err", err)
		saveError(err)
	}

	return firstError
}

func (is *instanceScraper) Stop() {
	is.cancel()
	<-is.exited
}
