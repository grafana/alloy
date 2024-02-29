package serverset

import (
	"testing"
	"time"

	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

var testCases = []struct {
	name       string
	config     string
	assertions func(t *testing.T, args Arguments, err error)
}{
	{
		name: "valid config",
		config: `
			servers = ["one", "two"]
			paths = ["/one/foo", "/two/bar"]
			timeout = "1m"
		`,
		assertions: func(t *testing.T, args Arguments, err error) {
			require.NoError(t, err)
			require.Equal(t, args.Timeout, time.Minute)
			require.Equal(t, []string{"one", "two"}, args.Servers)
			require.Equal(t, []string{"/one/foo", "/two/bar"}, args.Paths)
		},
	},
	{
		name: "default timeout",
		config: `
			servers = ["one", "two"]
			paths = ["/one/foo", "/two/bar"]
		`,
		assertions: func(t *testing.T, args Arguments, err error) {
			require.NoError(t, err)
			require.Equal(t, args.Timeout, 10*time.Second)
		},
	},
	{
		name: "missing servers",
		config: `
			servers = []
			paths = ["/one/foo", "/two/bar"]
		`,
		assertions: func(t *testing.T, args Arguments, err error) {
			require.ErrorContains(t, err, "discovery.serverset config must contain at least one Zookeeper server")
		},
	},
	{
		name: "missing paths",
		config: `
			servers = ["one", "two"]
			paths = null
		`,
		assertions: func(t *testing.T, args Arguments, err error) {
			require.ErrorContains(t, err, "discovery.serverset config must contain at least one path")
		},
	},
	{
		name: "invalid paths",
		config: `
			servers = ["one", "two"]
			paths = ["one/foo", "/two/bar"]
		`,
		assertions: func(t *testing.T, args Arguments, err error) {
			require.ErrorContains(t, err, "discovery.serverset config paths must begin with")
			require.ErrorContains(t, err, "one/foo")
		},
	},
}

func Test(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var args Arguments
			err := river.Unmarshal([]byte(tc.config), &args)
			tc.assertions(t, args, err)
		})
	}
}
