package docker

// NOTE: This code is adapted from Promtail (90a1d4593e2d690b37333386383870865fe177bf).
// The dockertarget package is used to configure and run the targets that can
// read logs from Docker containers and forward them to other loki components.

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/docker/dockertest"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/runtime/logging"
)

const restartInterval = 20 * time.Millisecond

var testRestartBackoff = backoff.Config{MinBackoff: restartInterval, MaxBackoff: restartInterval}

func TestTailer(t *testing.T) {
	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	server := dockertest.NewServer(t)
	server.AddContainer("flog")
	server.Log("flog", base, "line 1")
	server.Log("flog", base.Add(time.Second), "line 2")
	server.Log("flog", base.Add(2*time.Second), "line 3")

	entryHandler := loki.NewCollectingHandler()
	tailer := newTestTailer(t, server.URL, entryHandler.Receiver())

	ctx, cancel := context.WithCancel(t.Context())
	wg := sync.WaitGroup{}
	wg.Go(func() {
		tailer.Run(ctx, ctx)
	})

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, []string{"line 1", "line 2", "line 3"}, entryLines(entryHandler.Received()))
	}, 5*time.Second, 10*time.Millisecond)

	cancel()
	wg.Wait()
}

func TestTailerRestart(t *testing.T) {
	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	server := dockertest.NewServer(t)
	server.AddContainer("flog")
	server.Log("flog", base, "line 1")

	entryHandler := loki.NewCollectingHandler()
	tailer := newTestTailer(t, server.URL, entryHandler.Receiver())
	go tailer.Run(t.Context(), t.Context())

	// The container is already running, expect log lines.
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, []string{"line 1"}, entryLines(entryHandler.Received()))
	}, time.Second, 20*time.Millisecond)

	// Stops the container.
	server.Stop("flog", base.Add(time.Second))
	entryHandler.Clear()
	time.Sleep(5 * restartInterval)
	assert.Empty(t, entryHandler.Received()) // No log lines because the container was not running.

	// Restart the container and expect log lines.
	server.Start("flog")
	server.Log("flog", base.Add(2*time.Second), "line 2")

	// FIXME(kalleep): positions are whole seconds, so resuming re-sends every
	// line from the last read second. See https://github.com/grafana/alloy/issues/6954.
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, []string{"line 1", "line 2"}, entryLines(entryHandler.Received()))
	}, time.Second, 20*time.Millisecond)
}

func TestTailerNeverStarted(t *testing.T) {
	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	server := dockertest.NewServer(t)
	server.AddContainer("flog")
	server.Log("flog", base, "line 1")
	server.Stop("flog", base)

	entryHandler := loki.NewCollectingHandler()
	tailer := newTestTailer(t, server.URL, entryHandler.Receiver())

	go tailer.Run(t.Context(), t.Context())

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		logLines := entryHandler.Received()
		require.NotEmpty(c, logLines)
		require.Equal(c, "line 1", logLines[0].Line)
	}, time.Second, 20*time.Millisecond)
}

func TestTailerStopsWhenContainerNotFound(t *testing.T) {
	server := dockertest.NewServer(t)
	server.AddContainer("flog")
	server.Log("flog", time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), "some log line")

	entryHandler := loki.NewCollectingHandler()
	tailer := newTestTailer(t, server.URL, entryHandler.Receiver())

	done := make(chan struct{})
	go func() {
		tailer.Run(t.Context(), t.Context())
		close(done)
	}()

	// The container's logs must be shipped before it is removed and the tailer stops.
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, []string{"some log line"}, entryLines(entryHandler.Received()))
	}, 5*time.Second, 20*time.Millisecond, "expected the container's log line to be shipped before the tailer stopped")

	server.Remove("flog")

	select {
	case <-done:
		// Run exited after the container was not found.
	case <-time.After(5 * time.Second):
		t.Fatal("tailer did not stop after container was removed")
	}
}

var _ io.ReadCloser = (*stringReader)(nil)

func newStringReader(s string) *stringReader {
	return &stringReader{Reader: strings.NewReader(s)}
}

type stringReader struct {
	*strings.Reader
}

func (s *stringReader) Close() error {
	return nil
}

func TestTailerConsumeLines(t *testing.T) {
	t.Run("skip empty line", func(t *testing.T) {
		collector := loki.NewCollectingHandler()
		tailer := &tailer{
			logger:      logging.NewSlogNop(),
			recv:        collector.Receiver(),
			positions:   positions.NewNop(),
			containerID: "test",
			metrics:     newMetrics(prometheus.DefaultRegisterer),
			last:        atomic.NewInt64(0),
			since:       atomic.NewInt64(0),
		}

		bb := &bytes.Buffer{}
		writer := newStdWriter(bb, stdcopy.Stdout)
		_, err := writer.Write([]byte("2023-12-09T12:00:00.000000000Z \n2023-12-09T12:00:00.000000000Z line\n"))
		require.NoError(t, err)

		go func() {
			tailer.processLoop(t.Context(), t.Context(), false, newStringReader(bb.String()))
		}()

		require.Eventually(t, func() bool {
			return len(collector.Received()) == 1
		}, 2*time.Second, 50*time.Millisecond)

		entry := collector.Received()[0]

		expectedLine := "line"
		expectedTimestamp, err := time.Parse(time.RFC3339Nano, "2023-12-09T12:00:00.000000000Z")
		require.NoError(t, err)

		require.Equal(t, expectedLine, entry.Line)
		require.Equal(t, expectedTimestamp, entry.Timestamp)
	})

	t.Run("bigger than max size", func(t *testing.T) {
		collector := loki.NewCollectingHandler()
		tailer := &tailer{
			logger:      logging.NewSlogNop(),
			recv:        collector.Receiver(),
			positions:   positions.NewNop(),
			containerID: "test",
			metrics:     newMetrics(prometheus.DefaultRegisterer),
			last:        atomic.NewInt64(0),
			since:       atomic.NewInt64(0),
		}

		bb := &bytes.Buffer{}
		writer := newStdWriter(bb, stdcopy.Stdout)

		line := bytes.Repeat([]byte{'a'}, dockerMaxChunkSize*64*10)
		line = append(line, '\n')

		_, err := writer.Write(append([]byte("2023-12-09T12:00:00.000000000Z "), line...))
		require.NoError(t, err)

		_, err = writer.Write([]byte("2023-12-09T12:00:00.000000000Z next line\n"))
		require.NoError(t, err)

		go func() {
			tailer.processLoop(t.Context(), t.Context(), false, newStringReader(bb.String()))
		}()

		require.Eventually(t, func() bool {
			return len(collector.Received()) == 1
		}, 2*time.Second, 50*time.Millisecond)

		entry := collector.Received()[0]
		require.Equal(t, "next line", entry.Line)
	})
}

func TestChunkWriter(t *testing.T) {
	var (
		buf    bytes.Buffer
		logger = logging.NewSlogNop()
	)

	writer := newChunkWriter(&buf, logger)
	timestamp := []byte("2023-12-09T12:00:00.000000000Z ")
	shortLine := []byte("short log line\n")

	var longContent []byte
	for range 50 * 1024 {
		longContent = append(longContent, 'a')
	}
	longContent = append(longContent, '\n')

	// First part of long line
	chunk1 := append(timestamp, longContent[:32*1024]...)
	_, err := writer.Write(chunk1)
	require.NoError(t, err)

	// Second part of long line
	chunk2 := append(timestamp, longContent[32*1024:]...)
	_, err = writer.Write(chunk2)
	require.NoError(t, err)

	// Start a new short line
	chunk3 := append(timestamp, shortLine...)
	_, err = writer.Write(chunk3)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	expected := append(timestamp, longContent...)
	expected = append(expected, chunk3...)

	assert.Equal(t, expected, buf.Bytes())
}

func TestExtractTsFromBytes(t *testing.T) {
	t.Run("invalid timestamp", func(t *testing.T) {
		_, _, err := extractTsFromBytes([]byte("123 test\n"))
		require.Error(t, err)
	})

	t.Run("valid timestamp empty line", func(t *testing.T) {
		ts, _, err := extractTsFromBytes([]byte("2024-05-02T13:11:55.879889Z \n"))
		require.NoError(t, err)
		expectedTs, err := time.Parse(time.RFC3339Nano, "2024-05-02T13:11:55.879889Z")
		require.NoError(t, err)
		require.Equal(t, expectedTs, ts)
	})
	t.Run("valid timestamp no space", func(t *testing.T) {
		_, _, err := extractTsFromBytes([]byte("2024-05-02T13:11:55.879889Z\n"))
		require.Error(t, err)
	})
}

// newStdWriter frames each Write into the multiplexed stream format that
// stdcopy.StdCopy consumes.
func newStdWriter(w io.Writer, t stdcopy.StdType) io.Writer {
	return &stdWriter{Writer: w, t: t}
}

type stdWriter struct {
	io.Writer
	t stdcopy.StdType
}

func (sw *stdWriter) Write(p []byte) (int, error) {
	var hdr [8]byte
	hdr[0] = byte(sw.t)
	binary.BigEndian.PutUint32(hdr[4:], uint32(len(p)))
	if _, err := sw.Writer.Write(hdr[:]); err != nil {
		return 0, err
	}
	return sw.Writer.Write(p)
}

func TestNewClientDoesNotTimeOutLogStreams(t *testing.T) {
	release := make(chan struct{})
	server := newStallingDockerServer(release)
	defer server.Close()

	args := GetDefaultArguments()
	args.Host = server.URL

	cli, err := newClient(args)
	require.NoError(t, err)

	logs, err := cli.ContainerLogs(t.Context(), "flog", client.ContainerLogsOptions{ShowStdout: true, Follow: true})
	require.NoError(t, err)
	defer logs.Close()

	// Keep the stream open for a while before ending it.
	time.AfterFunc(500*time.Millisecond, func() { close(release) })

	body, err := io.ReadAll(logs)
	require.NoError(t, err)
	require.Equal(t, "done", string(body))
}

func TestTailerStartStopsWhenContextIsCanceled(t *testing.T) {
	release := make(chan struct{})
	server := newStallingDockerServer(release)
	defer server.Close()
	defer close(release)

	args := GetDefaultArguments()
	args.Host = server.URL

	cli, err := newClient(args)
	require.NoError(t, err)

	tailer := newTestTailer(t, server.URL, loki.NewCollectingHandler().Receiver())
	tailer.client = cli

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	time.AfterFunc(100*time.Millisecond, cancel)

	done := make(chan struct{})
	go func() {
		tailer.tail(ctx, ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("tail did not return after its context was canceled")
	}

	require.False(t, tailer.running.Load())
	require.NoError(t, tailer.err.Load(), "a canceled tail is a normal stop, not an error")
}

func newTestTailer(t *testing.T, host string, recv loki.LogsReceiver) *tailer {
	logger := logging.NewSlogNop()

	cli, err := client.New(client.WithHost(host))
	require.NoError(t, err)

	ps, err := positions.New(logger, positions.Config{
		Path: t.TempDir() + "/positions.yml",
	})
	require.NoError(t, err)

	tailer, err := newTailer(
		newMetrics(prometheus.NewRegistry()),
		logger,
		recv,
		ps,
		"flog",
		model.LabelSet{"job": "docker"},
		[]*relabel.Config{},
		cli,
		testRestartBackoff,
	)
	require.NoError(t, err)

	return tailer
}

// newStallingDockerServer returns a Docker API server whose log stream and
// container inspect responses only complete once release is closed.
func newStallingDockerServer(release <-chan struct{}) *httptest.Server {
	h := func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/logs"):
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			select {
			case <-release:
				_, _ = w.Write([]byte("done"))
			case <-r.Context().Done():
			}
		case strings.HasSuffix(r.URL.Path, "/json"):
			select {
			case <-release:
			case <-r.Context().Done():
			}
		default:
			_, _ = w.Write([]byte("OK"))
		}
	}
	return httptest.NewServer(http.HandlerFunc(h))
}
