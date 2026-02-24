//go:build alloyintegrationtests

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/grafana/alloy/integration-tests/docker/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const graphqlEndpoint = "http://localhost:12365/graphql"

func TestGraphQLAlloyInfo(t *testing.T) {
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		body := postGraphQL(collect, `{"query": "{ alloy { version isReady } }"}`)
		if body == nil {
			return
		}

		var result struct {
			Data struct {
				Alloy struct {
					Version string `json:"version"`
					IsReady bool   `json:"isReady"`
				} `json:"alloy"`
			} `json:"data"`
			Errors []graphQLError `json:"errors"`
		}
		if !assert.NoError(collect, json.Unmarshal(body, &result)) {
			return
		}

		assert.Empty(collect, result.Errors)
		assert.NotEmpty(collect, result.Data.Alloy.Version)
		assert.True(collect, result.Data.Alloy.IsReady)
	}, common.TestTimeoutEnv(t), common.DefaultRetryInterval)
}

func TestGraphQLComponents(t *testing.T) {
	expectedNames := map[string]bool{
		"prometheus.exporter.self": false,
		"prometheus.scrape":        false,
	}

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		body := postGraphQL(collect, `{"query": "{ components { id name health { message } } }"}`)
		if body == nil {
			return
		}

		var result struct {
			Data struct {
				Components []graphQLComponent `json:"components"`
			} `json:"data"`
			Errors []graphQLError `json:"errors"`
		}
		if !assert.NoError(collect, json.Unmarshal(body, &result)) {
			return
		}

		assert.Empty(collect, result.Errors)
		if !assert.Len(collect, result.Data.Components, len(expectedNames)) {
			return
		}

		for _, comp := range result.Data.Components {
			assert.NotEmpty(collect, comp.ID)
			if _, ok := expectedNames[comp.Name]; assert.True(collect, ok, "unexpected component name: %s", comp.Name) {
				expectedNames[comp.Name] = true
			}
		}
		for name, found := range expectedNames {
			assert.True(collect, found, "component %s not found", name)
		}
	}, common.TestTimeoutEnv(t), common.DefaultRetryInterval)
}

func TestGraphQLComponentByID(t *testing.T) {
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		body := postGraphQL(collect, `{"query": "{ component(id: \"prometheus.exporter.self.default\") { id name health { message } } }"}`)
		if body == nil {
			return
		}

		var result struct {
			Data struct {
				Component *graphQLComponent `json:"component"`
			} `json:"data"`
			Errors []graphQLError `json:"errors"`
		}
		if !assert.NoError(collect, json.Unmarshal(body, &result)) {
			return
		}

		assert.Empty(collect, result.Errors)
		if !assert.NotNil(collect, result.Data.Component) {
			return
		}
		assert.Equal(collect, "prometheus.exporter.self.default", result.Data.Component.ID)
		assert.Equal(collect, "prometheus.exporter.self", result.Data.Component.Name)
	}, common.TestTimeoutEnv(t), common.DefaultRetryInterval)
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

func postGraphQL(collect *assert.CollectT, query string) []byte {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(graphqlEndpoint, "application/json", bytes.NewBufferString(query))
	if !assert.NoError(collect, err) {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if !assert.NoError(collect, err) {
		return nil
	}
	if !assert.Equal(collect, http.StatusOK, resp.StatusCode) {
		return nil
	}
	return body
}
