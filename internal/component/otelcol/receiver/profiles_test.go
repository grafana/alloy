package receiver_test

import (
	"context"
	"errors"
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/pipeline"
	otelreceiver "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"
)

func TestReceiverProfilesErrors(t *testing.T) {
	createErr := errors.New("cannot create profiles receiver")
	for _, tc := range []struct {
		name    string
		factory otelreceiver.Factory
		want    error
	}{
		{name: "factory without experimental signals", factory: otelreceiver.NewFactory(otelcomponent.MustNewType("testcomponent"), func() otelcomponent.Config { return &struct{}{} }), want: pipeline.ErrSignalNotSupported},
		{name: "factory without profiles", factory: xreceiver.NewFactory(otelcomponent.MustNewType("testcomponent"), func() otelcomponent.Config { return &struct{}{} }), want: pipeline.ErrSignalNotSupported},
		{name: "creation failure", factory: xreceiver.NewFactory(otelcomponent.MustNewType("testcomponent"), func() otelcomponent.Config { return &struct{}{} },
			xreceiver.WithProfiles(func(context.Context, otelreceiver.Settings, otelcomponent.Config, xconsumer.Profiles) (xreceiver.Profiles, error) {
				return nil, createErr
			}, otelcomponent.StabilityLevelDevelopment)), want: createErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := componenttest.NewControllerFromReg(util.TestLogger(t), component.Registration{
				Name: "testcomponent", Args: fakeReceiverArgs{},
				Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
					return receiver.New(opts, tc.factory, args.(receiver.Arguments))
				},
			})
			err := ctrl.Run(t.Context(), fakeReceiverArgs{Output: &otelcol.ConsumerArguments{Profiles: []otelcol.Consumer{&fakeconsumer.Consumer{}}}}, func(opts component.Options) component.Options {
				opts.MinStability = featuregate.StabilityExperimental
				return opts
			})
			require.ErrorIs(t, err, tc.want)
		})
	}
}
