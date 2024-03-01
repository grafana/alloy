package integrations

import (
	"context"
	nethttp "net/http"
	"testing"

	"github.com/go-kit/log"
	"github.com/grafana/agent/internal/static/integrations/v2/autoscrape"
	"github.com/grafana/agent/internal/util"
	"github.com/prometheus/common/model"
	prom_config "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/http"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/require"
)

//
// Tests for controller's utilization of the MetricsIntegration interface.
//

func Test_controller_MetricsIntegration_Targets(t *testing.T) {
	integrationWithTarget := func(targetName string) Integration {
		return mockMetricsIntegration{
			HTTPIntegration: newWaitStartedIntegration(),
			TargetsFunc: func(Endpoint) []*targetgroup.Group {
				return []*targetgroup.Group{{
					Targets: []model.LabelSet{{model.AddressLabel: model.LabelValue(targetName)}},
				}}
			},
			ScrapeConfigsFunc: func(c discovery.Configs) []*autoscrape.ScrapeConfig { return nil },
		}
	}

	integrations := []Config{
		mockConfigNameTuple(t, "a", "instanceA").WithNewIntegrationFunc(func(l log.Logger, g Globals) (Integration, error) {
			return integrationWithTarget("a"), nil
		}),
		mockConfigNameTuple(t, "b", "instanceB").WithNewIntegrationFunc(func(l log.Logger, g Globals) (Integration, error) {
			return integrationWithTarget("b"), nil
		}),
	}

	// waitIntegrations starts a controller and waits for all of its integrations
	// to run.
	waitIntegrations := func(t *testing.T, ctrl *controller) {
		t.Helper()
		_ = newSyncController(t, ctrl)
		err := ctrl.forEachIntegration("/", func(ci *controlledIntegration, _ string) {
			wsi := ci.i.(mockMetricsIntegration).HTTPIntegration.(*waitStartedIntegration)
			_ = wsi.trigger.WaitContext(context.Background())
		})
		require.NoError(t, err)
	}

	t.Run("All", func(t *testing.T) {
		ctrl, err := newController(
			util.TestLogger(t),
			controllerConfig(integrations),
			Globals{},
		)
		require.NoError(t, err)
		waitIntegrations(t, ctrl)

		result := ctrl.Targets(Endpoint{Prefix: "/"}, TargetOptions{})
		expect := []*targetGroup{
			{Targets: []model.LabelSet{{model.AddressLabel: "a"}}},
			{Targets: []model.LabelSet{{model.AddressLabel: "b"}}},
		}
		require.Equal(t, expect, result)
	})

	t.Run("All by Integration", func(t *testing.T) {
		ctrl, err := newController(
			util.TestLogger(t),
			controllerConfig(integrations),
			Globals{},
		)
		require.NoError(t, err)
		waitIntegrations(t, ctrl)

		result := ctrl.Targets(Endpoint{Prefix: "/"}, TargetOptions{
			Integrations: []string{"a", "b"},
		})
		expect := []*targetGroup{
			{Targets: []model.LabelSet{{model.AddressLabel: "a"}}},
			{Targets: []model.LabelSet{{model.AddressLabel: "b"}}},
		}
		require.Equal(t, expect, result)
	})

	t.Run("Specific Integration", func(t *testing.T) {
		ctrl, err := newController(
			util.TestLogger(t),
			controllerConfig(integrations),
			Globals{},
		)
		require.NoError(t, err)
		waitIntegrations(t, ctrl)

		result := ctrl.Targets(Endpoint{Prefix: "/"}, TargetOptions{
			Integrations: []string{"a"},
		})
		expect := []*targetGroup{
			{Targets: []model.LabelSet{{model.AddressLabel: "a"}}},
		}
		require.Equal(t, expect, result)
	})
}

func Test_controller_MetricsIntegration_ScrapeConfig(t *testing.T) {
	integrationWithTarget := func(targetName string) Integration {
		return mockMetricsIntegration{
			HTTPIntegration: NoOpIntegration,
			ScrapeConfigsFunc: func(c discovery.Configs) []*autoscrape.ScrapeConfig {
				return []*autoscrape.ScrapeConfig{{
					Instance: "default",
					Config:   prom_config.ScrapeConfig{JobName: targetName},
				}}
			},
		}
	}

	integrations := []Config{
		mockConfigNameTuple(t, "a", "instanceA").WithNewIntegrationFunc(func(l log.Logger, g Globals) (Integration, error) {
			return integrationWithTarget("a"), nil
		}),
		mockConfigNameTuple(t, "b", "instanceB").WithNewIntegrationFunc(func(l log.Logger, g Globals) (Integration, error) {
			return integrationWithTarget("b"), nil
		}),
	}

	ctrl, err := newController(
		util.TestLogger(t),
		controllerConfig(integrations),
		Globals{},
	)
	require.NoError(t, err)
	_ = newSyncController(t, ctrl)

	result := ctrl.ScrapeConfigs("/", &http.DefaultSDConfig)
	expect := []*autoscrape.ScrapeConfig{
		{Instance: "default", Config: prom_config.ScrapeConfig{JobName: "a"}},
		{Instance: "default", Config: prom_config.ScrapeConfig{JobName: "b"}},
	}
	require.Equal(t, expect, result)
}

//
// Tests for controller's utilization of the MetricsIntegration interface.
//

type waitStartedIntegration struct {
	trigger *util.WaitTrigger
}

func newWaitStartedIntegration() *waitStartedIntegration {
	return &waitStartedIntegration{trigger: util.NewWaitTrigger()}
}

func (i *waitStartedIntegration) RunIntegration(ctx context.Context) error {
	i.trigger.Trigger()
	<-ctx.Done()
	return nil
}

func (i *waitStartedIntegration) Handler(prefix string) (nethttp.Handler, error) {
	return nil, nil
}

type mockMetricsIntegration struct {
	HTTPIntegration
	TargetsFunc       func(ep Endpoint) []*targetgroup.Group
	ScrapeConfigsFunc func(discovery.Configs) []*autoscrape.ScrapeConfig
}

func (m mockMetricsIntegration) Targets(ep Endpoint) []*targetgroup.Group {
	return m.TargetsFunc(ep)
}

func (m mockMetricsIntegration) ScrapeConfigs(cfgs discovery.Configs) []*autoscrape.ScrapeConfig {
	return m.ScrapeConfigsFunc(cfgs)
}
