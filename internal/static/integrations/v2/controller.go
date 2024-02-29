package integrations

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/gorilla/mux"
	"github.com/grafana/agent/internal/static/integrations/v2/autoscrape"
	"github.com/prometheus/prometheus/discovery"
	http_sd "github.com/prometheus/prometheus/discovery/http"
	"go.uber.org/atomic"
)

// controllerConfig holds a set of integration configs.
type controllerConfig []Config

// controller manages a set of integrations.
type controller struct {
	logger log.Logger

	mut          sync.Mutex
	cfg          controllerConfig
	globals      Globals
	integrations []*controlledIntegration // Running integrations

	runIntegrations chan []*controlledIntegration // Schedule integrations to run
}

// newController creates a new Controller. Controller is intended to be
// embedded inside of integrations that may want to multiplex other
// integrations.
func newController(l log.Logger, cfg controllerConfig, globals Globals) (*controller, error) {
	c := &controller{
		logger:          l,
		runIntegrations: make(chan []*controlledIntegration, 1),
	}
	if err := c.UpdateController(cfg, globals); err != nil {
		return nil, err
	}
	return c, nil
}

// run starts the controller and blocks until ctx is canceled.
func (c *controller) run(ctx context.Context) {
	pool := newWorkerPool(ctx, c.logger)
	defer pool.Close()

	for {
		select {
		case <-ctx.Done():
			level.Debug(c.logger).Log("msg", "controller exiting")
			return
		case newIntegrations := <-c.runIntegrations:
			pool.Reload(newIntegrations)
		}
	}
}

// controlledIntegration is a running Integration. A running integration is
// identified uniquely by its id.
type controlledIntegration struct {
	id      integrationID
	i       Integration
	c       Config // Config that generated i. Used for changing to see if a config changed.
	running atomic.Bool
}

func (ci *controlledIntegration) Running() bool {
	return ci.running.Load()
}

// integrationID uses a tuple of Name and Identifier to uniquely identify an
// integration.
type integrationID struct{ Name, Identifier string }

func (id integrationID) String() string {
	return fmt.Sprintf("%s/%s", id.Name, id.Identifier)
}

// UpdateController updates the Controller with new Controller and
// IntegrationOptions.
//
// UpdateController updates running integrations. Extensions can be
// recalculated by calling relevant methods like Handler or Targets.
func (c *controller) UpdateController(cfg controllerConfig, globals Globals) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	// Ensure that no singleton integration is defined twice
	var (
		duplicatedSingletons []string
		singletonSet         = make(map[string]struct{})
	)
	for _, cfg := range cfg {
		t, _ := RegisteredType(cfg)
		if t != TypeSingleton {
			continue
		}

		if _, exists := singletonSet[cfg.Name()]; exists {
			duplicatedSingletons = append(duplicatedSingletons, cfg.Name())
			continue
		}
		singletonSet[cfg.Name()] = struct{}{}
	}
	if len(duplicatedSingletons) == 1 {
		return fmt.Errorf("integration %q may only be defined once", duplicatedSingletons[0])
	} else if len(duplicatedSingletons) > 1 {
		list := strings.Join(duplicatedSingletons, ", ")
		return fmt.Errorf("the following integrations may only be defined once each: %s", list)
	}

	integrationIDMap := map[integrationID]struct{}{}

	integrations := make([]*controlledIntegration, 0, len(cfg))

NextConfig:
	for _, ic := range cfg {
		name := ic.Name()

		identifier, err := ic.Identifier(globals)
		if err != nil {
			return fmt.Errorf("could not build identifier for integration %q: %w", name, err)
		}

		if err := ic.ApplyDefaults(globals); err != nil {
			return fmt.Errorf("failed to apply defaults for %s/%s: %w", name, identifier, err)
		}

		id := integrationID{Name: name, Identifier: identifier}
		if _, exist := integrationIDMap[id]; exist {
			return fmt.Errorf("multiple instance names %q in integration %q", identifier, name)
		}
		integrationIDMap[id] = struct{}{}

		// Now that we know the ID for an integration, we can check to see if it's
		// running and can be dynamically updated.
		for _, ci := range c.integrations {
			if ci.id != id {
				continue
			}

			// If the configs haven't changed, then we don't need to do anything.
			if CompareConfigs(ci.c, ic) {
				integrations = append(integrations, ci)
				continue NextConfig
			}

			if ui, ok := ci.i.(UpdateIntegration); ok {
				if err := ui.ApplyConfig(ic, globals); errors.Is(err, ErrInvalidUpdate) {
					level.Warn(c.logger).Log("msg", "failed to dynamically update integration; will recreate", "integration", name, "instance", identifier, "err', err")
					break
				} else if err != nil {
					return fmt.Errorf("failed to update %s integration %q: %w", name, identifier, err)
				} else {
					// Update succeeded; re-use the running one and go to the next
					// integration to process.
					integrations = append(integrations, ci)
					continue NextConfig
				}
			}

			// We found the integration to update: we can stop this loop now.
			break
		}

		logger := log.With(c.logger, "integration", name, "instance", identifier)
		integration, err := ic.NewIntegration(logger, globals)
		if err != nil {
			return fmt.Errorf("failed to construct %s integration %q: %w", name, identifier, err)
		}

		// Create a new controlled integration.
		integrations = append(integrations, &controlledIntegration{
			id: id,
			i:  integration,
			c:  ic,
		})
	}

	// Schedule integrations to run
	c.runIntegrations <- integrations

	c.cfg = cfg
	c.globals = globals
	c.integrations = integrations
	return nil
}

// Handler returns an HTTP handler for the controller and its integrations.
// Handler will pass through requests to other running integrations. Handler
// always returns an http.Handler regardless of error.
//
// Handler is expensive to compute and should only be done after reloading the
// config.
func (c *controller) Handler(prefix string) (http.Handler, error) {
	var firstErr error
	saveFirstErr := func(err error) {
		if firstErr == nil {
			firstErr = err
		}
	}

	r := mux.NewRouter()

	err := c.forEachIntegration(prefix, func(ci *controlledIntegration, iprefix string) {
		id := ci.id

		i, ok := ci.i.(HTTPIntegration)
		if !ok {
			return
		}

		handler, err := i.Handler(iprefix + "/")
		if err != nil {
			saveFirstErr(fmt.Errorf("could not generate HTTP handler for %s integration %q: %w", id.Name, id.Identifier, err))
			return
		} else if handler == nil {
			return
		}

		// Anything that matches the integrationPrefix should be passed to the handler.
		// The reason these two are separated is if you have two instance names and one is a prefix of another
		// ie localhost and localhost2, localhost2 will never get called because localhost will always get precedence
		// add / fixes this, but to keep old behavior we need to ensure /localhost and localhost2 also work, hence
		// the second handlefunc below this one. https://github.com/grafana/agent/issues/1718
		hfunc := func(rw http.ResponseWriter, r *http.Request) {
			if !ci.Running() {
				http.Error(rw, fmt.Sprintf("%s integration intance %q not running", id.Name, id.Identifier), http.StatusServiceUnavailable)
				return
			}
			handler.ServeHTTP(rw, r)
		}
		r.PathPrefix(iprefix + "/").HandlerFunc(hfunc)
		// Handle calling the iprefix itself
		r.HandleFunc(iprefix, hfunc)
	})
	if err != nil {
		level.Warn(c.logger).Log("msg", "error when iterating over integrations to build HTTP handlers", "err", err)
	}

	// TODO(rfratto): navigation page for exact prefix match

	return r, firstErr
}

// forEachIntegration calculates the prefix for each integration and calls f.
// prefix will not end in /.
func (c *controller) forEachIntegration(basePrefix string, f func(ci *controlledIntegration, iprefix string)) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	// Pre-populate a mapping of integration name -> identifier. If there are
	// two instances of the same integration, we want to ensure unique routing.
	//
	// This special logic is done for backwards compatibility with the original
	// design of integrations.
	identifiersMap := map[string][]string{}
	for _, i := range c.integrations {
		identifiersMap[i.id.Name] = append(identifiersMap[i.id.Name], i.id.Identifier)
	}

	usedPrefixes := map[string]struct{}{}

	for _, ci := range c.integrations {
		id := ci.id
		multipleInstances := len(identifiersMap[id.Name]) > 1

		var integrationPrefix string
		if multipleInstances {
			// i.e., /integrations/mysqld_exporter/server-a
			integrationPrefix = path.Join(basePrefix, id.Name, id.Identifier)
		} else {
			// i.e., /integrations/node_exporter
			integrationPrefix = path.Join(basePrefix, id.Name)
		}

		f(ci, integrationPrefix)

		if _, exist := usedPrefixes[integrationPrefix]; exist {
			return fmt.Errorf("BUG: duplicate integration prefix %q", integrationPrefix)
		}
		usedPrefixes[integrationPrefix] = struct{}{}
	}
	return nil
}

// Targets returns the current set of targets across all integrations. Use opts
// to customize which targets are returned.
func (c *controller) Targets(ep Endpoint, opts TargetOptions) []*targetGroup {
	// Grab the integrations as fast as possible. We don't want to spend too much
	// time holding the mutex.
	type prefixedMetricsIntegration struct {
		id integrationID
		i  MetricsIntegration
		ep Endpoint
	}
	var mm []prefixedMetricsIntegration

	err := c.forEachIntegration(ep.Prefix, func(ci *controlledIntegration, iprefix string) {
		// Best effort liveness check. They might stop running when we request
		// their targets, which is fine, but we should save as much work as we
		// can.
		if !ci.Running() {
			return
		}
		if mi, ok := ci.i.(MetricsIntegration); ok {
			ep := Endpoint{Host: ep.Host, Prefix: iprefix}
			mm = append(mm, prefixedMetricsIntegration{id: ci.id, i: mi, ep: ep})
		}
	})
	if err != nil {
		level.Warn(c.logger).Log("msg", "error when iterating over integrations to get targets", "err", err)
	}

	var tgs []*targetGroup
	for _, mi := range mm {
		// If we're looking for a subset of integrations, filter out anything that doesn't match.
		if len(opts.Integrations) > 0 && !stringSliceContains(opts.Integrations, mi.id.Name) {
			continue
		}
		// If we're looking for a specific instance, filter out anything that doesn't match.
		if opts.Instance != "" && mi.id.Identifier != opts.Instance {
			continue
		}

		for _, tgt := range mi.i.Targets(mi.ep) {
			tgs = append(tgs, (*targetGroup)(tgt))
		}
	}
	sort.Slice(tgs, func(i, j int) bool {
		return tgs[i].Source < tgs[j].Source
	})
	return tgs
}

func stringSliceContains(ss []string, s string) bool {
	for _, check := range ss {
		if check == s {
			return true
		}
	}
	return false
}

// TargetOptions controls which targets should be returned by the subsystem.
type TargetOptions struct {
	// Integrations is the set of integrations to return. An empty slice will
	// default to returning all integrations.
	Integrations []string
	// Instance matches a specific instance from all integrations. An empty
	// string will match any instance.
	Instance string
}

// TargetOptionsFromParams creates TargetOptions from parsed URL query parameters.
func TargetOptionsFromParams(u url.Values) (TargetOptions, error) {
	var to TargetOptions

	rawIntegrations := u.Get("integrations")
	if rawIntegrations != "" {
		rawIntegrations, err := url.QueryUnescape(rawIntegrations)
		if err != nil {
			return to, fmt.Errorf("invalid value for integrations: %w", err)
		}
		to.Integrations = strings.Split(rawIntegrations, ",")
	}

	rawInstance := u.Get("instance")
	if rawInstance != "" {
		rawInstance, err := url.QueryUnescape(rawInstance)
		if err != nil {
			return to, fmt.Errorf("invalid value for instance: %w", err)
		}
		to.Instance = rawInstance
	}

	return to, nil
}

// ToParams will convert to into URL query parameters.
func (to TargetOptions) ToParams() url.Values {
	p := make(url.Values)
	if len(to.Integrations) != 0 {
		p.Set("integrations", url.QueryEscape(strings.Join(to.Integrations, ",")))
	}
	if to.Instance != "" {
		p.Set("instance", url.QueryEscape(to.Instance))
	}
	return p
}

// ScrapeConfigs returns a set of scrape configs to use for self-scraping.
// sdConfig should contain the full URL where the integrations SD API is
// exposed. ScrapeConfigs will inject unique query parameters per integration
// to limit what will be discovered.
func (c *controller) ScrapeConfigs(prefix string, sdConfig *http_sd.SDConfig) []*autoscrape.ScrapeConfig {
	// Grab the integrations as fast as possible. We don't want to spend too much
	// time holding the mutex.
	type prefixedMetricsIntegration struct {
		id     integrationID
		i      MetricsIntegration
		prefix string
	}
	var mm []prefixedMetricsIntegration

	err := c.forEachIntegration(prefix, func(ci *controlledIntegration, iprefix string) {
		if mi, ok := ci.i.(MetricsIntegration); ok {
			mm = append(mm, prefixedMetricsIntegration{id: ci.id, i: mi, prefix: iprefix})
		}
	})
	if err != nil {
		level.Warn(c.logger).Log("msg", "error when iterating over integrations to get scrape configs", "err", err)
	}

	var cfgs []*autoscrape.ScrapeConfig
	for _, mi := range mm {
		// sdConfig will be pointing to the targets API. By default, this returns absolutely everything.
		// We want to use the query parameters to inform the API to only return
		// specific targets.
		opts := TargetOptions{
			Integrations: []string{mi.id.Name},
			Instance:     mi.id.Identifier,
		}

		integrationSDConfig := *sdConfig
		integrationSDConfig.URL = sdConfig.URL + "?" + opts.ToParams().Encode()
		sds := discovery.Configs{&integrationSDConfig}
		cfgs = append(cfgs, mi.i.ScrapeConfigs(sds)...)
	}
	sort.Slice(cfgs, func(i, j int) bool {
		return cfgs[i].Config.JobName < cfgs[j].Config.JobName
	})
	return cfgs
}
