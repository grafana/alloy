package metricsutils

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"

	v1 "github.com/grafana/agent/internal/static/integrations"
	v2 "github.com/grafana/agent/internal/static/integrations/v2"
	"github.com/grafana/agent/internal/static/integrations/v2/common"
	"github.com/grafana/agent/internal/util"
)

// NewNamedShim returns a v2.UpgradeFunc which will upgrade a v1.Config to a
// v2.Config with a new name.
func NewNamedShim(newName string) v2.UpgradeFunc {
	return func(before v1.Config, common common.MetricsConfig) v2.UpgradedConfig {
		return &ConfigShim{
			Orig:         before,
			Common:       common,
			nameOverride: newName,
		}
	}
}

// Shim upgrades a v1.Config to a v2.Config. The resulting config is NOT
// registered. Shim matches the v2.UpgradeFunc type.
func Shim(before v1.Config, common common.MetricsConfig) (after v2.UpgradedConfig) {
	return &ConfigShim{Orig: before, Common: common}
}

type ConfigShim struct {
	Orig         v1.Config
	Common       common.MetricsConfig
	nameOverride string
}

var (
	_ v2.Config           = (*ConfigShim)(nil)
	_ v2.UpgradedConfig   = (*ConfigShim)(nil)
	_ v2.ComparableConfig = (*ConfigShim)(nil)
)

func (s *ConfigShim) LegacyConfig() (v1.Config, common.MetricsConfig) { return s.Orig, s.Common }

func (s *ConfigShim) Name() string {
	if s.nameOverride != "" {
		return s.nameOverride
	}
	return s.Orig.Name()
}

func (s *ConfigShim) ApplyDefaults(g v2.Globals) error {
	s.Common.ApplyDefaults(g.SubsystemOpts.Metrics.Autoscrape)
	if id, err := s.Identifier(g); err == nil {
		s.Common.InstanceKey = &id
	}
	return nil
}

func (s *ConfigShim) ConfigEquals(c v2.Config) bool {
	o, ok := c.(*ConfigShim)
	if !ok {
		return false
	}
	return util.CompareYAML(s.Orig, o.Orig) && util.CompareYAML(s.Common, o.Common)
}

func (s *ConfigShim) Identifier(g v2.Globals) (string, error) {
	if s.Common.InstanceKey != nil {
		return *s.Common.InstanceKey, nil
	}
	return s.Orig.InstanceKey(g.AgentIdentifier)
}

func (s *ConfigShim) NewIntegration(l log.Logger, g v2.Globals) (v2.Integration, error) {
	v1Integration, err := s.Orig.NewIntegration(l)
	if err != nil {
		return nil, err
	}

	id, err := s.Identifier(g)
	if err != nil {
		return nil, err
	}

	// Generate our handler. Original integrations didn't accept a prefix, and
	// just assumed that they would be wired to /metrics somewhere.
	handler, err := v1Integration.MetricsHandler()
	if err != nil {
		return nil, fmt.Errorf("generating http handler: %w", err)
	} else if handler == nil {
		handler = http.NotFoundHandler()
	}

	// Generate targets. Original integrations used a static set of targets,
	// so this mapping can always be generated just once.
	//
	// Targets are generated from the result of ScrapeConfigs(), which returns a
	// tuple of job name and relative metrics path.
	//
	// Job names were prefixed at the subsystem level with integrations/, so we
	// will retain that behavior here.
	v1ScrapeConfigs := v1Integration.ScrapeConfigs()
	targets := make([]handlerTarget, 0, len(v1ScrapeConfigs))
	for _, sc := range v1ScrapeConfigs {
		targets = append(targets, handlerTarget{
			MetricsPath: sc.MetricsPath,
			Labels: model.LabelSet{
				model.JobLabel: model.LabelValue("integrations/" + sc.JobName),
			},
		})
	}

	// Convert the run function. Original integrations sometimes returned
	// ctx.Err() on exit. This isn't recommended anymore, but we need to hide the
	// error if it happens, since the error was previously ignored.
	runFunc := func(ctx context.Context) error {
		err := v1Integration.Run(ctx)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, context.Canceled) && ctx.Err() != nil:
			// Hide error that no longer happens in newer integrations.
			return nil
		default:
			return err
		}
	}

	// Aggregate our converted settings into a v2 integration.
	return &metricsHandlerIntegration{
		integrationName: s.Name(),
		instanceID:      id,

		common:  s.Common,
		globals: g,
		handler: handler,
		targets: targets,

		runFunc: runFunc,
	}, nil
}
