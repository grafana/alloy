package runtime_test

import (
	"errors"
	"testing"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/stretchr/testify/require"

	// Install Components
	_ "github.com/grafana/alloy/internal/component/all"
)

func TestTypeCheck(t *testing.T) {
	testFile := `
		
prometheus.exporter.unix "test" {
}

prometheus.scrape "demo" {
	targets    = prometheus.exporter.unix.test.targets
	forward_to = [prometheus.remote_write.rw.receiver]
}

prometheus.remote_write "rw" {
  endpoint {
  }
}
	`

	source, err := runtime.ParseSource("test", []byte(testFile))
	require.NoError(t, err)

	err = runtime.TypeCheck(source, runtime.Options{
		ControllerID:         "1",
		DataPath:             t.TempDir(),
		MinStability:         featuregate.StabilityExperimental,
		EnableCommunityComps: false,
	})

	t.Log("Typecheck errors: ", err)

	var diags diag.Diagnostics
	if errors.As(err, &diags) {
		for _, diag := range diags {
			t.Log(diag)
		}
	}
}
