package dockerswarm

import (
	"testing"
	"time"

	promConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	promdiscovery "github.com/prometheus/prometheus/discovery/moby"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func TestAlloyUnmarshal(t *testing.T) {
	alloyCfg := `
		host = "unix:///var/run/docker.sock"
		role = "nodes"
		port = 81
		filter {
			name = "n1"
			values = ["v11", "v12"]
		}
		filter {
			name = "n2"
			values = ["v21"]
		}
		refresh_interval = "12s"
		basic_auth {
			username = "username"
			password = "pass"
		}
		http_headers = {
			"foo" = ["foobar"],
		}
		`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)
	require.ElementsMatch(t, []Filter{{"n1", []string{"v11", "v12"}}, {"n2", []string{"v21"}}}, args.Filters)
	assert.Equal(t, "unix:///var/run/docker.sock", args.Host)
	assert.Equal(t, "nodes", args.Role)
	assert.Equal(t, 81, args.Port)
	assert.Equal(t, 12*time.Second, args.RefreshInterval)
	assert.Equal(t, "username", args.HTTPClientConfig.BasicAuth.Username)
	assert.Equal(t, alloytypes.Secret("pass"), args.HTTPClientConfig.BasicAuth.Password)

	header := args.HTTPClientConfig.HTTPHeaders.Headers["foo"][0]
	require.Equal(t, "foobar", string(header))
}

func TestConvert(t *testing.T) {
	alloyArgs := Arguments{
		Host:            "host",
		Role:            "nodes",
		Port:            81,
		Filters:         []Filter{{"n1", []string{"v11", "v12"}}, {"n2", []string{"v21"}}},
		RefreshInterval: time.Minute,
		HTTPClientConfig: config.HTTPClientConfig{
			BasicAuth: &config.BasicAuth{
				Username: "username",
				Password: "pass",
			},
		},
	}

	promArgs := alloyArgs.Convert().(*promdiscovery.DockerSwarmSDConfig)
	assert.Equal(t, 2, len(promArgs.Filters))
	assert.Equal(t, "n1", promArgs.Filters[0].Name)
	require.ElementsMatch(t, []string{"v11", "v12"}, promArgs.Filters[0].Values)
	assert.Equal(t, "n2", promArgs.Filters[1].Name)
	require.ElementsMatch(t, []string{"v21"}, promArgs.Filters[1].Values)
	assert.Equal(t, "host", promArgs.Host)
	assert.Equal(t, "nodes", promArgs.Role)
	assert.Equal(t, 81, promArgs.Port)
	assert.Equal(t, model.Duration(time.Minute), promArgs.RefreshInterval)
	assert.Equal(t, "username", promArgs.HTTPClientConfig.BasicAuth.Username)
	assert.Equal(t, promConfig.Secret("pass"), promArgs.HTTPClientConfig.BasicAuth.Password)
}

func TestValidateRole(t *testing.T) {
	alloyArgs := Arguments{
		Host:            "host",
		Role:            "nodes",
		RefreshInterval: time.Second,
	}
	err := alloyArgs.Validate()
	require.NoError(t, err)

	alloyArgs.Role = "services"
	err = alloyArgs.Validate()
	require.NoError(t, err)

	alloyArgs.Role = "tasks"
	err = alloyArgs.Validate()
	require.NoError(t, err)

	alloyArgs.Role = "wrong"
	err = alloyArgs.Validate()
	assert.Error(t, err, "invalid role wrong, expected tasks, services, or nodes")
}

func TestValidateUrl(t *testing.T) {
	alloyArgs := Arguments{
		Host:            "::",
		Role:            "nodes",
		RefreshInterval: time.Second,
	}
	err := alloyArgs.Validate()
	assert.Error(t, err, "parse \"::\": missing protocol scheme")
}
