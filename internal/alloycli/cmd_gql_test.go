package alloycli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAlloyGqlRunExecutesQuery(t *testing.T) {
	var receivedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req struct {
			Query string `json:"query"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		receivedQuery = req.Query

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"data":{"alloy":{"isReady":true}}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	g := &alloyGql{httpAddr: server.URL}
	var output bytes.Buffer
	require.NoError(t, g.Run("alloy { isReady }", &output))

	require.Equal(t, "query { alloy { isReady } }", receivedQuery)
	require.JSONEq(t, `{"alloy":{"isReady":true}}`, output.String())
}

func TestAlloyGqlRunLeavesCompleteQueryUnchanged(t *testing.T) {
	var receivedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query string `json:"query"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		receivedQuery = req.Query

		_, err := w.Write([]byte(`{"data":{"alloy":{"version":"dev"}}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	g := &alloyGql{httpAddr: server.URL}
	var output bytes.Buffer
	require.NoError(t, g.Run("{ alloy { version } }", &output))

	require.Equal(t, "{ alloy { version } }", receivedQuery)
	require.JSONEq(t, `{"alloy":{"version":"dev"}}`, output.String())
}

func TestAlloyGqlRunReturnsErrorForGraphQLErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"errors":[{"message":"bad query"}]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	g := &alloyGql{httpAddr: server.URL}
	var output bytes.Buffer
	runErr := g.Run("{ alloy {", &output)

	require.Error(t, runErr)
	require.ErrorContains(t, runErr, "GraphQL response contains errors")
	require.Contains(t, output.String(), "bad query")
}
