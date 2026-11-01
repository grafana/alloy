package receiver

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

func TestServerArgumentsCORSExtraAllowedHeaders(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config string
		want   []string
	}{
		{name: "omitted"},
		{name: "empty", config: `cors_extra_allowed_headers = []`, want: []string{}},
		{name: "configured", config: `cors_extra_allowed_headers = ["X-Custom-Header"]`, want: []string{"X-Custom-Header"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var args Arguments
			require.NoError(t, syntax.Unmarshal([]byte("server {\n"+tc.config+"\n}\noutput {}"), &args))
			require.Equal(t, tc.want, args.Server.CORSExtraAllowedHeaders)
		})
	}
}
