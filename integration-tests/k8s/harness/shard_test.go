package harness

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseShardString_Valid(t *testing.T) {
	tests := []struct {
		in   string
		want shardConfig
	}{
		{"0/1", shardConfig{index: 0, total: 1}},
		{"0/3", shardConfig{index: 0, total: 3}},
		{"2/3", shardConfig{index: 2, total: 3}},
		{"7/8", shardConfig{index: 7, total: 8}},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseShardString(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParseShardString_Invalid(t *testing.T) {
	cases := []string{
		"",        // empty
		"1",       // missing /n
		"1/2/3",   // too many parts
		"a/3",     // non-int index
		"1/b",     // non-int total
		"0/0",     // total == 0
		"0/-1",    // total negative
		"-1/3",    // index negative
		"3/3",     // index == total (must be < total)
		"5/3",     // index > total
		"foo/bar", // both non-int
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			_, err := parseShardString(in)
			require.Error(t, err)
		})
	}
}

func TestValidateShard_DelegatesToParser(t *testing.T) {
	require.NoError(t, ValidateShard("0/3"))
	require.Error(t, ValidateShard(""))
	require.Error(t, ValidateShard("3/3"))
}

func TestShouldRun_NoSharding(t *testing.T) {
	// total == 0 means unsharded: everything runs.
	s := shardConfig{}
	for _, key := range []string{"", "anything", "integration-tests/k8s/tests/foo"} {
		assert.True(t, s.shouldRun(key), "unsharded should run %q", key)
	}
}

func TestShouldRun_PartitionsKeysExactlyOnce(t *testing.T) {
	// Correctness invariant: every key lands on exactly one shard
	// (else we'd silently skip or duplicate tests).
	keys := []string{
		"integration-tests/k8s/tests/mimir-alerts-kubernetes",
		"integration-tests/k8s/tests/prometheus-operator",
		"integration-tests/k8s/tests/foo",
		"integration-tests/k8s/tests/bar",
		"integration-tests/k8s/tests/baz",
		"integration-tests/k8s/tests/qux",
		"a", "b", "c",
	}
	for _, n := range []int{1, 2, 3, 4, 7, 16} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			for _, key := range keys {
				hits := 0
				for i := 0; i < n; i++ {
					if (shardConfig{index: i, total: n}).shouldRun(key) {
						hits++
					}
				}
				assert.Equal(t, 1, hits, "key %q hit on %d/%d shards (want 1)", key, hits, n)
			}
		})
	}
}

func TestShouldRun_Deterministic(t *testing.T) {
	// Same key + same config must always give the same answer.
	s := shardConfig{index: 1, total: 3}
	for _, key := range []string{"foo", "bar", "baz", "longer/package/path"} {
		first := s.shouldRun(key)
		for i := 0; i < 5; i++ {
			require.Equal(t, first, s.shouldRun(key), "non-deterministic for key %q", key)
		}
	}
}

func TestShouldRun_DistributesAcrossShards(t *testing.T) {
	// Sanity: with 8 realistic keys and n=3, no shard is empty.
	keys := []string{
		"integration-tests/k8s/tests/mimir-alerts-kubernetes",
		"integration-tests/k8s/tests/prometheus-operator",
		"integration-tests/k8s/tests/loki-rules",
		"integration-tests/k8s/tests/tempo-traces",
		"integration-tests/k8s/tests/pyroscope-profiles",
		"integration-tests/k8s/tests/otel-receivers",
		"integration-tests/k8s/tests/extra-test-1",
		"integration-tests/k8s/tests/extra-test-2",
	}
	const n = 3
	counts := make([]int, n)
	for _, key := range keys {
		for i := 0; i < n; i++ {
			if (shardConfig{index: i, total: n}).shouldRun(key) {
				counts[i]++
			}
		}
	}
	for i, c := range counts {
		assert.Greater(t, c, 0, "shard %d/%d got 0 keys, distribution looks broken", i, n)
	}
}
