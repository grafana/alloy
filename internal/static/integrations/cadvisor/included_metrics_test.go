//go:build linux

package cadvisor

import (
	"maps"
	"sync"
	"testing"

	"github.com/google/cadvisor/container"
	"github.com/stretchr/testify/require"
)

func TestConfig_GetIncludedMetrics(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config

		// expectExact, when set, is compared against the whole returned set.
		expectExact container.MetricSet
		// expectIncluded and expectExcluded are checked kind by kind.
		expectIncluded []container.MetricKind
		expectExcluded []container.MetricKind
		expectErr      string
	}{
		{
			name: "unset disabled_metrics keeps the built-in default-disabled set",
			cfg:  Config{},
			expectExcluded: []container.MetricKind{
				container.ResctrlMetrics,
				container.ProcessMetrics,
				container.NetworkTcpUsageMetrics,
				container.NetworkUdpUsageMetrics,
			},
			expectIncluded: []container.MetricKind{
				container.CpuUsageMetrics,
				container.MemoryUsageMetrics,
				container.DiskUsageMetrics,
				container.DiskIOMetrics,
			},
		},
		{
			name:        "empty disabled_metrics disables nothing",
			cfg:         Config{DisabledMetrics: []string{}},
			expectExact: container.AllMetrics,
		},
		{
			// The list overrides the built-in set rather than adding to it, so kinds that are
			// disabled by default are enabled again unless they are listed.
			name: "non-empty disabled_metrics overrides the built-in default-disabled set",
			cfg:  Config{DisabledMetrics: []string{"disk", "diskIO"}},
			expectExcluded: []container.MetricKind{
				container.DiskUsageMetrics,
				container.DiskIOMetrics,
			},
			expectIncluded: []container.MetricKind{
				container.ResctrlMetrics,
				container.ProcessMetrics,
				container.CpuUsageMetrics,
			},
		},
		{
			name: "enabled_metrics selects exactly the listed kinds",
			cfg:  Config{EnabledMetrics: []string{"cpu", "memory"}},
			expectExact: container.MetricSet{
				container.CpuUsageMetrics:    struct{}{},
				container.MemoryUsageMetrics: struct{}{},
			},
		},
		{
			name: "enabled_metrics takes precedence over disabled_metrics",
			cfg: Config{
				DisabledMetrics: []string{"cpu"},
				EnabledMetrics:  []string{"cpu", "memory"},
			},
			expectExact: container.MetricSet{
				container.CpuUsageMetrics:    struct{}{},
				container.MemoryUsageMetrics: struct{}{},
			},
		},
		{
			name: "empty enabled_metrics falls back to disabled_metrics",
			cfg: Config{
				DisabledMetrics: []string{"disk"},
				EnabledMetrics:  []string{},
			},
			expectExcluded: []container.MetricKind{container.DiskUsageMetrics},
			expectIncluded: []container.MetricKind{container.CpuUsageMetrics, container.ResctrlMetrics},
		},
		{
			name:      "unsupported disabled_metrics kind is rejected",
			cfg:       Config{DisabledMetrics: []string{"not_a_metric"}},
			expectErr: `failed to set disabled metrics: unsupported metric "not_a_metric" specified`,
		},
		{
			name:      "unsupported enabled_metrics kind is rejected",
			cfg:       Config{EnabledMetrics: []string{"not_a_metric"}},
			expectErr: `failed to set enabled metrics: unsupported metric "not_a_metric" specified`,
		},
		{
			// disabled_metrics is validated even when enabled_metrics wins.
			name: "unsupported disabled_metrics kind is rejected alongside enabled_metrics",
			cfg: Config{
				DisabledMetrics: []string{"not_a_metric"},
				EnabledMetrics:  []string{"cpu"},
			},
			expectErr: `failed to set disabled metrics: unsupported metric "not_a_metric" specified`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			included, err := tc.cfg.GetIncludedMetrics()
			if tc.expectErr != "" {
				require.EqualError(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)

			if tc.expectExact != nil {
				require.True(t, maps.Equal(tc.expectExact, included), "expected %q, got %q", tc.expectExact, included)
			}
			for _, kind := range tc.expectIncluded {
				require.Truef(t, included.Has(kind), "expected %q to be included, got %q", kind, included)
			}
			for _, kind := range tc.expectExcluded {
				require.Falsef(t, included.Has(kind), "expected %q to be excluded, got %q", kind, included)
			}
		})
	}
}

// TestConfig_GetIncludedMetrics_KeepsDefaultsIntact makes sure that parsing a configuration never
// mutates the package-level default-disabled set, which would leak the selection of one component
// into every later one.
func TestConfig_GetIncludedMetrics_KeepsDefaultsIntact(t *testing.T) {
	want := maps.Clone(defaultDisabledMetrics)

	overridden := Config{DisabledMetrics: []string{"disk"}}
	_, err := overridden.GetIncludedMetrics()
	require.NoError(t, err)

	require.True(t, maps.Equal(want, defaultDisabledMetrics), "the built-in default-disabled set was mutated: %q", defaultDisabledMetrics)

	// A later configuration that doesn't set disabled_metrics still gets the built-in set.
	included, err := (&Config{}).GetIncludedMetrics()
	require.NoError(t, err)
	require.False(t, included.Has(container.ResctrlMetrics), "the previous configuration leaked into a later one: %q", included)
	require.True(t, included.Has(container.DiskUsageMetrics), "the previous configuration leaked into a later one: %q", included)
}

// TestConfig_GetIncludedMetrics_Concurrent covers several components being evaluated at the same
// time. Run with -race to catch shared mutable state.
func TestConfig_GetIncludedMetrics_Concurrent(t *testing.T) {
	configs := []Config{
		{},
		{DisabledMetrics: []string{"disk"}},
		{DisabledMetrics: []string{}},
		{EnabledMetrics: []string{"cpu"}},
	}

	var wg sync.WaitGroup
	for range 8 {
		for _, cfg := range configs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				included, err := cfg.GetIncludedMetrics()
				require.NoError(t, err)
				require.NotEmpty(t, included)
			}()
		}
	}
	wg.Wait()

	included, err := (&Config{}).GetIncludedMetrics()
	require.NoError(t, err)
	require.False(t, included.Has(container.ResctrlMetrics), "concurrent evaluation leaked into a later configuration: %q", included)
}
