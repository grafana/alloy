package collector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryHashRegistry_SetAndGet(t *testing.T) {
	r := NewQueryHashRegistry(100, time.Hour)
	r.Set("123", "fp_a", "books")

	info, ok := r.Get("123")
	require.True(t, ok)
	assert.Equal(t, "fp_a", info.Fingerprint)
	assert.Equal(t, "books", info.DatabaseName)

	_, ok = r.Get("missing")
	assert.False(t, ok)
}

func TestQueryHashRegistry_Snapshot(t *testing.T) {
	r := NewQueryHashRegistry(100, time.Hour)
	r.Set("1", "fp_a", "db1")
	r.Set("2", "fp_b", "db2")

	snap := r.Snapshot()
	assert.Len(t, snap, 2)
	assert.Equal(t, "fp_a", snap["1"].Fingerprint)
	assert.Equal(t, "fp_b", snap["2"].Fingerprint)
}

func TestQueryHashRegistry_TTLEviction(t *testing.T) {
	r := NewQueryHashRegistry(100, 50*time.Millisecond)
	r.Set("1", "fp_a", "db")
	time.Sleep(80 * time.Millisecond)
	_, ok := r.Get("1")
	assert.False(t, ok, "entry should have expired")
}
