package logs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/stretchr/testify/require"
)

func TestLogs(t *testing.T) {
	dir := t.TempDir()
	c, err := NewComponent(
		component.Options{
			DataPath: dir,
		},
		Arguments{
			WriteCadence:     1 * time.Second,
			NumberOfFiles:    1,
			MessageMaxLength: 10,
			MessageMinLength: 10,
			FileChurnPercent: 0,
			FileRefresh:      1 * time.Minute,
		})
	require.NoError(t, err)
	ctx := context.Background()
	ctx, cncl := context.WithTimeout(ctx, 5*time.Second)
	c.Run(ctx)
	cncl()
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Equal(t, 1, len(files))
	fi, err := files[0].Info()
	require.NoError(t, err)

	require.True(t, fi.Name() == "1.log")
	require.True(t, fi.Size() > 0)
	data, err := os.ReadFile(filepath.Join(dir, fi.Name()))
	require.NoError(t, err)
	lines := strings.Split(string(data), "\n")
	for _, l := range lines {
		if len(l) == 0 {
			continue
		}
		entry := make(map[string]string)
		err = json.Unmarshal([]byte(l), &entry)
		require.NoError(t, err)
	}

}
