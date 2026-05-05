package opamp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestParseURI(t *testing.T) {
	p, err := parseURI("opamp:/tmp/bootstrap.yaml")
	require.NoError(t, err)
	require.Equal(t, "/tmp/bootstrap.yaml", p)

	_, err = parseURI("file:/x")
	require.Error(t, err)
}

func TestComposeDefaultShellAndExtension(t *testing.T) {
	id := uuid.MustParse("01234567-0123-0123-0123-012345678901")
	out, err := Compose(nil, nil, id, 4310)
	require.NoError(t, err)
	require.Contains(t, string(out), "127.0.0.1:4310")
	require.Contains(t, string(out), "01234567-0123-0123-0123-012345678901")
	require.Contains(t, string(out), "extensions:")
	require.Contains(t, string(out), "otlp:")
}

func TestReadAndParseBootstrap(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "b.yaml")
	content := `opamp:
  server:
    endpoint: ws://127.0.0.1:9/v1/opamp
  storage:
    directory: ` + dir + `
receivers:
  nop: {}
exporters:
  nop: {}
service:
  pipelines:
    traces:
      receivers: [nop]
      exporters: [nop]
`
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))

	mgmt, root, err := readAndParseBootstrap(p)
	require.NoError(t, err)
	require.Equal(t, "ws://127.0.0.1:9/v1/opamp", mgmt.Server.Endpoint)
	require.Contains(t, root, "receivers")
	require.NotContains(t, root, "opamp")
}

func TestSaveLoadLastRemoteConfig(t *testing.T) {
	dir := t.TempDir()
	rc := &protobufs.AgentRemoteConfig{
		ConfigHash: []byte{1, 2, 3},
	}
	require.NoError(t, SaveLastReceivedRemoteConfig(dir, rc))
	loaded, err := LoadLastReceivedRemoteConfig(dir)
	require.NoError(t, err)
	require.Equal(t, rc.ConfigHash, loaded.ConfigHash)
}

func TestProviderScheme(t *testing.T) {
	p := newProvider(confmaptest.NewNopProviderSettings())
	require.NoError(t, confmaptest.ValidateProviderScheme(p))
}
