package graphql

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

func TestGraphQL(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-graphql",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:   ns.Name(),
		Release:     "alloy-test-graphql",
		ConfigPath:  "./config/config.alloy",
		ValuesPath:  "./config/alloy-values.yaml",
		PortForward: true,
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, alloy},
	})

	endpoint := alloy.Endpoint("/graphql")

	t.Run("AlloyInfo", func(t *testing.T) {
		var result struct {
			Data struct {
				Alloy struct {
					Version string `json:"version"`
					IsReady bool   `json:"isReady"`
				} `json:"alloy"`
			} `json:"data"`
			Errors []graphQLError `json:"errors"`
		}
		postGraphQL(t, endpoint, `{"query": "{ alloy { version isReady } }"}`, &result)

		assert.Empty(t, result.Errors)
		assert.NotEmpty(t, result.Data.Alloy.Version)
		assert.True(t, result.Data.Alloy.IsReady)
	})

	t.Run("Components", func(t *testing.T) {
		expectedNames := map[string]bool{
			"prometheus.exporter.self": false,
			"prometheus.scrape":        false,
			"prometheus.relabel":       false,
		}

		var result struct {
			Data struct {
				Components []graphQLComponent `json:"components"`
			} `json:"data"`
			Errors []graphQLError `json:"errors"`
		}
		postGraphQL(t, endpoint, `{"query": "{ components { id name health { message } } }"}`, &result)

		assert.Empty(t, result.Errors)
		require.Len(t, result.Data.Components, len(expectedNames))

		for _, comp := range result.Data.Components {
			assert.NotEmpty(t, comp.ID)
			if _, ok := expectedNames[comp.Name]; assert.True(t, ok, "unexpected component name: %s", comp.Name) {
				expectedNames[comp.Name] = true
			}
		}
		for name, found := range expectedNames {
			assert.True(t, found, "component %s not found", name)
		}
	})

	t.Run("ComponentByID", func(t *testing.T) {
		var result struct {
			Data struct {
				Component *graphQLComponent `json:"component"`
			} `json:"data"`
			Errors []graphQLError `json:"errors"`
		}
		postGraphQL(t, endpoint, `{"query": "{ component(id: \"prometheus.exporter.self.default\") { id name health { message } } }"}`, &result)

		assert.Empty(t, result.Errors)
		require.NotNil(t, result.Data.Component)
		assert.Equal(t, "prometheus.exporter.self.default", result.Data.Component.ID)
		assert.Equal(t, "prometheus.exporter.self", result.Data.Component.Name)
	})
}

type graphQLComponent struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Health struct {
		Message string `json:"message"`
	} `json:"health"`
}

type graphQLError struct {
	Message string `json:"message"`
}

func postGraphQL(t *testing.T, endpoint, query string, out any) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		resp, err := client.Post(endpoint, "application/json", strings.NewReader(query))
		if !assert.NoError(c, err) {
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(c, err)
		require.Equal(c, http.StatusOK, resp.StatusCode, "body: %s", body)
		require.NoError(c, json.Unmarshal(body, out))
	}, time.Minute, 500*time.Millisecond)
}
