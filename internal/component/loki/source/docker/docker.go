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
	"sync"
	"time"

	"github.com/docker/docker/client"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/alloy/logging/level"
	"github.com/grafana/alloy/internal/component"
	types "github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/discovery"
	dt "github.com/grafana/alloy/internal/component/loki/source/docker/internal/dockertarget"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/useragent"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
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
	metrics *dt.Metrics

	mut           sync.RWMutex
	args          Arguments
	manager       *manager
	lastOptions   *options
	handler       loki.LogsReceiver
	posFile       positions.Positions
	rcs           []*relabel.Config
	defaultLabels model.LabelSet

	receiversMut sync.RWMutex
	receivers    []loki.LogsReceiver
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
		opts:    o,
		metrics: dt.NewMetrics(o.Registerer),

		handler:   loki.NewLogsReceiver(),
		manager:   newManager(o.Logger, nil),
		receivers: args.ForwardTo,
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
	defer c.posFile.Stop()

	defer func() {
		c.mut.Lock()
		defer c.mut.Unlock()

		// Guard for safety, but it's not possible for Run to be called without
		// c.tailer being initialized.
		if c.manager != nil {
			c.manager.stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.handler.Chan():
			c.receiversMut.RLock()
			receivers := c.receivers
			c.receiversMut.RUnlock()
			for _, receiver := range receivers {
				receiver.Chan() <- entry
			}
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	// Update the receivers before anything else, just in case something fails.
	c.receiversMut.Lock()
	c.receivers = newArgs.ForwardTo
	c.receiversMut.Unlock()

	c.mut.Lock()
	defer c.mut.Unlock()

	managerOpts, err := c.getManagerOptions(newArgs)
	if err != nil {
		return err
	}

	if managerOpts != c.lastOptions {
		// Options changed; pass it to the tailer.
		// This will never fail because it only fails if the context gets canceled.
		_ = c.manager.updateOptions(context.Background(), managerOpts)
		c.lastOptions = managerOpts
	}

	defaultLabels := make(model.LabelSet, len(newArgs.Labels))
	for k, v := range newArgs.Labels {
		defaultLabels[model.LabelName(k)] = model.LabelValue(v)
	}
	c.defaultLabels = defaultLabels

	if newArgs.RelabelRules != nil && len(newArgs.RelabelRules) > 0 {
		c.rcs = alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelRules)
	} else {
		c.rcs = []*relabel.Config{}
	}

	// Convert input targets into targets to give to tailer.
	targets := make([]*dt.Target, 0, len(newArgs.Targets))
	seenTargets := make(map[string]struct{}, len(newArgs.Targets))

	for _, target := range newArgs.Targets {
		containerID, ok := target[dockerLabelContainerID]
		if !ok {
			level.Debug(c.opts.Logger).Log("msg", "docker target did not include container ID label:"+dockerLabelContainerID)
			continue
		}
		if _, seen := seenTargets[containerID]; seen {
			continue
		}
		seenTargets[containerID] = struct{}{}

		var labels = make(model.LabelSet)
		for k, v := range target {
			labels[model.LabelName(k)] = model.LabelValue(v)
		}

		tgt, err := dt.NewTarget(
			c.metrics,
			log.With(c.opts.Logger, "target", fmt.Sprintf("docker/%s", containerID)),
			c.manager.opts.handler,
			c.manager.opts.positions,
			containerID,
			labels.Merge(c.defaultLabels),
			c.rcs,
			c.manager.opts.client,
		)
		if err != nil {
			return err
		}
		targets = append(targets, tgt)
	}

	// This will never fail because it only fails if the context gets canceled.
	_ = c.manager.syncTargets(context.Background(), targets)

	c.args = newArgs
	return nil
}

// getTailerOptions gets tailer options from arguments. If args hasn't changed
// from the last call to getTailerOptions, c.lastOptions is returned.
// c.lastOptions must be updated by the caller.
//
// getTailerOptions must only be called when c.mut is held.
func (c *Component) getManagerOptions(args Arguments) (*options, error) {
	if reflect.DeepEqual(c.args.Host, args.Host) && c.lastOptions != nil {
		return c.lastOptions, nil
	}

	hostURL, err := url.Parse(args.Host)
	if err != nil {
		return c.lastOptions, err
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
			return c.lastOptions, err
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
		return c.lastOptions, fmt.Errorf("failed to build docker client: %w", err)
	}

	return &options{
		client:              client,
		handler:             loki.NewEntryHandler(c.handler.Chan(), func() {}),
		positions:           c.posFile,
		taskRestartInterval: 5 * time.Second,
	}, nil
}

// DebugInfo returns information about the status of tailed targets.
func (c *Component) DebugInfo() interface{} {
	var res readerDebugInfo
	for _, tgt := range c.manager.targets() {
		details := tgt.Details()
		res.TargetsInfo = append(res.TargetsInfo, targetInfo{
			Labels:     tgt.LabelsStr(),
			ID:         details["id"],
			LastError:  details["error"],
			IsRunning:  details["running"],
			ReadOffset: details["position"],
		})
	}
	return res
}

type readerDebugInfo struct {
	TargetsInfo []targetInfo `alloy:"targets_info,block"`
}

type targetInfo struct {
	ID         string `alloy:"id,attr"`
	LastError  string `alloy:"last_error,attr"`
	Labels     string `alloy:"labels,attr"`
	IsRunning  string `alloy:"is_running,attr"`
	ReadOffset string `alloy:"read_offset,attr"`
}
