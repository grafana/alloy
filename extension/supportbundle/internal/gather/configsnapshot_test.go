package gather

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"gopkg.in/yaml.v3"
)

func TestConfigGathererNoSnapshot(t *testing.T) {
	g := &Config{}
	files, err := g.Gather(context.Background(), Options{})
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestConfigCapturesUnexpanded(t *testing.T) {
	// Effective holds the expanded secret. Unexpanded holds the redacted form.
	effective := confmap.NewFromStringMap(map[string]any{
		"exporters": map[string]any{"otlp": map[string]any{"headers": map[string]any{"authorization": "super-secret-token"}}},
	})
	unexpanded := confmap.NewFromStringMap(map[string]any{
		"exporters": map[string]any{"otlp": map[string]any{"headers": map[string]any{"authorization": "[REDACTED]"}}},
	})

	g := &Config{}
	g.Store(extensioncapabilities.NewConfigSnapshot(effective, unexpanded))

	files, err := g.Gather(context.Background(), Options{})
	require.NoError(t, err)

	m := gatherToMap(t, files)
	require.Contains(t, m, "config.yaml")

	// The bundle holds the redacted value, never the expanded secret.
	require.Contains(t, string(m["config.yaml"]), "[REDACTED]")
	require.NotContains(t, string(m["config.yaml"]), "super-secret-token")

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(m["config.yaml"], &parsed))
	require.Contains(t, parsed, "exporters")

	// The effective config is kept in memory for endpoint discovery only.
	require.NotNil(t, g.EffectiveConf())
}

// TestConfigNeverCapturesEffectiveSecrets checks the safety net: when the
// runtime provides no redacted unexpanded config, the gatherer must not fall
// back to the effective config, which holds expanded secrets.
func TestConfigNeverCapturesEffectiveSecrets(t *testing.T) {
	effective := confmap.NewFromStringMap(map[string]any{
		"exporters": map[string]any{"otlp": map[string]any{"headers": map[string]any{"authorization": "super-secret-token"}}},
	})
	// Unexpanded is nil, as it would be when the runtime does not redact.
	g := &Config{}
	g.Store(extensioncapabilities.NewConfigSnapshot(effective, nil))

	files, err := g.Gather(context.Background(), Options{})
	require.NoError(t, err)

	// No config file, and the secret never appears anywhere.
	require.Empty(t, files)
	for _, f := range files {
		require.NotContains(t, string(f.Content), "super-secret-token")
	}
}
