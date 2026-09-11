package supportbundle

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pipeline"
	"gopkg.in/yaml.v3"
)

func startTestExtension(t *testing.T) *supportBundleExtension {
	t.Helper()

	cfg := createDefaultConfig().(*Config)
	cfg.ServerConfig.NetAddr.Endpoint = "localhost:0"
	require.NoError(t, cfg.Validate())

	ext, err := createExtension(context.Background(), extensiontest.NewNopSettings(typeStr), cfg)
	require.NoError(t, err)

	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

	sb := ext.(*supportBundleExtension)
	require.Eventually(t, func() bool { return sb.addr() != nil }, time.Second, 10*time.Millisecond)

	t.Cleanup(func() {
		require.NoError(t, sb.Shutdown(context.Background()))
	})
	return sb
}

func fetchBundle(t *testing.T, sb *supportBundleExtension, query string) map[string][]byte {
	t.Helper()

	url := "http://" + sb.addr().String() + sb.cfg.Path + query
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/zip", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
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
	return got
}

func TestExtensionServesBundle(t *testing.T) {
	sb := startTestExtension(t)

	// A zero duration skips the CPU profile. Mutex and block are always emitted.
	got := fetchBundle(t, sb, "?duration=0")
	require.Contains(t, got, "otelcol-support-bundle/metadata.yaml")
	require.Contains(t, got, "otelcol-support-bundle/pprof/heap.pprof")
	require.Contains(t, got, "otelcol-support-bundle/pprof/goroutine.pprof")
	require.Contains(t, got, "otelcol-support-bundle/pprof/mutex.pprof")
	require.Contains(t, got, "otelcol-support-bundle/pprof/block.pprof")
	require.NotContains(t, got, "otelcol-support-bundle/pprof/cpu.pprof")

	var m map[string]any
	require.NoError(t, yaml.Unmarshal(got["otelcol-support-bundle/metadata.yaml"], &m))
	require.NotEmpty(t, m["go_version"])
}

func TestExtensionServesWindowedProfiles(t *testing.T) {
	sb := startTestExtension(t)

	// A non-zero duration adds the windowed profiles to the bundle.
	got := fetchBundle(t, sb, "?duration=100ms")
	require.Contains(t, got, "otelcol-support-bundle/pprof/cpu.pprof")
	require.Contains(t, got, "otelcol-support-bundle/pprof/mutex.pprof")
	require.Contains(t, got, "otelcol-support-bundle/pprof/block.pprof")
	require.Contains(t, got, "otelcol-support-bundle/pprof/heap.pprof")
}

func TestExtensionServesConfigSnapshot(t *testing.T) {
	sb := startTestExtension(t)

	unexpanded := confmap.NewFromStringMap(map[string]any{
		"receivers": map[string]any{"otlp": map[string]any{"protocols": map[string]any{"grpc": map[string]any{}}}},
	})
	snapshot := extensioncapabilities.NewConfigSnapshot(unexpanded, unexpanded)
	require.NoError(t, sb.NotifyConfigSnapshot(context.Background(), snapshot))

	got := fetchBundle(t, sb, "?duration=0")
	require.Contains(t, got, "otelcol-support-bundle/config.yaml")
	require.Contains(t, string(got["otelcol-support-bundle/config.yaml"]), "receivers")
}

func TestExtensionServesComponentStatus(t *testing.T) {
	sb := startTestExtension(t)

	id := componentstatus.NewInstanceID(component.MustNewID("otlp"), component.KindReceiver,
		pipeline.NewID(pipeline.SignalTraces))
	sb.ComponentStatusChanged(id, componentstatus.NewEvent(componentstatus.StatusOK))

	got := fetchBundle(t, sb, "?duration=0")
	require.Contains(t, got, "otelcol-support-bundle/component-status.yaml")
	require.Contains(t, string(got["otelcol-support-bundle/component-status.yaml"]), "StatusOK")
}

func TestExtensionSequentialRequests(t *testing.T) {
	sb := startTestExtension(t)

	// Two requests in a row both succeed. The handler mutex serializes them.
	for i := 0; i < 2; i++ {
		got := fetchBundle(t, sb, "?duration=0")
		require.Contains(t, got, "otelcol-support-bundle/metadata.yaml")
	}
}

func TestExtensionShutdownWithoutStart(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	ext, err := createExtension(context.Background(), extensiontest.NewNopSettings(typeStr), cfg)
	require.NoError(t, err)

	var _ component.Component = ext
	require.NoError(t, ext.Shutdown(context.Background()))
}
