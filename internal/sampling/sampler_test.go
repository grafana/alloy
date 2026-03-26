package sampling

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateRate(t *testing.T) {
	tests := []struct {
		name    string
		rate    float64
		wantErr bool
	}{
		{"valid zero", 0, false},
		{"valid one", 1, false},
		{"valid half", 0.5, false},
		{"invalid negative", -0.1, true},
		{"invalid over one", 1.1, true},
		{"invalid large", 2.0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRate(tt.rate)
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "rate must be between")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSampler_RateZeroAlwaysFalse(t *testing.T) {
	s := NewSampler(0)
	for i := 0; i < 100; i++ {
		require.False(t, s.ShouldSample(), "rate 0 should never sample")
	}
}

func TestSampler_RateOneAlwaysTrue(t *testing.T) {
	s := NewSampler(1)
	for i := 0; i < 100; i++ {
		require.True(t, s.ShouldSample(), "rate 1 should always sample")
	}
}

func TestSampler_RateHalfApproximatelyHalf(t *testing.T) {
	s := NewSampler(0.5)
	const n = 1000
	var trues int
	for i := 0; i < n; i++ {
		if s.ShouldSample() {
			trues++
		}
	}
	// Allow 35-65% to avoid flakiness
	require.GreaterOrEqual(t, trues, int(0.35*float64(n)), "expected at least ~35%% sampled")
	require.LessOrEqual(t, trues, int(0.65*float64(n)), "expected at most ~65%% sampled")
}

func TestSampler_Update(t *testing.T) {
	s := NewSampler(0.5)
	// After Update(0), all false
	s.Update(0)
	for i := 0; i < 50; i++ {
		require.False(t, s.ShouldSample())
	}
	// After Update(1), all true
	s.Update(1)
	for i := 0; i < 50; i++ {
		require.True(t, s.ShouldSample())
	}
}

func TestSampler_OutOfRangeRateDeterministic(t *testing.T) {
	// Out-of-range rate is not clamped; callers must ValidateRate first.
	// Our ShouldSample guards still yield deterministic behavior (no randomness).
	sNeg := NewSampler(-1)
	require.False(t, sNeg.ShouldSample(), "negative rate → never sample")
	sOver := NewSampler(2)
	require.True(t, sOver.ShouldSample(), "rate > 1 → always sample")
}
