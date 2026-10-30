package processor_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/pipeline"
	otelprocessor "go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/xprocessor"
)

type profilesProcessor struct{ xconsumer.Profiles }

func (*profilesProcessor) Start(context.Context, otelcomponent.Host) error { return nil }
func (*profilesProcessor) Shutdown(context.Context) error                  { return nil }

func profilesController(t *testing.T, factory otelprocessor.Factory) *componenttest.Controller {
	t.Helper()
	return componenttest.NewControllerFromReg(util.TestLogger(t), component.Registration{
		Name: "testcomponent", Args: fakeProcessorArgs{}, Exports: otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return processor.New(opts, factory, args.(processor.Arguments))
		},
	})
}

func TestProcessorProfilesUpdate(t *testing.T) {
	factory := xprocessor.NewFactory(otelcomponent.MustNewType("testcomponent"), func() otelcomponent.Config { return &struct{}{} },
		xprocessor.WithProfiles(func(_ context.Context, _ otelprocessor.Settings, _ otelcomponent.Config, next xconsumer.Profiles) (xprocessor.Profiles, error) {
			return &profilesProcessor{Profiles: next}, nil
		}, otelcomponent.StabilityLevelDevelopment))
	ctrl := profilesController(t, factory)
	first := make(chan pprofile.Profiles, 1)
	second := make(chan pprofile.Profiles, 1)
	output := func(ch chan pprofile.Profiles) fakeProcessorArgs {
		return fakeProcessorArgs{Output: &otelcol.ConsumerArguments{Profiles: []otelcol.Consumer{&fakeconsumer.Consumer{
			ConsumeProfilesFunc: func(ctx context.Context, pd pprofile.Profiles) error {
				select {
				case ch <- pd:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		}}}}
	}
	ctx := componenttest.TestContext(t)
	go func() {
		require.NoError(t, ctrl.Run(ctx, output(first), func(opts component.Options) component.Options {
			opts.MinStability = featuregate.StabilityExperimental
			return opts
		}))
	}()
	require.NoError(t, ctrl.WaitRunning(time.Second))
	require.NoError(t, ctrl.WaitExports(time.Second))
	input := ctrl.Exports().(otelcol.ConsumerExports).Input
	want := testdata.GenerateProfiles(1)
	consume := func(ch chan pprofile.Profiles) {
		t.Helper()
		require.NoError(t, input.ConsumeProfiles(ctx, want))
		select {
		case got := <-ch:
			require.Equal(t, want, got)
		case <-time.After(time.Second):
			t.Fatal("processor did not forward profiles")
		}
	}
	consume(first)
	require.NoError(t, ctrl.Update(output(second)))
	require.Same(t, input, ctrl.Exports().(otelcol.ConsumerExports).Input)
	consume(second)
	select {
	case <-first:
		t.Fatal("profiles sent to the previous output after update")
	default:
	}
}

func TestProcessorProfilesErrors(t *testing.T) {
	createErr := errors.New("cannot create profiles processor")
	for _, tc := range []struct {
		name      string
		factory   otelprocessor.Factory
		stability featuregate.Stability
		want      error
	}{
		{name: "factory without experimental signals", factory: otelprocessor.NewFactory(otelcomponent.MustNewType("testcomponent"), func() otelcomponent.Config { return &struct{}{} }), stability: featuregate.StabilityExperimental, want: pipeline.ErrSignalNotSupported},
		{name: "factory without profiles", factory: xprocessor.NewFactory(otelcomponent.MustNewType("testcomponent"), func() otelcomponent.Config { return &struct{}{} }), stability: featuregate.StabilityExperimental, want: pipeline.ErrSignalNotSupported},
		{name: "creation failure", factory: xprocessor.NewFactory(otelcomponent.MustNewType("testcomponent"), func() otelcomponent.Config { return &struct{}{} },
			xprocessor.WithProfiles(func(context.Context, otelprocessor.Settings, otelcomponent.Config, xconsumer.Profiles) (xprocessor.Profiles, error) {
				return nil, createErr
			}, otelcomponent.StabilityLevelDevelopment)), stability: featuregate.StabilityExperimental, want: createErr},
		{name: "generally available", stability: featuregate.StabilityGenerallyAvailable},
		{name: "public preview", stability: featuregate.StabilityPublicPreview},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := profilesController(t, tc.factory)
			err := ctrl.Run(t.Context(), fakeProcessorArgs{Output: &otelcol.ConsumerArguments{Profiles: []otelcol.Consumer{&fakeconsumer.Consumer{}}}}, func(opts component.Options) component.Options {
				opts.MinStability = tc.stability
				return opts
			})
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			} else {
				require.ErrorContains(t, err, "--stability.level")
			}
		})
	}
}
