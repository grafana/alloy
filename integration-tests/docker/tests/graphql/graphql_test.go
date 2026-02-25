//go:build alloyintegrationtests

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const graphqlEndpoint = "http://localhost:12365/graphql"

func TestGraphQLAlloyInfo(t *testing.T) {
	body := postGraphQL(t, `{"query": "{ alloy { version isReady } }"}`)

	var result struct {
		Data struct {
			Alloy struct {
				Version string `json:"version"`
				IsReady bool   `json:"isReady"`
			} `json:"alloy"`
		} `json:"data"`
		Errors []graphQLError `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(body, &result))

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.Data.Alloy.Version)
	assert.True(t, result.Data.Alloy.IsReady)
}

func TestGraphQLComponents(t *testing.T) {
	expectedNames := map[string]bool{
		"prometheus.exporter.self": false,
		"prometheus.scrape":        false,
		"prometheus.remote_write":  false,
	}

	body := postGraphQL(t, `{"query": "{ components { id name health { message } } }"}`)

	var result struct {
		Data struct {
			Components []graphQLComponent `json:"components"`
		} `json:"data"`
		Errors []graphQLError `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(body, &result))

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
}

func TestGraphQLComponentByID(t *testing.T) {
	body := postGraphQL(t, `{"query": "{ component(id: \"prometheus.exporter.self.default\") { id name health { message } } }"}`)

	var result struct {
		Data struct {
			Component *graphQLComponent `json:"component"`
		} `json:"data"`
		Errors []graphQLError `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(body, &result))

	assert.Empty(t, result.Errors)
	require.NotNil(t, result.Data.Component)
	assert.Equal(t, "prometheus.exporter.self.default", result.Data.Component.ID)
	assert.Equal(t, "prometheus.exporter.self", result.Data.Component.Name)
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

func postGraphQL(t *testing.T, query string) []byte {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(graphqlEndpoint, "application/json", bytes.NewBufferString(query))
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return body
}
