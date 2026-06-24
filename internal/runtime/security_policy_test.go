package runtime

import (
	"errors"
	"io"
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/internal/controller"
	"github.com/grafana/alloy/internal/runtime/internal/testcomponents"
	"github.com/grafana/alloy/internal/runtime/internal/worker"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

// stubPolicy is a minimal SecurityPolicyChecker for use in tests.
type stubPolicy struct {
	deniedComponents map[string]bool
	deniedEndpoints  map[string]bool
}

func (p *stubPolicy) CheckComponent(name string) error {
	if p.deniedComponents[name] {
		return errors.New("component denied by policy")
	}
	return nil
}

func (p *stubPolicy) CheckEndpoint(url string) error {
	if p.deniedEndpoints[url] {
		return errors.New("endpoint denied by policy")
	}
	return nil
}

// testEgressArgs is a minimal Arguments type with alloy tags so the VM can
// decode the URL from the config block, and EgressSpec() surfaces it.
type testEgressArgs struct {
	URL        string `alloy:"url,attr,optional"`
	HasDynamic bool   `alloy:"has_dynamic,attr,optional"`
}

func (a testEgressArgs) EgressSpec() component.EgressSpec {
	var endpoints []string
	if a.URL != "" {
		endpoints = []string{a.URL}
	}
	return component.EgressSpec{Endpoints: endpoints, HasDynamic: a.HasDynamic}
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

func testEgressRegistry() component.Registry {
	return component.NewRegistryMap(featuregate.StabilityGenerallyAvailable, false,
		map[string]component.Registration{
			"test.egress": {
				Name:      "test.egress",
				Stability: featuregate.StabilityGenerallyAvailable,
				Args:      testEgressArgs{},
				Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
					return &testcomponents.Fake{}, nil
				},
			},
		})
}

func TestSecurityPolicy_AllowedComponentLoads(t *testing.T) {
	opts := testOptions(t)
	opts.SecurityPolicy = &stubPolicy{deniedComponents: map[string]bool{"test.blocked": true}}

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
	opts.SecurityPolicy = &stubPolicy{deniedComponents: map[string]bool{"test.blocked": true}}

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

// --- Endpoint gate tests ---
// egressArgs is registered as the Args type for test components below so that
// EgressSpec() is available on the resolved arguments at evaluate time.
// The alloy config syntax `test.egress "x" {}` will decode to egressArgs{}.
// To inject specific endpoint values we use the Build function to capture args.

func TestSecurityPolicy_AllowedEndpointLoads(t *testing.T) {
	opts := testOptions(t)
	opts.SecurityPolicy = &stubPolicy{
		deniedEndpoints: map[string]bool{"https://evil.com/exfil": true},
	}

	cfg := `test.egress "x" { url = "https://allowed.com/push" }`
	src, err := ParseSource(t.Name(), []byte(cfg))
	require.NoError(t, err)

	ctrl, err := newController(controllerOptions{
		Options:           opts,
		ComponentRegistry: testEgressRegistry(),
		ModuleRegistry:    newModuleRegistry(),
	})
	require.NoError(t, err)
	require.NoError(t, ctrl.LoadSource(src, nil, ""))
}

func TestSecurityPolicy_DeniedEndpointBlocksLoad(t *testing.T) {
	opts := testOptions(t)
	opts.SecurityPolicy = &stubPolicy{
		deniedEndpoints: map[string]bool{"https://evil.com/exfil": true},
	}

	cfg := `test.egress "x" { url = "https://evil.com/exfil" }`
	src, err := ParseSource(t.Name(), []byte(cfg))
	require.NoError(t, err)

	ctrl, err := newController(controllerOptions{
		Options:           opts,
		ComponentRegistry: testEgressRegistry(),
		ModuleRegistry:    newModuleRegistry(),
	})
	require.NoError(t, err)
	err = ctrl.LoadSource(src, nil, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "endpoint denied by policy")
}

func TestSecurityPolicy_NilPolicyAllowsAnyEndpoint(t *testing.T) {
	opts := testOptions(t)

	cfg := `test.egress "x" { url = "https://evil.com/exfil" }`
	src, err := ParseSource(t.Name(), []byte(cfg))
	require.NoError(t, err)

	ctrl, err := newController(controllerOptions{
		Options:           opts,
		ComponentRegistry: testEgressRegistry(),
		ModuleRegistry:    newModuleRegistry(),
	})
	require.NoError(t, err)
	require.NoError(t, ctrl.LoadSource(src, nil, ""))
}

// TestSecurityPolicy_EnforcedInModuleController tests that endpoint policy is enforced
// inside module controllers (the code path used by import.http / user_pipeline).
// This was previously broken because moduleControllerOptions didn't carry the policy.
func TestSecurityPolicy_EnforcedInModuleController(t *testing.T) {
	s, err := logging.New(io.Discard, logging.DefaultOptions)
	require.NoError(t, err)

	policy := &stubPolicy{deniedEndpoints: map[string]bool{"https://evil.com/exfil": true}}

	serviceMap := controller.NewServiceMap([]service.Service{})
	modCtrlOpts := &moduleControllerOptions{
		Logger:            s,
		DataPath:          t.TempDir(),
		MinStability:      featuregate.StabilityPublicPreview,
		Reg:               prometheus.NewRegistry(),
		ModuleRegistry:    newModuleRegistry(),
		WorkerPool:        worker.NewFixedWorkerPool(1, 100),
		ServiceMap:        serviceMap,
		ComponentRegistry: testEgressRegistry(),
		SecurityPolicy:    policy,
	}

	modCtrl := newModuleController(modCtrlOpts)
	mod, err := modCtrl.NewModule("test", nil)
	require.NoError(t, err)

	// Denied endpoint inside the module must be rejected.
	err = mod.LoadConfig([]byte(`test.egress "x" { url = "https://evil.com/exfil" }`), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "endpoint denied by policy")
}

func TestSecurityPolicy_AllowedEndpointInModuleController(t *testing.T) {
	s, err := logging.New(io.Discard, logging.DefaultOptions)
	require.NoError(t, err)

	policy := &stubPolicy{deniedEndpoints: map[string]bool{"https://evil.com/exfil": true}}

	serviceMap := controller.NewServiceMap([]service.Service{})
	modCtrlOpts := &moduleControllerOptions{
		Logger:            s,
		DataPath:          t.TempDir(),
		MinStability:      featuregate.StabilityPublicPreview,
		Reg:               prometheus.NewRegistry(),
		ModuleRegistry:    newModuleRegistry(),
		WorkerPool:        worker.NewFixedWorkerPool(1, 100),
		ServiceMap:        serviceMap,
		ComponentRegistry: testEgressRegistry(),
		SecurityPolicy:    policy,
	}

	modCtrl := newModuleController(modCtrlOpts)
	mod, err := modCtrl.NewModule("test", nil)
	require.NoError(t, err)

	err = mod.LoadConfig([]byte(`test.egress "x" { url = "https://allowed.com/push" }`), nil)
	require.NoError(t, err)
}
