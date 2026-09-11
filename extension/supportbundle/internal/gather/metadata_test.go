package gather

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"gopkg.in/yaml.v3"
)

func TestMetadataGatherer(t *testing.T) {
	start := time.Now().Add(-5 * time.Minute)
	opts := Options{
		BuildInfo: component.BuildInfo{
			Command:     "otelcol-test",
			Description: "Test Collector",
			Version:     "1.2.3",
		},
		StartTime: start,
		Duration:  30 * time.Second,
	}

	files, err := Metadata{}.Gather(context.Background(), opts)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "metadata.yaml", files[0].Path)

	var m metadata
	require.NoError(t, yaml.Unmarshal(files[0].Content, &m))
	require.Equal(t, "1.2.3", m.Version)
	require.Equal(t, runtime.GOOS, m.GOOS)
	require.NotEmpty(t, m.Uptime)
	require.Equal(t, "30s", m.CollectionDuration)
	// The bundle is collected after the collector started.
	require.True(t, m.CollectedAt.After(start))
}

func TestMetadataResourceAttributes(t *testing.T) {
	opts := Options{
		StartTime:          time.Now(),
		ResourceAttributes: map[string]string{"service.name": "my-collector"},
	}

	files, err := Metadata{}.Gather(context.Background(), opts)
	require.NoError(t, err)

	var m metadata
	require.NoError(t, yaml.Unmarshal(files[0].Content, &m))
	require.Equal(t, "my-collector", m.ResourceAttributes["service.name"])
}
