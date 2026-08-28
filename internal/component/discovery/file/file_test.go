package file

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/file"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

func TestUnmarshal(t *testing.T) {
	cfg := `
	refresh_interval = "10m"
	files = ["file1", "file2"]`

	var args Arguments
	err := syntax.Unmarshal([]byte(cfg), &args)
	require.NoError(t, err)
	require.Equal(t, 2, len(args.Files))
	require.Equal(t, 10*time.Minute, args.RefreshInterval)
}

func TestUnmarshal_Defaults(t *testing.T) {
	cfg := `files = ["file1"]`

	var args Arguments
	err := syntax.Unmarshal([]byte(cfg), &args)
	require.NoError(t, err)
	require.Equal(t, 1, len(args.Files))
	require.Equal(t, 5*time.Minute, args.RefreshInterval)
}

func TestConvert(t *testing.T) {
	args := Arguments{
		Files:           []string{"file1", "file2"},
		RefreshInterval: 10 * time.Minute,
	}

	promSDConfig := args.Convert().(*prom_discovery.SDConfig)
	require.Equal(t, 2, len(promSDConfig.Files))
	require.Equal(t, model.Duration(10*time.Minute), promSDConfig.RefreshInterval)
}
