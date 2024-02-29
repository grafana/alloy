package apache_http

import (
	"fmt"
	"testing"

	integrations_v2 "github.com/grafana/agent/internal/static/integrations/v2"
	"github.com/stretchr/testify/require"
)

func TestApacheHttp_Identifier(t *testing.T) {
	globals := integrations_v2.Globals{}
	hosts := []string{"localhost", "localhost:8080", "10.0.0.1", "10.0.0.1:8080"}

	for _, host := range hosts {
		cfg := Config{
			ApacheAddr: fmt.Sprintf("http://%s/server-status?auto", host),
		}

		id, err := cfg.Identifier(globals)
		require.NoError(t, err)
		require.Equal(t, id, host)
	}
}
