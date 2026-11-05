package receiver

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

func TestServerArgumentsCORSAllowedHeaders(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config string
		want   []string
	}{
		{name: "omitted", want: []string{"content-type", "content-encoding", "idempotency-key", "traceparent", "x-api-key", "x-faro-session-id", "x-scope-orgid"}},
		{name: "empty", config: `cors_allowed_headers = []`, want: []string{}},
		{name: "configured", config: `cors_allowed_headers = ["X-Custom-Header"]`, want: []string{"X-Custom-Header"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var args Arguments
			require.NoError(t, syntax.Unmarshal([]byte("server {\n"+tc.config+"\n}\noutput {}"), &args))
			require.Equal(t, tc.want, args.Server.CORSAllowedHeaders)
		})
	}
}
