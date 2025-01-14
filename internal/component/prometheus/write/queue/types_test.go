package queue

import (
	common "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestBasicAuthAndTLSBothSetError(t *testing.T) {
	args := defaultArgs()
	args.Endpoints = make([]EndpointConfig, 1)
	args.Endpoints[0] = defaultEndpointConfig()
	args.Endpoints[0].Name = "test"
	args.Endpoints[0].TLSConfig = &common.TLSConfig{}
	args.Endpoints[0].BasicAuth = &BasicAuth{}
	err := args.Validate()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "cannot have both BasicAuth and TLSConfig"))
}
