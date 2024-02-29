package integrations

import (
	"context"
	"sync"
	"testing"

	"github.com/go-kit/log"
	"github.com/grafana/agent/internal/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

//
// Tests for controller's utilization of the UpdateIntegration interface.
//

// Test_controller_UpdateIntegration ensures that the controller will call
// UpdateIntegration for integrations that support it.
func Test_controller_UpdateIntegration(t *testing.T) {
	var (
		integrationStartWg sync.WaitGroup
		applies, starts    atomic.Uint64
	)

	mockIntegration := mockUpdateIntegration{
		Integration: FuncIntegration(func(ctx context.Context) error {
			starts.Inc()
			integrationStartWg.Done()
			<-ctx.Done()
			return nil
		}),
		ApplyConfigFunc: func(Config, Globals) error {
			applies.Inc()
			return nil
		},
	}

	cfg := controllerConfig{
		mockConfig{
			NameFunc:          func() string { return mockIntegrationName },
			ConfigEqualsFunc:  func(Config) bool { return false },
			ApplyDefaultsFunc: func(g Globals) error { return nil },
			IdentifierFunc: func(Globals) (string, error) {
				return mockIntegrationName, nil
			},
			NewIntegrationFunc: func(log.Logger, Globals) (Integration, error) {
				integrationStartWg.Add(1)
				return mockIntegration, nil
			},
		},
	}

	ctrl, err := newController(util.TestLogger(t), cfg, Globals{})
	require.NoError(t, err, "failed to create controller")

	sc := newSyncController(t, ctrl)

	// Wait for our integration to start.
	integrationStartWg.Wait()

	// Try to apply again.
	require.NoError(t, sc.UpdateController(cfg, ctrl.globals), "failed to re-apply config")
	integrationStartWg.Wait()

	sc.Stop()

	require.Equal(t, uint64(1), applies.Load(), "dynamic reload should have occurred")
	require.Equal(t, uint64(1), starts.Load(), "restart should not have occurred")
}

type mockUpdateIntegration struct {
	Integration
	ApplyConfigFunc func(Config, Globals) error
}

func (m mockUpdateIntegration) ApplyConfig(c Config, g Globals) error {
	return m.ApplyConfigFunc(c, g)
}
