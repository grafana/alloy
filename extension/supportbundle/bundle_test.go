package supportbundle

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/extension/supportbundle/internal/gather"
)

func TestWriteBundle(t *testing.T) {
	files := []gather.File{
		{Path: "metadata.yaml", Content: []byte("version: 1.2.3\n")},
		{Path: "pprof/heap.pprof", Content: []byte("heap-data")},
	}

	var buf bytes.Buffer
	require.NoError(t, writeBundle(&buf, "otelcol-support-bundle", files))

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)

	got := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		content, err := io.ReadAll(rc)
		require.NoError(t, rc.Close())
		require.NoError(t, err)
		got[f.Name] = content
	}

	require.Contains(t, got, "otelcol-support-bundle/metadata.yaml")
	require.Contains(t, got, "otelcol-support-bundle/pprof/heap.pprof")
	require.Equal(t, []byte("version: 1.2.3\n"), got["otelcol-support-bundle/metadata.yaml"])
	require.Equal(t, []byte("heap-data"), got["otelcol-support-bundle/pprof/heap.pprof"])
}
