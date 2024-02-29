package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/agent/internal/static/integrations/v2/autoscrape"
	"github.com/grafana/agent/internal/static/metrics"
	"github.com/prometheus/common/model"
	http_sd "github.com/prometheus/prometheus/discovery/http"
)

const (
	// IntegrationsSDEndpoint is the API endpoint where the integration HTTP SD
	// API is exposed. The API uses query parameters to customize what gets
	// returned by discovery.
	IntegrationsSDEndpoint = "/agent/api/v1/metrics/integrations/sd"

	// IntegrationsAutoscrapeTargetsEndpoint is the API endpoint where autoscrape
	// integrations targets are exposed.
	IntegrationsAutoscrapeTargetsEndpoint = "/agent/api/v1/metrics/integrations/targets"
)

// DefaultSubsystemOptions holds the default settings for a Controller.
var (
	DefaultSubsystemOptions = SubsystemOptions{
		Metrics: DefaultMetricsSubsystemOptions,
	}

	DefaultMetricsSubsystemOptions = MetricsSubsystemOptions{
		Autoscrape: autoscrape.DefaultGlobal,
	}
)

// SubsystemOptions controls how the integrations subsystem behaves.
type SubsystemOptions struct {
	Metrics MetricsSubsystemOptions `yaml:"metrics,omitempty"`

	// Configs are configurations of integration to create. Unmarshaled through
	// the custom UnmarshalYAML method of Controller.
	Configs Configs `yaml:"-"`
}

// MetricsSubsystemOptions controls how metrics integrations behave.
type MetricsSubsystemOptions struct {
	Autoscrape autoscrape.Global `yaml:"autoscrape,omitempty"`
}

// ApplyDefaults will apply defaults to o.
func (o *SubsystemOptions) ApplyDefaults(mcfg *metrics.Config) error {
	if o.Metrics.Autoscrape.ScrapeInterval == 0 {
		o.Metrics.Autoscrape.ScrapeInterval = mcfg.Global.Prometheus.ScrapeInterval
	}
	if o.Metrics.Autoscrape.ScrapeTimeout == 0 {
		o.Metrics.Autoscrape.ScrapeTimeout = mcfg.Global.Prometheus.ScrapeTimeout
	}

	return nil
}

// MarshalYAML implements yaml.Marshaler for SubsystemOptions. Integrations
// will be marshaled inline.
func (o SubsystemOptions) MarshalYAML() (interface{}, error) {
	return MarshalYAML(o)
}

// UnmarshalYAML implements yaml.Unmarshaler for SubsystemOptions. Inline
// integrations will be unmarshaled into o.Configs.
func (o *SubsystemOptions) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*o = DefaultSubsystemOptions
	return UnmarshalYAML(o, unmarshal)
}

// Subsystem runs the integrations subsystem, managing a set of integrations.
type Subsystem struct {
	logger log.Logger

	mut         sync.RWMutex
	globals     Globals
	apiHandler  http.Handler // generated from controller
	autoscraper *autoscrape.Scraper

	ctrl             *controller
	stopController   context.CancelFunc
	controllerExited chan struct{}
}

// NewSubsystem creates and starts a new integrations Subsystem. Every field in
// IntegrationOptions must be filled out.
func NewSubsystem(l log.Logger, globals Globals) (*Subsystem, error) {
	autoscraper := autoscrape.NewScraper(l, globals.Metrics.InstanceManager(), globals.DialContextFunc)

	l = log.With(l, "component", "integrations")

	ctrl, err := newController(l, controllerConfig(globals.SubsystemOpts.Configs), globals)
	if err != nil {
		autoscraper.Stop()
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	ctrlExited := make(chan struct{})
	go func() {
		ctrl.run(ctx)
		close(ctrlExited)
	}()

	s := &Subsystem{
		logger: l,

		globals:     globals,
		autoscraper: autoscraper,

		ctrl:             ctrl,
		stopController:   cancel,
		controllerExited: ctrlExited,
	}
	if err := s.ApplyConfig(globals); err != nil {
		cancel()
		autoscraper.Stop()
		return nil, err
	}
	return s, nil
}

// ApplyConfig updates the configuration of the integrations subsystem.
func (s *Subsystem) ApplyConfig(globals Globals) error {
	const prefix = "/integrations/"

	s.mut.Lock()
	defer s.mut.Unlock()

	if err := s.ctrl.UpdateController(controllerConfig(globals.SubsystemOpts.Configs), globals); err != nil {
		return fmt.Errorf("error applying integrations: %w", err)
	}

	var firstErr error
	saveFirstErr := func(err error) {
		if firstErr == nil {
			firstErr = err
		}
	}

	// Set up HTTP wiring
	{
		handler, err := s.ctrl.Handler(prefix)
		if err != nil {
			saveFirstErr(fmt.Errorf("HTTP handler update failed: %w", err))
		}
		s.apiHandler = handler
	}

	// Set up self-scraping
	{
		httpSDConfig := http_sd.DefaultSDConfig
		httpSDConfig.RefreshInterval = model.Duration(time.Second * 5) // TODO(rfratto): make configurable?

		apiURL := globals.CloneAgentBaseURL()
		apiURL.Path = IntegrationsSDEndpoint
		httpSDConfig.URL = apiURL.String()

		scrapeConfigs := s.ctrl.ScrapeConfigs(prefix, &httpSDConfig)
		if err := s.autoscraper.ApplyConfig(scrapeConfigs); err != nil {
			saveFirstErr(fmt.Errorf("configuring autoscraper failed: %w", err))
		}
	}

	s.globals = globals
	return firstErr
}

// WireAPI hooks up integration endpoints to r.
func (s *Subsystem) WireAPI(r *mux.Router) {
	const prefix = "/integrations"
	r.PathPrefix(prefix).HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		s.mut.RLock()
		handler := s.apiHandler
		s.mut.RUnlock()

		if handler == nil {
			rw.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(rw, "Integrations HTTP endpoints not yet available")
			return
		}
		handler.ServeHTTP(rw, r)
	})

	r.HandleFunc(IntegrationsSDEndpoint, func(rw http.ResponseWriter, r *http.Request) {
		targetOptions, err := TargetOptionsFromParams(r.URL.Query())
		if err != nil {
			http.Error(rw, fmt.Sprintf("invalid query parameters: %s", err), http.StatusBadRequest)
			return
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)

		tgs := s.ctrl.Targets(Endpoint{
			Host:   r.Host,
			Prefix: prefix,
		}, targetOptions)

		// Normalize targets. We may have targets in the group with non-address
		// labels. These need to be retained, so we'll just split everything up
		// into multiple groups.
		//
		// TODO(rfratto): optimize to remove redundant groups
		finalTgs := []*targetGroup{}
		for _, group := range tgs {
			for _, target := range group.Targets {
				// Create the final labels for the group. This will be everything from
				// the group and the target (except for model.AddressLabel). Labels
				// from target take precedence labels from over group.
				groupLabels := group.Labels.Merge(target)
				delete(groupLabels, model.AddressLabel)

				finalTgs = append(finalTgs, &targetGroup{
					Targets: []model.LabelSet{{model.AddressLabel: target[model.AddressLabel]}},
					Labels:  groupLabels,
				})
			}
		}

		enc := json.NewEncoder(rw)
		_ = enc.Encode(finalTgs)
	})

	r.HandleFunc(IntegrationsAutoscrapeTargetsEndpoint, func(rw http.ResponseWriter, r *http.Request) {
		allTargets := s.autoscraper.TargetsActive()
		metrics.ListTargetsHandler(allTargets).ServeHTTP(rw, r)
	})
}

// Stop stops the manager and all running integrations. Blocks until all
// running integrations exit.
func (s *Subsystem) Stop() {
	s.autoscraper.Stop()
	s.stopController()
	<-s.controllerExited
}
