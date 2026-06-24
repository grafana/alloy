package runtime

import (
	"errors"
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/internal/testcomponents"
	"github.com/stretchr/testify/require"
)

// stubPolicy is a minimal SecurityPolicyChecker for use in tests.
type stubPolicy struct {
	denied map[string]bool
}

func (p *stubPolicy) CheckComponent(name string) error {
	if p.denied[name] {
		return errors.New("component denied by policy")
	}
	return nil
}

func testRegistryWithComponents(names ...string) component.Registry {
	regs := make(map[string]component.Registration, len(names))
	for _, name := range names {
		n := name
		regs[n] = component.Registration{
			Name:      n,
			Stability: featuregate.StabilityGenerallyAvailable,
			Args:      struct{}{},
			Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
				return &testcomponents.Fake{}, nil
			},
		}
	}
	return component.NewRegistryMap(featuregate.StabilityGenerallyAvailable, false, regs)
}

func TestSecurityPolicy_AllowedComponentLoads(t *testing.T) {
	opts := testOptions(t)
	opts.SecurityPolicy = &stubPolicy{denied: map[string]bool{"test.blocked": true}}

	cfg := `test.allowed "x" {}`
	src, err := ParseSource(t.Name(), []byte(cfg))
	require.NoError(t, err)

	ctrl, err := newController(controllerOptions{
		Options:           opts,
		ComponentRegistry: testRegistryWithComponents("test.allowed", "test.blocked"),
		ModuleRegistry:    newModuleRegistry(),
	})
	require.NoError(t, err)
	require.NoError(t, ctrl.LoadSource(src, nil, ""))
}

func TestSecurityPolicy_DeniedComponentBlocksLoad(t *testing.T) {
	opts := testOptions(t)
	opts.SecurityPolicy = &stubPolicy{denied: map[string]bool{"test.blocked": true}}

	cfg := `test.blocked "x" {}`
	src, err := ParseSource(t.Name(), []byte(cfg))
	require.NoError(t, err)

	ctrl, err := newController(controllerOptions{
		Options:           opts,
		ComponentRegistry: testRegistryWithComponents("test.allowed", "test.blocked"),
		ModuleRegistry:    newModuleRegistry(),
	})
	require.NoError(t, err)
	err = ctrl.LoadSource(src, nil, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "denied by policy")
}

func TestSecurityPolicy_NilPolicyAllowsAll(t *testing.T) {
	opts := testOptions(t)
	// SecurityPolicy is nil by default — everything should load.

	cfg := `test.blocked "x" {}`
	src, err := ParseSource(t.Name(), []byte(cfg))
	require.NoError(t, err)

	ctrl, err := newController(controllerOptions{
		Options:           opts,
		ComponentRegistry: testRegistryWithComponents("test.blocked"),
		ModuleRegistry:    newModuleRegistry(),
	})
	require.NoError(t, err)
	require.NoError(t, ctrl.LoadSource(src, nil, ""))
}
