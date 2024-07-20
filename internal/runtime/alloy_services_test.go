package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/internal/controller"
	"github.com/grafana/alloy/internal/runtime/internal/testcomponents"
	"github.com/grafana/alloy/internal/runtime/internal/testservices"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func TestServices(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		startedSvc = util.NewWaitTrigger()

		svc = &testservices.Fake{
			RunFunc: func(ctx context.Context, _ service.Host) error {
				startedSvc.Trigger()

				<-ctx.Done()
				return nil
			},
		}
	)

	opts := testOptions(t)
	opts.Services = append(opts.Services, svc)

	ctrl := New(opts)
	require.NoError(t, ctrl.LoadSource(makeEmptyFile(t), nil))

	// Start the controller. This should cause our service to run.
	go ctrl.Run(ctx)

	require.NoError(t, startedSvc.Wait(5*time.Second), "Service did not start")
}

func TestServices_Configurable(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)
	type ServiceOptions struct {
		Name string `alloy:"name,attr"`
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		updateCalled = util.NewWaitTrigger()

		svc = &testservices.Fake{
			DefinitionFunc: func() service.Definition {
				return service.Definition{
					Name:       "fake",
					ConfigType: ServiceOptions{},
					Stability:  featuregate.StabilityPublicPreview,
				}
			},

			UpdateFunc: func(newConfig any) error {
				defer updateCalled.Trigger()

				require.IsType(t, ServiceOptions{}, newConfig)
				require.Equal(t, "John Doe", newConfig.(ServiceOptions).Name)
				return nil
			},
		}
	)

	f, err := ParseSource(t.Name(), []byte(`
		fake {
			name = "John Doe"
		}
	`))
	require.NoError(t, err)
	require.NotNil(t, f)

	opts := testOptions(t)
	opts.Services = append(opts.Services, svc)

	ctrl := New(opts)

	require.NoError(t, ctrl.LoadSource(f, nil))

	// Start the controller. This should cause our service to run.
	go ctrl.Run(ctx)

	require.NoError(t, updateCalled.Wait(5*time.Second), "Service was not configured")
}

// TestServices_Configurable_Optional ensures that a service with optional
// arguments is configured properly even when it is not defined in the config
// file.
func TestServices_Configurable_Optional(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)
	type ServiceOptions struct {
		Name string `alloy:"name,attr,optional"`
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		updateCalled = util.NewWaitTrigger()

		svc = &testservices.Fake{
			DefinitionFunc: func() service.Definition {
				return service.Definition{
					Name:       "fake",
					ConfigType: ServiceOptions{},
					Stability:  featuregate.StabilityPublicPreview,
				}
			},

			UpdateFunc: func(newConfig any) error {
				defer updateCalled.Trigger()

				require.IsType(t, ServiceOptions{}, newConfig)
				require.Equal(t, ServiceOptions{}, newConfig.(ServiceOptions))
				return nil
			},
		}
	)

	opts := testOptions(t)
	opts.Services = append(opts.Services, svc)

	ctrl := New(opts)

	require.NoError(t, ctrl.LoadSource(makeEmptyFile(t), nil))

	// Start the controller. This should cause our service to run.
	go ctrl.Run(ctx)

	require.NoError(t, updateCalled.Wait(5*time.Second), "Service was not configured")
}

func TestAlloy_GetServiceConsumers(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)
	var (
		svcA = &testservices.Fake{
			DefinitionFunc: func() service.Definition {
				return service.Definition{
					Name: "svc_a",
				}
			},
		}

		svcB = &testservices.Fake{
			DefinitionFunc: func() service.Definition {
				return service.Definition{
					Name:      "svc_b",
					DependsOn: []string{"svc_a"},
				}
			},
		}
	)

	opts := testOptions(t)
	opts.Services = append(opts.Services, svcA, svcB)

	ctrl := New(opts)
	defer cleanUpController(ctrl)
	require.NoError(t, ctrl.LoadSource(makeEmptyFile(t), nil))

	expectConsumers := []service.Consumer{{
		Type:  service.ConsumerTypeService,
		ID:    "svc_b",
		Value: svcB,
	}}
	require.Equal(t, expectConsumers, ctrl.GetServiceConsumers("svc_a"))
}

func TestComponents_Using_Services(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		componentBuilt = util.NewWaitTrigger()
		serviceStarted = util.NewWaitTrigger()

		serviceStartedCount = atomic.NewInt64(0)
	)

	var (
		existsSvc = &testservices.Fake{
			DefinitionFunc: func() service.Definition {
				return service.Definition{Name: "exists"}
			},

			RunFunc: func(ctx context.Context, host service.Host) error {
				if serviceStartedCount.Add(1) > 1 {
					require.FailNow(t, "service should only be started once by the root controller")
				}

				serviceStarted.Trigger()

				<-ctx.Done()
				return nil
			},
		}

		registry = controller.NewRegistryMap(
			featuregate.StabilityGenerallyAvailable,
			true,
			map[string]component.Registration{
				"service_consumer": {
					Name:      "service_consumer",
					Stability: featuregate.StabilityGenerallyAvailable,
					Args:      struct{}{},
					Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
						// Call Trigger in a defer so we can make some extra assertions before
						// the test exits.
						defer componentBuilt.Trigger()

						_, err := opts.GetServiceData("exists")
						require.NoError(t, err, "component should be able to access services which exist")

						_, err = opts.GetServiceData("does_not_exist")
						require.Error(t, err, "component should not be able to access non-existent service")

						return &testcomponents.Fake{}, nil
					},
				},
			},
		)
	)

	cfg := `
		service_consumer "example" {}
	`

	f, err := ParseSource(t.Name(), []byte(cfg))
	require.NoError(t, err)
	require.NotNil(t, f)

	opts := testOptions(t)
	opts.Services = append(opts.Services, existsSvc)

	ctrl := newController(controllerOptions{
		Options:           opts,
		ComponentRegistry: registry,
		ModuleRegistry:    newModuleRegistry(),
	})
	require.NoError(t, ctrl.LoadSource(f, nil))
	go ctrl.Run(ctx)

	require.NoError(t, componentBuilt.Wait(5*time.Second), "Component should have been built")
	require.NoError(t, serviceStarted.Wait(5*time.Second), "Service should have been started")
}

func TestComponents_Using_Services_In_Modules(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	componentBuilt := util.NewWaitTrigger()

	var (
		existsSvc = &testservices.Fake{
			DefinitionFunc: func() service.Definition {
				return service.Definition{Name: "exists"}
			},
		}

		registry = controller.NewRegistryMap(
			featuregate.StabilityGenerallyAvailable,
			true,
			map[string]component.Registration{
				"module_loader": {
					Name:      "module_loader",
					Args:      struct{}{},
					Stability: featuregate.StabilityGenerallyAvailable,
					Build: func(opts component.Options, _ component.Arguments) (component.Component, error) {
						mod, err := opts.ModuleController.NewModule("", nil)
						require.NoError(t, err, "Failed to create module")

						err = mod.LoadConfig([]byte(`service_consumer "example" {}`), nil)
						require.NoError(t, err, "Failed to load module config")

						return &testcomponents.Fake{
							RunFunc: func(ctx context.Context) error {
								mod.Run(ctx)
								<-ctx.Done()
								return nil
							},
						}, nil
					},
				},

				"service_consumer": {
					Name:      "service_consumer",
					Args:      struct{}{},
					Stability: featuregate.StabilityGenerallyAvailable,
					Build: func(opts component.Options, _ component.Arguments) (component.Component, error) {
						// Call Trigger in a defer so we can make some extra assertions before
						// the test exits.
						defer componentBuilt.Trigger()

						_, err := opts.GetServiceData("exists")
						require.NoError(t, err, "component should be able to access services which exist")

						return &testcomponents.Fake{}, nil
					},
				},
			},
		)
	)

	cfg := `module_loader "example" {}`

	f, err := ParseSource(t.Name(), []byte(cfg))
	require.NoError(t, err)
	require.NotNil(t, f)

	opts := testOptions(t)
	opts.Services = append(opts.Services, existsSvc)

	ctrl := newController(controllerOptions{
		Options:           opts,
		ComponentRegistry: registry,
		ModuleRegistry:    newModuleRegistry(),
	})
	require.NoError(t, ctrl.LoadSource(f, nil))
	go ctrl.Run(ctx)

	require.NoError(t, componentBuilt.Wait(5*time.Second), "Component should have been built")
}

func makeEmptyFile(t *testing.T) *Source {
	t.Helper()

	f, err := ParseSource(t.Name(), nil)
	require.NoError(t, err)
	require.NotNil(t, f)

	return f
}
