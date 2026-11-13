//go:build alloyintegrationtests

package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDockerEntrypoint(t *testing.T) {
	response, err := http.Get("http://127.0.0.1:12345/-/ready")
	require.NoError(t, err)
	defer response.Body.Close()

	require.Equal(t, http.StatusOK, response.StatusCode)
}
