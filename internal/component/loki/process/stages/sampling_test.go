package stages

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
)

// TestSamplingStage can't use runPipelineTest since sampling is probabilistic:
// which entries survive isn't deterministic, only how many roughly should.
func TestSamplingStage(t *testing.T) {
	cfg := loadConfig(`
	stage.sampling {
	  rate = 0.5
	}
	`)

	newEntries := func() []Entry {
		entries := make([]Entry, 100)
		for i := range entries {
			entries[i] = newEntry(map[string]any{}, model.LabelSet{}, testMatchLogLineApp1, time.Now())
		}
		return entries
	}

	// sampling rate = 0.5, entries len = 100, the theoretical sample size is
	// 50. Assert 30 < n < 70 to avoid flakes while still catching gross bugs.
	assertSampleSize := func(t *testing.T, n int) {
		t.Helper()
		assert.GreaterOrEqual(t, n, 30)
		assert.LessOrEqual(t, n, 70)
	}

	t.Run("Stage", func(t *testing.T) {
		p, err := NewPipeline(logging.NewSlogNop(), cfg, prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable)
		require.NoError(t, err)
		defer p.Cleanup()
		defer p.Stop()

		out := processEntries(p, newEntries()...)
		assertSampleSize(t, len(out))
	})

	t.Run("New Stage", func(t *testing.T) {
		var collected []Entry
		next := func(_ context.Context, entries []Entry) error {
			collected = append(collected, entries...)
			return nil
		}

		p, err := NewPipeline2(logging.NewSlogNop(), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable, cfg, next)
		require.NoError(t, err)
		defer p.Stop()

		require.NoError(t, p.process(context.Background(), newEntries()))
		assertSampleSize(t, len(collected))
	})
}

func TestValidateSamplingConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *SamplingConfig
		wantErr error
	}{
		{
			name: "Invalid rate",
			config: &SamplingConfig{
				SamplingRate: 12,
			},
			wantErr: fmt.Errorf(errSamplingStageInvalidRate, 12.0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.Validate(); ((err != nil) && (err.Error() != tt.wantErr.Error())) || (err == nil && tt.wantErr != nil) {
				t.Errorf("validateDropConfig() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
