package docker

import (
	"net/url"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	types "github.com/grafana/alloy/internal/component/common/config"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func TestComponent(t *testing.T) {
	// Use host that works on all platforms (including Windows).
	var cfg = `
		host       = "tcp://127.0.0.1:9375"
		targets    = []
		forward_to = []
	`

	var args Arguments
	err := syntax.Unmarshal([]byte(cfg), &args)
	require.NoError(t, err)

	ctrl, err := componenttest.NewControllerFromID(util.TestLogger(t), "loki.source.docker")
	require.NoError(t, err)

	go func() {
		err := ctrl.Run(t.Context(), args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Minute))
}

func TestComponentDuplicateTargets(t *testing.T) {
	// Use host that works on all platforms (including Windows).
	var cfg = `
		host       = "tcp://127.0.0.1:9376"
		targets    = [
			{__meta_docker_container_id = "foo", __meta_docker_port_private = "8080"},
			{__meta_docker_container_id = "foo", __meta_docker_port_private = "8081"},
		]
		forward_to = []
	`

	var args Arguments
	err := syntax.Unmarshal([]byte(cfg), &args)
	require.NoError(t, err)

	ctrl, err := componenttest.NewControllerFromID(util.TestLogger(t), "loki.source.docker")
	require.NoError(t, err)

	go func() {
		err := ctrl.Run(t.Context(), args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Minute))

	cmp, err := New(component.Options{
		ID:         "loki.source.docker.test",
		Logger:     logging.NewSlogNop(),
		Registerer: prometheus.NewRegistry(),
		DataPath:   t.TempDir(),
	}, args)
	require.NoError(t, err)

	require.Equal(t, 1, cmp.scheduler.Len())
	for s := range cmp.scheduler.Sources() {
		ss := s.(*tailer)
		require.Equal(t, "{__meta_docker_container_id=\"foo\", __meta_docker_port_private=\"8080\"}", ss.labelsStr)
	}

	var newCfg = `
		host       = "tcp://127.0.0.1:9376"
		targets    = [
			{__meta_docker_container_id = "foo", __meta_docker_port_private = "8081"},
			{__meta_docker_container_id = "foo", __meta_docker_port_private = "8080"},
		]
		forward_to = []
	`
	err = syntax.Unmarshal([]byte(newCfg), &args)
	require.NoError(t, err)
	cmp.Update(args)
	// Although the order of the targets changed, the filtered target stays the same.
	require.Equal(t, 1, cmp.scheduler.Len())
	for s := range cmp.scheduler.Sources() {
		ss := s.(*tailer)
		require.Equal(t, "{__meta_docker_container_id=\"foo\", __meta_docker_port_private=\"8080\"}", ss.labelsStr)
	}
}

func TestRequiresReset(t *testing.T) {
	tests := []struct {
		desc     string
		a, b     Arguments
		expected bool
	}{
		{
			desc:     "both zero",
			a:        Arguments{},
			b:        Arguments{},
			expected: false,
		},
		{
			desc:     "two defaults",
			a:        GetDefaultArguments(),
			b:        GetDefaultArguments(),
			expected: false,
		},
		{
			desc: "all fields set and identical",
			a: Arguments{
				Host:             "tcp://127.0.0.1:9375",
				RelabelRules:     alloy_relabel.Rules{{Action: alloy_relabel.Drop, TargetLabel: "foo", Regex: mustNewRegexp(t, "f(.*)")}},
				HTTPClientConfig: &types.HTTPClientConfig{BasicAuth: &types.BasicAuth{Username: "user", Password: "password"}},
				RefreshInterval:  time.Minute,
			},
			b: Arguments{
				Host:             "tcp://127.0.0.1:9375",
				RelabelRules:     alloy_relabel.Rules{{Action: alloy_relabel.Drop, TargetLabel: "foo", Regex: mustNewRegexp(t, "f(.*)")}},
				HTTPClientConfig: &types.HTTPClientConfig{BasicAuth: &types.BasicAuth{Username: "user", Password: "password"}},
				RefreshInterval:  time.Minute,
			},
			expected: false,
		},
		{
			desc:     "different relabel rule",
			a:        Arguments{RelabelRules: alloy_relabel.Rules{{Action: alloy_relabel.Drop, Regex: mustNewRegexp(t, "f(.*)")}}},
			b:        Arguments{RelabelRules: alloy_relabel.Rules{{Action: alloy_relabel.Drop, Regex: mustNewRegexp(t, ".*")}}},
			expected: true,
		},
		{
			desc:     "different host",
			a:        Arguments{Host: "tcp://127.0.0.1:9375"},
			b:        Arguments{Host: "tcp://127.0.0.1:9376"},
			expected: true,
		},
		{
			desc:     "different refresh interval",
			a:        Arguments{RefreshInterval: time.Minute},
			b:        Arguments{RefreshInterval: 2 * time.Minute},
			expected: true,
		},
		{
			desc:     "different targets",
			a:        Arguments{Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{"__meta_docker_container_id": "foo"})}},
			b:        Arguments{Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{"__meta_docker_container_id": "bar"})}},
			expected: false,
		},
		{
			desc:     "different labels",
			a:        Arguments{Labels: map[string]string{"env": "dev"}},
			b:        Arguments{Labels: map[string]string{"env": "prod"}},
			expected: false,
		},

		{
			desc:     "unset and default http client config",
			a:        Arguments{HTTPClientConfig: nil},
			b:        Arguments{HTTPClientConfig: types.CloneDefaultHTTPClientConfig()},
			expected: true,
		},
		{
			desc:     "different bearer token",
			a:        Arguments{HTTPClientConfig: &types.HTTPClientConfig{BearerToken: "token"}},
			b:        Arguments{HTTPClientConfig: &types.HTTPClientConfig{BearerToken: "other-token"}},
			expected: true,
		},
		{
			desc:     "unset and set basic auth",
			a:        Arguments{HTTPClientConfig: &types.HTTPClientConfig{}},
			b:        Arguments{HTTPClientConfig: &types.HTTPClientConfig{BasicAuth: &types.BasicAuth{Username: "user"}}},
			expected: true,
		},
		{
			desc:     "reordered oauth2 scopes",
			a:        Arguments{HTTPClientConfig: &types.HTTPClientConfig{OAuth2: &types.OAuth2Config{Scopes: []string{"scope1", "scope2"}}}},
			b:        Arguments{HTTPClientConfig: &types.HTTPClientConfig{OAuth2: &types.OAuth2Config{Scopes: []string{"scope2", "scope1"}}}},
			expected: true,
		},
		{
			desc:     "different oauth2 endpoint params",
			a:        Arguments{HTTPClientConfig: &types.HTTPClientConfig{OAuth2: &types.OAuth2Config{EndpointParams: map[string]string{"param1": "value1"}}}},
			b:        Arguments{HTTPClientConfig: &types.HTTPClientConfig{OAuth2: &types.OAuth2Config{EndpointParams: map[string]string{"param1": "value2"}}}},
			expected: true,
		},
		{
			desc:     "different tls config",
			a:        Arguments{HTTPClientConfig: &types.HTTPClientConfig{TLSConfig: types.TLSConfig{CAFile: "/path/to/file.ca"}}},
			b:        Arguments{HTTPClientConfig: &types.HTTPClientConfig{TLSConfig: types.TLSConfig{CAFile: "/other/path/to/file.ca"}}},
			expected: true,
		},
		{
			desc:     "same proxy url",
			a:        Arguments{HTTPClientConfig: &types.HTTPClientConfig{ProxyConfig: &types.ProxyConfig{ProxyURL: mustParseURL(t, "http://0.0.0.0:11111")}}},
			b:        Arguments{HTTPClientConfig: &types.HTTPClientConfig{ProxyConfig: &types.ProxyConfig{ProxyURL: mustParseURL(t, "http://0.0.0.0:11111")}}},
			expected: false,
		},
		{
			desc:     "different proxy url",
			a:        Arguments{HTTPClientConfig: &types.HTTPClientConfig{ProxyConfig: &types.ProxyConfig{ProxyURL: mustParseURL(t, "http://0.0.0.0:11111")}}},
			b:        Arguments{HTTPClientConfig: &types.HTTPClientConfig{ProxyConfig: &types.ProxyConfig{ProxyURL: mustParseURL(t, "http://0.0.0.0:22222")}}},
			expected: true,
		},
		{
			desc:     "different http header value",
			a:        Arguments{HTTPClientConfig: &types.HTTPClientConfig{HTTPHeaders: &types.Headers{Headers: map[string][]alloytypes.Secret{"X-Test": {"value"}}}}},
			b:        Arguments{HTTPClientConfig: &types.HTTPClientConfig{HTTPHeaders: &types.Headers{Headers: map[string][]alloytypes.Secret{"X-Test": {"other-value"}}}}},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.expected, requiresReset(tc.a, tc.b))
			require.Equal(t, tc.expected, requiresReset(tc.b, tc.a))
		})
	}
}

func mustParseURL(t *testing.T, s string) types.URL {
	t.Helper()

	u, err := url.Parse(s)
	require.NoError(t, err)
	return types.URL{URL: u}
}

func mustNewRegexp(t *testing.T, s string) alloy_relabel.Regexp {
	t.Helper()

	var re alloy_relabel.Regexp
	require.NoError(t, re.UnmarshalText([]byte(s)))
	return re
}
