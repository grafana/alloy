package cloudflare

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.cloudflare",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the
// loki.source.cloudflare component.
type Arguments struct {
	APIToken         alloytypes.Secret   `alloy:"api_token,attr"`
	ZoneID           string              `alloy:"zone_id,attr"`
	APIURL           string              `alloy:"api_url,attr,optional"`
	Labels           map[string]string   `alloy:"labels,attr,optional"`
	Workers          int                 `alloy:"workers,attr,optional"`
	PullRange        time.Duration       `alloy:"pull_range,attr,optional"`
	FieldsType       FieldsType          `alloy:"fields_type,attr,optional"`
	AdditionalFields []string            `alloy:"additional_fields,attr,optional"`
	ForwardTo        []loki.LogsReceiver `alloy:"forward_to,attr"`
}

func (c Arguments) tailerConfig() *tailerConfig {
	lbls := make(model.LabelSet, len(c.Labels))
	for k, v := range c.Labels {
		lbls[model.LabelName(k)] = model.LabelValue(v)
	}
	return &tailerConfig{
		APIToken:         string(c.APIToken),
		ZoneID:           c.ZoneID,
		APIURL:           strings.TrimSuffix(c.APIURL, "/"),
		Labels:           lbls,
		Workers:          c.Workers,
		PullRange:        model.Duration(c.PullRange),
		FieldsType:       c.FieldsType,
		AdditionalFields: c.AdditionalFields,
		Backoff:          defaultBackoff,
	}
}

// DefaultArguments sets the configuration defaults.
var DefaultArguments = Arguments{
	APIURL:     defaultAPIURL,
	Workers:    3,
	PullRange:  1 * time.Minute,
	FieldsType: FieldsTypeDefault,
}

// SetToDefault implements syntax.Defaulter.
func (c *Arguments) SetToDefault() {
	*c = DefaultArguments
}

// Validate implements syntax.Validator.
func (c *Arguments) Validate() error {
	if c.APIToken == "" {
		return fmt.Errorf("api_token must not be empty")
	}
	if _, err := fieldsForType(c.FieldsType, c.AdditionalFields); err != nil {
		return err
	}
	if c.PullRange < 0 {
		return fmt.Errorf("pull_range must be a positive duration")
	}
	u, err := url.Parse(c.APIURL)
	if err != nil {
		return fmt.Errorf("api_url is not a valid URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("api_url must be an absolute http or https URL, got %q", c.APIURL)
	}
	return nil
}

// Component implements the loki.source.cloudflare component.
type Component struct {
	opts    component.Options
	posFile positions.Positions
	metrics *metrics
	runner  *loki.SourceRunner[*tailerConfig]

	// mut protects tailer, the most recently started tailer.
	mut    sync.RWMutex
	tailer *tailer
}

var (
	_ component.Component       = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
	_ component.DebugComponent  = (*Component)(nil)
)

// New creates a new loki.source.cloudflare component.
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
		metrics: newMetrics(o.Registerer),
		posFile: positionsFile,
	}
	c.runner = loki.NewSourceRunner(loki.SourceRunnerOptions[*tailerConfig]{
		Logger: o.Logger,
		Start:  c.startTailer,
		// Stop positions first so entries that are still being drained don't
		// advance them.
		OnShutdown: c.posFile.Stop,
	})

	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Component) startTailer(_ context.Context, cfg *tailerConfig) (loki.Producer[*tailerConfig], error) {
	t, err := newTailer(c.metrics, c.opts.Logger, c.runner.Entries(), c.posFile, cfg)
	if err != nil {
		return loki.Producer[*tailerConfig]{}, fmt.Errorf("failed to create cloudflare target with provided config: %w", err)
	}
	c.mut.Lock()
	c.tailer = t
	c.mut.Unlock()

	return loki.Producer[*tailerConfig]{
		Stop: func() {
			t.Stop()
			// Stop reporting debug info for a tailer that no longer runs.
			c.mut.Lock()
			defer c.mut.Unlock()
			if c.tailer == t {
				c.tailer = nil
			}
		},
		Exited: t.exited,
	}, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	c.runner.Run(ctx)
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	c.runner.Update(newArgs.ForwardTo, newArgs.tailerConfig())
	return nil
}

// CurrentHealth implements component.HealthComponent.
func (c *Component) CurrentHealth() component.Health {
	return c.runner.CurrentHealth()
}

// DebugInfo returns information about the status of targets.
func (c *Component) DebugInfo() any {
	c.mut.RLock()
	defer c.mut.RUnlock()

	if c.tailer == nil {
		return targetDebugInfo{}
	}
	return targetDebugInfo{
		Ready:   c.tailer.ready(),
		Details: c.tailer.details(),
	}
}

type targetDebugInfo struct {
	Ready   bool              `alloy:"ready,attr"`
	Details map[string]string `alloy:"target_info,attr"`
}
