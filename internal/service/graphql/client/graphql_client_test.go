package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecuteReturnsResponseBodyForNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"message":"bad query"}]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	c := NewGraphQLClient(server.URL)
	_, err := c.Execute("{ alloy {")

	require.Error(t, err)
	require.ErrorContains(t, err, "400 Bad Request")
	require.ErrorContains(t, err, "bad query")
}

func TestExecuteReturnsGraphQLErrorsInResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"errors":[{"message":"bad query"}]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	c := NewGraphQLClient(server.URL)
	response, err := c.Execute("{ alloy {")

	require.NoError(t, err)
	require.Len(t, response.Errors, 1)
	require.Contains(t, string(response.Raw), "bad query")
}
