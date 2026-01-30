package docker

// NOTE: This code is adapted from Promtail (90a1d4593e2d690b37333386383870865fe177bf).

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/docker/docker/client"
	"github.com/go-kit/log"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	types "github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/common/loki"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/loki/source"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/useragent"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.docker",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

var userAgent = useragent.Get()

const (
	dockerLabel                = model.MetaLabelPrefix + "docker_"
	dockerLabelContainerPrefix = dockerLabel + "container_"
	dockerLabelContainerID     = dockerLabelContainerPrefix + "id"
	dockerLabelLogStream       = dockerLabelContainerPrefix + "log_stream"
	dockerMaxChunkSize         = 16384
)

// Arguments holds values which are used to configure the loki.source.docker
// component.
type Arguments struct {
	Host             string                  `alloy:"host,attr"`
	Targets          []discovery.Target      `alloy:"targets,attr"`
	ForwardTo        []loki.LogsReceiver     `alloy:"forward_to,attr"`
	Labels           map[string]string       `alloy:"labels,attr,optional"`
	RelabelRules     alloy_relabel.Rules     `alloy:"relabel_rules,attr,optional"`
	HTTPClientConfig *types.HTTPClientConfig `alloy:"http_client_config,block,optional"`
	RefreshInterval  time.Duration           `alloy:"refresh_interval,attr,optional"`
}

// GetDefaultArguments return an instance of Arguments with the optional fields
// initialized.
func GetDefaultArguments() Arguments {
	return Arguments{
		HTTPClientConfig: types.CloneDefaultHTTPClientConfig(),
		RefreshInterval:  60 * time.Second,
	}
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = GetDefaultArguments()
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if _, err := url.Parse(a.Host); err != nil {
		return fmt.Errorf("failed to parse Docker host %q: %w", a.Host, err)
	}
	// We must explicitly Validate because HTTPClientConfig is squashed and it won't run otherwise
	if a.HTTPClientConfig != nil {
		if a.RefreshInterval <= 0 {
			return fmt.Errorf("refresh_interval must be positive, got %q", a.RefreshInterval)
		}
		return a.HTTPClientConfig.Validate()
	}

	return nil
}

var (
	_ component.Component      = (*Component)(nil)
	_ component.DebugComponent = (*Component)(nil)
)

// Component implements the loki.source.file component.
type Component struct {
	opts    component.Options
	metrics *metrics
	exited  *atomic.Bool

	mut       sync.RWMutex
	args      Arguments
	scheduler *source.Scheduler[string]
	client    client.APIClient
	handler   loki.LogsReceiver
	posFile   positions.Positions
	rcs       []*relabel.Config

	fanout *loki.Fanout
}

// New creates a new loki.source.file component.
func New(o component.Options, args Arguments) (*Component, error) {
	err := os.MkdirAll(o.DataPath, 0750)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	positionsFile, err := positions.New(o.Logger, positions.Config{
		SyncPeriod:        10 * time.Second,
		PositionsFile:     filepath.Join(o.DataPath, "positions.yml"),
		IgnoreInvalidYaml: false,
		ReadOnly:          false,
	})
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:      o,
		metrics:   newMetrics(o.Registerer),
		exited:    atomic.NewBool(false),
		handler:   loki.NewLogsReceiver(),
		scheduler: source.NewScheduler[string](),
		fanout:    loki.NewFanout(args.ForwardTo),
		posFile:   positionsFile,
	}

	// Call to Update() to start readers and set receivers once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.exited.Store(true)
		c.posFile.Stop()

		// Start black hole drain routine to prevent deadlock when we call c.scheduler.Stop().
		source.Drain(c.handler, func() {
			c.mut.Lock()
			defer c.mut.Unlock()
			c.scheduler.Stop()
		})
	}()

	// Start consume and fanout loop
	source.Consume(ctx, c.handler, c.fanout)
	return nil
}

type promTarget struct {
	labels      model.LabelSet
	fingerPrint model.Fingerprint
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()

	c.fanout.UpdateChildren(newArgs.ForwardTo)

	client, err := c.getClient(newArgs)
	if err != nil {
		return err
	}

	if client != c.client {
		c.client = client
		// Stop all tailers because we need to restart them.
		c.scheduler.Reset()
	}

	defaultLabels := make(model.LabelSet, len(newArgs.Labels))
	for k, v := range newArgs.Labels {
		defaultLabels[model.LabelName(k)] = model.LabelValue(v)
	}

	c.rcs = alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelRules)

	promTargets := make([]promTarget, len(newArgs.Targets))
	for i, target := range newArgs.Targets {
		labelsCopy := target.LabelSet()
		promTargets[i] = promTarget{labels: labelsCopy, fingerPrint: labelsCopy.Fingerprint()}
	}

	// Sorting the targets before filtering ensures consistent filtering of targets
	// when multiple targets share the same containerID.
	sort.Slice(promTargets, func(i, j int) bool {
		return promTargets[i].fingerPrint < promTargets[j].fingerPrint
	})

	source.Reconcile(
		c.opts.Logger,
		c.scheduler,
		slices.Values(promTargets),
		func(target promTarget) string { return string(target.labels[dockerLabelContainerID]) },
		func(containerID string, target promTarget) (source.Source[string], error) {
			if containerID == "" {
				level.Debug(c.opts.Logger).Log("msg", "docker target did not include container ID label:"+dockerLabelContainerID)
				return nil, source.ErrSkip
			}

			return newTailer(
				c.metrics,
				log.With(c.opts.Logger, "component", "tailer", "container", fmt.Sprintf("docker/%s", containerID)),
				c.handler,
				c.posFile,
				containerID,
				target.labels.Merge(defaultLabels),
				c.rcs,
				client,
				5*time.Second,
				func() bool { return c.exited.Load() },
			)
		},
	)

	c.args = newArgs
	return nil
}

// getClient creates a client from args. If args hasn't changed
// from the last call to getClient, c.client is returned.
// getClient must only be called when c.mut is held.
func (c *Component) getClient(args Arguments) (client.APIClient, error) {
	if reflect.DeepEqual(c.args.Host, args.Host) && c.client != nil {
		return c.client, nil
	}

	hostURL, err := url.Parse(args.Host)
	if err != nil {
		return c.client, err
	}

	opts := []client.Opt{
		client.WithHost(args.Host),
		client.WithAPIVersionNegotiation(),
	}

	// There are other protocols than HTTP supported by the Docker daemon, like
	// unix, which are not supported by the HTTP client. Passing HTTP client
	// options to the Docker client makes those non-HTTP requests fail.
	if hostURL.Scheme == "http" || hostURL.Scheme == "https" {
		rt, err := config.NewRoundTripperFromConfig(*args.HTTPClientConfig.Convert(), "docker_sd")
		if err != nil {
			return c.client, err
		}
		opts = append(opts,
			client.WithHTTPClient(&http.Client{
				Transport: rt,
				Timeout:   args.RefreshInterval,
			}),
			client.WithScheme(hostURL.Scheme),
			client.WithHTTPHeaders(map[string]string{
				"User-Agent": userAgent,
			}),
		)
	}

	client, err := client.NewClientWithOpts(opts...)
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "could not create new Docker client", "err", err)
		return c.client, fmt.Errorf("failed to build docker client: %w", err)
	}

	return client, nil
}

// DebugInfo returns information about the status of tailed targets.
func (c *Component) DebugInfo() any {
	c.mut.RLock()
	defer c.mut.RUnlock()

	var res readerDebugInfo
	for s := range c.scheduler.Sources() {
		t := s.(*tailer)
		res.TargetsInfo = append(res.TargetsInfo, t.DebugInfo())
	}
	return res
}

type readerDebugInfo struct {
	TargetsInfo []sourceInfo `alloy:"targets_info,block"`
}

type sourceInfo struct {
	ID         string `alloy:"id,attr"`
	LastError  string `alloy:"last_error,attr"`
	Labels     string `alloy:"labels,attr"`
	IsRunning  bool   `alloy:"is_running,attr"`
	ReadOffset string `alloy:"read_offset,attr"`
}
