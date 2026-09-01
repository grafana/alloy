package supportbundle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	t.Run("accepts default config", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		require.NoError(t, cfg.Validate())
	})

	t.Run("rejects path without leading slash", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Path = "support"
		require.Error(t, cfg.Validate())
	})

	t.Run("rejects zero default_collection_duration", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.DefaultCollectionDuration = 0
		require.Error(t, cfg.Validate())
	})

	t.Run("rejects negative max_collection_duration", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.MaxCollectionDuration = -1 * time.Second
		require.Error(t, cfg.Validate())
	})

	t.Run("rejects default above max", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.DefaultCollectionDuration = 90 * time.Second
		cfg.MaxCollectionDuration = 60 * time.Second
		require.Error(t, cfg.Validate())
	})

	t.Run("rejects negative log_buffer_limit", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.LogBufferLimit = -1
		require.Error(t, cfg.Validate())
	})

	t.Run("rejects path with mux pattern characters", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Path = "/{id}"
		require.Error(t, cfg.Validate())
	})

	t.Run("rejects write_timeout shorter than max window", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.WriteTimeout = cfg.MaxCollectionDuration // not strictly greater
		require.Error(t, cfg.Validate())
	})

	t.Run("accepts write_timeout greater than max window", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.WriteTimeout = cfg.MaxCollectionDuration + time.Minute
		require.NoError(t, cfg.Validate())
	})
}
