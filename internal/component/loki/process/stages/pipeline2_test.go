package stages

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestNewPipelineStopsOnFailure(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	cfgs := loadConfig(`
	stage.regex {
		expression = "[unclosed"
	}
	stage.match {
		selector = "{app=\"x\"}"
		action   = "keep"

		stage.multiline {
			firstline     = "^START"
			max_wait_time = "10ms"
		}
	}
	stage.multiline {
		firstline     = "^START"
		max_wait_time = "10ms"
	}
	`)

	next := func(_ context.Context, _ []Entry) error { return nil }
	_, err := newPipeline(logging.NewSlogNop(), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable, cfgs, next)
	require.Error(t, err)
}
