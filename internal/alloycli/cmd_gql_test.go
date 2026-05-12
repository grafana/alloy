package alloycli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommandIncludesGQLCommand(t *testing.T) {
	cmd := Command()

	var foundGQL, foundREPL bool
	for _, subcommand := range cmd.Commands() {
		switch subcommand.Name() {
		case "gql":
			foundGQL = true
		case "repl":
			foundREPL = true
		}
	}

	require.True(t, foundGQL)
	require.False(t, foundREPL)
}

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
	output := captureStdout(t, func() {
		require.NoError(t, g.Run("alloy { isReady }"))
	})

	require.Equal(t, "query { alloy { isReady } }", receivedQuery)
	require.JSONEq(t, `{"alloy":{"isReady":true}}`, output)
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
	output := captureStdout(t, func() {
		require.NoError(t, g.Run("{ alloy { version } }"))
	})

	require.Equal(t, "{ alloy { version } }", receivedQuery)
	require.JSONEq(t, `{"alloy":{"version":"dev"}}`, output)
}

func TestAlloyGqlRunReturnsErrorForGraphQLErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"errors":[{"message":"bad query"}]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	g := &alloyGql{httpAddr: server.URL}
	var runErr error
	output := captureStdout(t, func() {
		runErr = g.Run("{ alloy {")
	})

	require.Error(t, runErr)
	require.ErrorContains(t, runErr, "GraphQL response contains errors")
	require.Contains(t, output, "bad query")
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	os.Stdout = writer
	fn()
	require.NoError(t, writer.Close())

	out, err := io.ReadAll(reader)
	require.NoError(t, err)
	return string(out)
}
