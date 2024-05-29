package auth_test

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func TestAuth(t *testing.T) {
	var (
		waitCreated = util.NewWaitTrigger()
		onCreated   = func() {
			waitCreated.Trigger()
		}
	)

	// Create and start our Alloy component. We then wait for it to export a
	// consumer that we can send data to.
	te := newTestEnvironment(t, onCreated)
	te.Start(fakeAuthArgs{})

	require.NoError(t, waitCreated.Wait(time.Second), "extension never created")
}

type testEnvironment struct {
	t *testing.T

	Controller *componenttest.Controller
}

func newTestEnvironment(t *testing.T, onCreated func()) *testEnvironment {
	t.Helper()

	reg := component.Registration{
		Name:    "testcomponent",
		Args:    fakeAuthArgs{},
		Exports: otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			factory := otelextension.NewFactory(
				otelcomponent.MustNewType("testcomponent"),
				func() otelcomponent.Config { return nil },
				func(
					_ context.Context,
					_ otelextension.CreateSettings,
					_ otelcomponent.Config,
				) (otelcomponent.Component, error) {

					onCreated()
					return nil, nil
				}, otelcomponent.StabilityLevelUndefined,
			)

			return auth.New(opts, factory, args.(auth.Arguments))
		},
	}

	return &testEnvironment{
		t:          t,
		Controller: componenttest.NewControllerFromReg(util.TestLogger(t), reg),
	}
}

func (te *testEnvironment) Start(args component.Arguments) {
	go func() {
		ctx := componenttest.TestContext(te.t)
		err := te.Controller.Run(ctx, args)
		require.NoError(te.t, err, "failed to run component")
	}()
}

type fakeAuthArgs struct {
}

var _ auth.Arguments = fakeAuthArgs{}

func (fa fakeAuthArgs) Convert() (otelcomponent.Config, error) {
	return &struct{}{}, nil
}

func (fa fakeAuthArgs) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

func (fa fakeAuthArgs) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func TestNormalizeType(t *testing.T) {
	type tc struct {
		input    string
		expected string
	}
	testcases := []tc{
		{"foo", "foo"},
		{"foo1", "foo1"},
		{"fooBar", "fooBar"},

		{"foo.bar", "foo_bar"},
		{"foo/bar", "foo_bar"},
		{"foo.bar/baz", "foo_bar_baz"},
		{"foo/bar_baz", "foo_bar_baz"},

		{"thisStringsConstructedSoThatItsLengthIsSetToBeAtSixtyThreeChars", "thisStringsConstructedSoThatItsLengthIsSetToBeAtSixtyThreeChars"},
		{"whileThisOneHeresConstructedSoThatItsSizeIsEqualToSixtyFourChars", "whileThisOneHeresConstructedSoThatItsSiz2d7fa5d2"},
	}

	// https://github.com/open-telemetry/opentelemetry-collector/blob/e09b25f7d1b4090a9b7b73ef7d3c514592331554/component/config.go#L127-L131
	var typeRegexp = regexp.MustCompile(`^[a-zA-Z][0-9a-zA-Z_]{0,62}$`)

	for _, tt := range testcases {
		actual := auth.NormalizeType(tt.input)
		require.Equal(t, tt.expected, actual)
		require.True(t, typeRegexp.MatchString(actual))
	}
}

func (fe fakeAuthArgs) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	var dma otelcolCfg.DebugMetricsArguments
	dma.SetToDefault()
	return dma
}
