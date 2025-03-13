package auth_test

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pipeline"
)

func TestAuth(t *testing.T) {
	var (
		waitCreated = util.NewWaitTrigger()
		onCreated   = func() {
			waitCreated.Trigger()
		}
	)

	fakeAuthArgs := &fakeAuthArgs{}
	fakeAuthArgs.On("AuthFeatures").Return(auth.ClientAndServerAuthSupported)

	// Create and start our Alloy component. We then wait for it to export a
	// consumer that we can send data to.
	te := newTestEnvironment(t, onCreated)
	te.Start(fakeAuthArgs)

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
		Exports: auth.Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			factory := otelextension.NewFactory(
				otelcomponent.MustNewType("testcomponent"),
				func() otelcomponent.Config { return nil },
				func(
					_ context.Context,
					_ otelextension.Settings,
					_ otelcomponent.Config,
				) (otelextension.Extension, error) {

					onCreated()
					return fakeOtelComponent{}, nil
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

type fakeOtelComponent struct {
}

func (f fakeOtelComponent) Start(ctx context.Context, host otelcomponent.Host) error {
	return nil
}

func (f fakeOtelComponent) Shutdown(ctx context.Context) error {
	return nil
}

type fakeAuthArgs struct {
	mock.Mock
}

var _ auth.Arguments = &fakeAuthArgs{}

func (fa *fakeAuthArgs) ConvertClient() (otelcomponent.Config, error) {
	return &struct{}{}, nil
}

func (fa *fakeAuthArgs) ConvertServer() (otelcomponent.Config, error) {
	return &struct{}{}, nil
}

func (fa *fakeAuthArgs) AuthFeatures() auth.AuthFeature {
	result := fa.Called()
	return result.Get(0).(auth.AuthFeature)
}

func (fa *fakeAuthArgs) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (fa *fakeAuthArgs) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
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

func (fe *fakeAuthArgs) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	var dma otelcolCfg.DebugMetricsArguments
	dma.SetToDefault()
	return dma
}

// TestValidateAuthFeature tests that it returns if the flag returned by an otel auth
// extension is correctly interpreted.
func TestValidateAuthFeature(t *testing.T) {
	type tc struct {
		input    auth.AuthFeature
		expected bool
	}
	testcases := []tc{
		{auth.ClientAndServerAuthSupported, true},
		{auth.ClientAuthSupported, true},
		{auth.ServerAuthSupported, true},
		{5, false},
	}

	for _, tc := range testcases {
		actual := auth.ValidateAuthFeatures(tc.input)
		require.Equal(t, tc.expected, actual)
	}
}

// TestHasAuthFeature tests that HasAuthFeature correctly determines
// whether the auth extension has an authentication feature, server or client.
func TestHasAuthFeature(t *testing.T) {
	type input struct {
		flag    auth.AuthFeature
		feature auth.AuthFeature
	}
	type tc struct {
		input    input
		expected bool
	}

	testcases := []tc{
		{input{flag: auth.ClientAuthSupported, feature: auth.ClientAuthSupported}, true},
		{input{flag: auth.ServerAuthSupported, feature: auth.ServerAuthSupported}, true},
		{input{flag: auth.ClientAndServerAuthSupported, feature: auth.ClientAuthSupported}, true},
		{input{flag: auth.ClientAndServerAuthSupported, feature: auth.ServerAuthSupported}, true},
		{input{flag: auth.ClientAuthSupported, feature: auth.ServerAuthSupported}, false},
		{input{flag: auth.ServerAuthSupported, feature: auth.ClientAuthSupported}, false},
	}

	for _, tc := range testcases {
		actual := auth.HasAuthFeature(tc.input.flag, tc.input.feature)
		require.Equal(t, tc.expected, actual, "flag:", tc.input.flag, "feature:", tc.input.feature)
	}
}

// TestAuthHandler runs a simple functional test by running the component
// against a mock auth extension. It validates the component starts correctly
// and when the handler requests either a server or client handler the correct output
// is returned
func TestAuthHandler(t *testing.T) {
	var (
		waitCreated = util.NewWaitTrigger()
		onCreated   = func() {
			waitCreated.Trigger()
		}
	)

	// Test case definition
	type input struct {
		support auth.AuthFeature
	}

	type expected struct {
		clientAuthSupported bool
		serverAuthSupported bool
		err                 error
	}
	type tc struct {
		input    input
		expected expected
	}

	// Test cases
	tcs := []tc{
		// Validates
		{
			input: input{support: auth.ClientAuthSupported}, expected: expected{
				clientAuthSupported: true, serverAuthSupported: false, err: auth.ErrNotServerExtension,
			},
		},
		{
			input: input{support: auth.ServerAuthSupported}, expected: expected{
				clientAuthSupported: false, serverAuthSupported: true, err: auth.ErrNotClientExtension,
			},
		},
		{
			input: input{support: auth.ClientAndServerAuthSupported}, expected: expected{
				clientAuthSupported: true, serverAuthSupported: true, err: nil,
			},
		},
	}

	for _, tc := range tcs {
		// Spin up a test component
		te := newTestEnvironment(t, onCreated)

		// Mock the return of AuthFeatures to avoid creating test-specific implementations
		// for each combination of authentication extensions supported by the OpenTelemetry collector.
		fakeAuthArgs := &fakeAuthArgs{}
		fakeAuthArgs.On("AuthFeatures").Return(tc.input.support)

		// Start the test environment and validate it comes up properly.
		te.Start(fakeAuthArgs)
		require.NoError(t, waitCreated.Wait(time.Second), "extension never created")
		require.NoError(t, te.Controller.WaitRunning(time.Second), "extension never started running")
		require.NoError(t, te.Controller.WaitExports(time.Second), "extension never exported anything")

		// Retrieve the exports of the component, make sure it exported a handler.
		export := te.Controller.Exports()
		authExport, ok := export.(auth.Exports)
		require.True(t, ok, "auth component didn't export an auth export type")
		require.NotNil(t, authExport.Handler)

		// Check the state of the handler and verify it is correct.
		clientEh, err := authExport.Handler.GetExtension(auth.Client)
		validateHandler(t, clientEh, tc.expected.clientAuthSupported, err, tc.expected.err)

		serverEh, err := authExport.Handler.GetExtension(auth.Server)
		validateHandler(t, serverEh, tc.expected.serverAuthSupported, err, tc.expected.err)
	}
}

// validateHandler determines what the correct state of the extension handler should be depending
// on the test case state. If the extension supports the authentication requested it should return
// the extension. Otherwise it should return an error saying the extension does not support the requested
// type of authentication.
func validateHandler(t *testing.T, eh *auth.ExtensionHandler, authSupported bool, actualErr error, expectedError error) {
	t.Helper()
	if authSupported {
		require.NoError(t, actualErr)
		require.NotNil(t, eh.Extension)
		require.NotNil(t, eh.ID)
	} else {
		require.NotNil(t, actualErr)
		require.ErrorIs(t, actualErr, expectedError)
	}
}
