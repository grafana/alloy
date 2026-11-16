package http

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRetrieveEnvironmentVariables_RedactsProxies(t *testing.T) {
	env := map[string]string{
		"HTTP_PROXY":  "http://user:secret@proxy.example.com:3128",
		"https_proxy": "http://user:secret@proxy.example.com:3128",
		"NO_PROXY":    "localhost",
	}
	getenv := func(key string) string { return env[key] }

	values := retrieveEnvironmentVariables(getenv)

	require.Contains(t, values, "HTTP_PROXY=<redacted>")
	require.Contains(t, values, "https_proxy=<redacted>")
	require.Contains(t, values, "HTTPS_PROXY=")
	require.Contains(t, values, "NO_PROXY=localhost")
	for _, v := range values {
		require.NotContains(t, v, "secret")
	}
}
