package common

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/labelstore"
)

type Component struct {
	mut     sync.RWMutex
	config  *operator.Arguments
	manager crdManagerInterface
	ls      labelstore.LabelStore

	onUpdate  chan struct{}
	opts      component.Options
	healthMut sync.RWMutex
	health    component.Health

	crdManagerFactory crdManagerFactory

	kind    string
	cluster cluster.Cluster
}

func New(o component.Options, args component.Arguments, kind string) (*Component, error) {
	data, err := o.GetServiceData(cluster.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get information about cluster service: %w", err)
	}
	clusterData := data.(cluster.Cluster)

	service, err := o.GetServiceData(labelstore.ServiceName)
	if err != nil {
		return nil, err
	}
	ls := service.(labelstore.LabelStore)
	c := &Component{
		opts:              o,
		onUpdate:          make(chan struct{}, 1),
		kind:              kind,
		cluster:           clusterData,
		ls:                ls,
		crdManagerFactory: realCrdManagerFactory{},
	}
	return c, c.Update(args)
}

func (c *Component) CurrentHealth() component.Health {
	c.healthMut.RLock()
	defer c.healthMut.RUnlock()
	return c.health
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	// innerCtx gets passed to things we create, so we can restart everything anytime we get an update.
	// Ideally, this component has very little dynamic config, and won't have frequent updates.
	var innerCtx context.Context
	// cancel is the func we use to trigger a stop to all downstream processors we create
	var cancel func()
	defer func() {
		if cancel != nil {
			cancel()
		}
	}()

	c.reportHealth(nil)
	errChan := make(chan error, 1)
	wg := sync.WaitGroup{}
	defer wg.Wait()
	for {
		select {
		case <-ctx.Done():
			if cancel != nil {
				cancel()
			}
			return nil
		case err := <-errChan:
			c.reportHealth(err)
		case <-c.onUpdate:
			c.mut.Lock()
			manager := c.crdManagerFactory.New(c.opts, c.cluster, c.opts.Logger, c.config, c.kind, c.ls)
			c.manager = manager

			// Wait for the old manager to stop.
			// If we start the new manager before stopping the old one,
			// the new manager might not be able to register its debug metrics due to a duplicate registration error.
			if cancel != nil {
				cancel()
			}
			wg.Wait()

			innerCtx, cancel = context.WithCancel(ctx)
			wg.Add(1)
			go func() {
				if err := manager.Run(innerCtx); err != nil {
					level.Error(c.opts.Logger).Log("msg", "error running crd manager", "err", err)
					errChan <- err
				}
				wg.Done()
			}()
			c.mut.Unlock()
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	// TODO(jcreixell): Initialize manager here so we can return errors back early to the caller.
	// See https://github.com/grafana/agent/pull/2688#discussion_r1152384425
	c.mut.Lock()
	cfg := args.(operator.Arguments)
	c.config = &cfg
	c.mut.Unlock()

	if cfg.Scrape.EnableTypeAndUnitLabels && !c.opts.MinStability.Permits(featuregate.StabilityExperimental) {
		return fmt.Errorf("enable_type_and_unit_labels is an experimental feature, and must be enabled by setting the stability.level flag to experimental")
	}

	select {
	case c.onUpdate <- struct{}{}:
	default:
	}
	return nil
}

// NotifyClusterChange implements component.ClusterComponent.
func (c *Component) NotifyClusterChange() {
	c.mut.RLock()
	defer c.mut.RUnlock()

	if !c.config.Clustering.Enabled {
		return // no-op
	}

	if c.manager != nil {
		c.manager.ClusteringUpdated()
	}
}

// DebugInfo returns debug information for this component.
func (c *Component) DebugInfo() any {
	return c.manager.DebugInfo()
}

func (c *Component) reportHealth(err error) {
	c.healthMut.Lock()
	defer c.healthMut.Unlock()

	if err != nil {
		c.health = component.Health{
			Health:     component.HealthTypeUnhealthy,
			Message:    err.Error(),
			UpdateTime: time.Now(),
		}
		return
	} else {
		c.health = component.Health{
			Health:     component.HealthTypeHealthy,
			UpdateTime: time.Now(),
		}
	}
}

func (c *Component) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// very simple path handling
		// only responds to `/scrapeConfig/$NS/$NAME`
		c.mut.RLock()
		man := c.manager
		c.mut.RUnlock()
		path := strings.Trim(r.URL.Path, "/")
		parts := strings.Split(path, "/")
		if man == nil || len(parts) != 3 || parts[0] != "scrapeConfig" {
			w.WriteHeader(404)
			return
		}
		ns := parts[1]
		name := parts[2]
		scs := man.GetScrapeConfig(ns, name)
		if len(scs) == 0 {
			w.WriteHeader(404)
			return
		}
		dat, err := yaml.Marshal(scs)
		if err != nil {
			if _, err = w.Write([]byte(err.Error())); err != nil {
				return
			}
			w.WriteHeader(500)
			return
		}
		_, err = w.Write(dat)
		if err != nil {
			w.WriteHeader(500)
		}
	})
}
