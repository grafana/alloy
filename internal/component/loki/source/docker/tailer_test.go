package docker

// NOTE: This code is adapted from Promtail (90a1d4593e2d690b37333386383870865fe177bf).
// The dockertarget package is used to configure and run the targets that can
// read logs from Docker containers and forward them to other loki components.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
)

const restartInterval = 20 * time.Millisecond

func TestTailer(t *testing.T) {
	server := newDockerServer(t)
	defer server.Close()

	logger := log.NewNopLogger()
	entryHandler := loki.NewCollectingHandler()
	client, err := client.NewClientWithOpts(client.WithHost(server.URL))
	require.NoError(t, err)

	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: t.TempDir() + "/positions.yml",
	})
	require.NoError(t, err)

	tailer, err := newTailer(
		newMetrics(prometheus.NewRegistry()),
		logger,
		entryHandler.Receiver(),
		ps,
		"flog",
		model.LabelSet{"job": "docker"},
		[]*relabel.Config{},
		client,
		restartInterval,
		func() bool { return false },
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	wg := sync.WaitGroup{}
	wg.Go(func() {
		tailer.Run(ctx)
	})

	expectedLines := []string{
		"5.3.69.55 - - [09/Dec/2021:09:15:02 +0000] \"HEAD /brand/users/clicks-and-mortar/front-end HTTP/2.0\" 503 27087",
		"101.54.183.185 - - [09/Dec/2021:09:15:03 +0000] \"POST /next-generation HTTP/1.0\" 416 11468",
		"69.27.137.160 - runolfsdottir2670 [09/Dec/2021:09:15:03 +0000] \"HEAD /content/visionary/engineer/cultivate HTTP/1.1\" 302 2975",
		"28.104.242.74 - - [09/Dec/2021:09:15:03 +0000] \"PATCH /value-added/cultivate/systems HTTP/2.0\" 405 11843",
		"150.187.51.54 - satterfield1852 [09/Dec/2021:09:15:03 +0000] \"GET /incentivize/deliver/innovative/cross-platform HTTP/1.1\" 301 13032",
	}

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assertExpectedLog(c, entryHandler, expectedLines)
	}, 5*time.Second, 100*time.Millisecond, "Expected log lines were not found within the time limit.")

	entryHandler.Clear()
	// restart target to simulate container restart
	cancel()
	wg.Wait()

	ctx, cancel = context.WithCancel(t.Context())
	defer cancel()
	wg.Go(func() {
		tailer.Run(ctx)
	})
	expectedLinesAfterRestart := []string{
		"243.115.12.215 - - [09/Dec/2023:09:16:57 +0000] \"DELETE /morph/exploit/granular HTTP/1.0\" 500 26468",
		"221.41.123.237 - - [09/Dec/2023:09:16:57 +0000] \"DELETE /user-centric/whiteboard HTTP/2.0\" 205 22487",
		"89.111.144.144 - - [09/Dec/2023:09:16:57 +0000] \"DELETE /open-source/e-commerce HTTP/1.0\" 401 11092",
		"62.180.191.187 - - [09/Dec/2023:09:16:57 +0000] \"DELETE /cultivate/integrate/technologies HTTP/2.0\" 302 12979",
		"156.249.2.192 - - [09/Dec/2023:09:16:57 +0000] \"POST /revolutionize/mesh/metrics HTTP/2.0\" 401 5297",
	}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assertExpectedLog(c, entryHandler, expectedLinesAfterRestart)
	}, 5*time.Second, 100*time.Millisecond, "Expected log lines after restart were not found within the time limit.")
}

func TestTailerStartStopStressTest(t *testing.T) {
	server := newDockerServer(t)
	defer server.Close()

	logger := log.NewNopLogger()
	entryHandler := loki.NewCollectingHandler()

	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: t.TempDir() + "/positions.yml",
	})
	require.NoError(t, err)

	client, err := client.NewClientWithOpts(client.WithHost(server.URL))
	require.NoError(t, err)

	tgt, err := newTailer(
		newMetrics(prometheus.NewRegistry()),
		logger,
		entryHandler.Receiver(),
		ps,
		"flog",
		model.LabelSet{"job": "docker"},
		[]*relabel.Config{},
		client,
		restartInterval,
		func() bool { return false },
	)
	require.NoError(t, err)

	tgt.startIfNotRunning()

	// Stress test the concurrency of StartIfNotRunning and Stop
	wg := sync.WaitGroup{}
	for range 1000 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tgt.startIfNotRunning()
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			tgt.stop()
		}()
	}
	wg.Wait()
}

func TestTailerRestart(t *testing.T) {
	finishedAt := "2024-05-02T13:11:55.879889Z"
	runningState := atomic.NewBool(true)

	client := clientMock{
		logLine:    "2024-05-02T13:11:55.879889Z caller=module_service.go:114 msg=\"module stopped\" module=distributor",
		running:    func() bool { return runningState.Load() },
		finishedAt: func() string { return finishedAt },
	}
	expectedLogLine := "caller=module_service.go:114 msg=\"module stopped\" module=distributor"

	tailer, entryHandler := setupTailer(t, client)
	go tailer.Run(t.Context())

	// The container is already running, expect log lines.
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		logLines := entryHandler.Received()
		if assert.NotEmpty(c, logLines) {
			assert.Equal(c, expectedLogLine, logLines[0].Line)
		}
	}, time.Second, 20*time.Millisecond, "Expected log lines were not found within the time limit.")

	// Stops the container.
	runningState.Store(false)
	time.Sleep(restartInterval + 10*time.Millisecond) // Sleep for a duration greater than targetRestartInterval to make sure it stops sending log lines.
	entryHandler.Clear()
	time.Sleep(restartInterval + 10*time.Millisecond)
	assert.Empty(t, entryHandler.Received()) // No log lines because the container was not running.

	// Restart the container and expect log lines.
	runningState.Store(true)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		logLines := entryHandler.Received()
		if assert.NotEmpty(c, logLines) {
			assert.Equal(c, expectedLogLine, logLines[0].Line)
		}
	}, time.Second, 20*time.Millisecond, "Expected log lines were not found within the time limit after restart.")
}

func TestTailerNeverStarted(t *testing.T) {
	runningState := false
	finishedAt := "2024-05-02T13:11:55.879889Z"
	client := clientMock{
		logLine:    "2024-05-02T13:11:55.879889Z caller=module_service.go:114 msg=\"module stopped\" module=distributor",
		running:    func() bool { return runningState },
		finishedAt: func() string { return finishedAt },
	}
	expectedLogLine := "caller=module_service.go:114 msg=\"module stopped\" module=distributor"

	tailer, entryHandler := setupTailer(t, client)

	ctx, cancel := context.WithCancel(t.Context())
	go tailer.Run(ctx)

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		logLines := entryHandler.Received()
		if assert.NotEmpty(c, logLines) {
			assert.Equal(c, expectedLogLine, logLines[0].Line)
		}
	}, time.Second, 20*time.Millisecond, "Expected log lines were not found within the time limit after restart.")

	require.NotPanics(t, func() { cancel() })
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
			logger:            log.NewNopLogger(),
			recv:              collector.Receiver(),
			positions:         positions.NewNop(),
			containerID:       "test",
			metrics:           newMetrics(prometheus.DefaultRegisterer),
			running:           true,
			wg:                sync.WaitGroup{},
			last:              atomic.NewInt64(0),
			since:             atomic.NewInt64(0),
			componentStopping: func() bool { return false },
		}

		bb := &bytes.Buffer{}
		writer := stdcopy.NewStdWriter(bb, stdcopy.Stdout)
		_, err := writer.Write([]byte("2023-12-09T12:00:00.000000000Z \n2023-12-09T12:00:00.000000000Z line\n"))
		require.NoError(t, err)

		tailer.wg.Add(3)
		go func() {
			tailer.processLoop(t.Context(), false, newStringReader(bb.String()))
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

	t.Run("bigger than mix size", func(t *testing.T) {
		collector := loki.NewCollectingHandler()
		tailer := &tailer{
			logger:            log.NewJSONLogger(os.Stdout),
			recv:              collector.Receiver(),
			positions:         positions.NewNop(),
			containerID:       "test",
			metrics:           newMetrics(prometheus.DefaultRegisterer),
			running:           true,
			wg:                sync.WaitGroup{},
			last:              atomic.NewInt64(0),
			since:             atomic.NewInt64(0),
			componentStopping: func() bool { return false },
		}

		bb := &bytes.Buffer{}
		writer := stdcopy.NewStdWriter(bb, stdcopy.Stdout)

		line := bytes.Repeat([]byte{'a'}, dockerMaxChunkSize*64*10)
		line = append(line, '\n')

		_, err := writer.Write(append([]byte("2023-12-09T12:00:00.000000000Z "), line...))
		require.NoError(t, err)

		_, err = writer.Write([]byte("2023-12-09T12:00:00.000000000Z next line\n"))
		require.NoError(t, err)

		tailer.wg.Add(3)

		go func() {
			tailer.processLoop(t.Context(), false, newStringReader(bb.String()))
		}()

		require.Eventually(t, func() bool {
			return len(collector.Received()) == 1
		}, 2*time.Second, 50*time.Millisecond)

		entry := collector.Received()[0]
		require.Equal(t, "next line", entry.Line)
	})
}

func TestChunkWriter(t *testing.T) {
	logger := log.NewNopLogger()
	var buf bytes.Buffer
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

func newDockerServer(t *testing.T) *httptest.Server {
	h := func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		ctx := r.Context()
		var writeErr error
		switch {
		case strings.HasSuffix(path, "/logs"):
			var filePath string
			if strings.Contains(r.URL.RawQuery, "since=0") {
				filePath = "testdata/flog.log"
			} else {
				filePath = "testdata/flog_after_restart.log"
			}
			dat, err := os.ReadFile(filePath)
			require.NoError(t, err)
			_, writeErr = w.Write(dat)
		default:
			w.Header().Set("Content-Type", "application/json")
			info := container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					State: &container.State{},
				},
				Mounts:          []container.MountPoint{},
				Config:          &container.Config{Tty: false},
				NetworkSettings: &container.NetworkSettings{},
			}
			writeErr = json.NewEncoder(w).Encode(info)
		}
		if writeErr != nil {
			select {
			case <-ctx.Done():
				// Context was done, the write error is likely client disconnect or server shutdown, ignore
				return
			default:
				require.NoError(t, writeErr, "unexpected write error not caused by context being done")
			}
		}
	}

	return httptest.NewServer(http.HandlerFunc(h))
}

// assertExpectedLog will verify that all expectedLines were received, in any order, without duplicates.
func assertExpectedLog(c *assert.CollectT, entryHandler *loki.CollectingHandler, expectedLines []string) {
	logLines := entryHandler.Received()
	testLogLines := make(map[string]int)
	for _, l := range logLines {
		if containsString(expectedLines, l.Line) {
			testLogLines[l.Line] += 1
		}
	}
	// assert that all log lines were received
	assert.Len(c, testLogLines, len(expectedLines))
	// assert that there are no duplicated log lines
	for _, v := range testLogLines {
		assert.Equal(c, v, 1)
	}
}

func containsString(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

func setupTailer(t *testing.T, client clientMock) (*tailer, *loki.CollectingHandler) {
	logger := log.NewNopLogger()
	entryHandler := loki.NewCollectingHandler()

	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: t.TempDir() + "/positions.yml",
	})
	require.NoError(t, err)

	tailer, err := newTailer(
		newMetrics(prometheus.NewRegistry()),
		logger,
		entryHandler.Receiver(),
		ps,
		"flog",
		model.LabelSet{"job": "docker"},
		[]*relabel.Config{},
		client,
		restartInterval,
		func() bool { return false },
	)
	require.NoError(t, err)

	return tailer, entryHandler
}

type clientMock struct {
	client.APIClient
	logLine    string
	running    func() bool
	finishedAt func() string
}

func (mock clientMock) ContainerInspect(ctx context.Context, c string) (container.InspectResponse, error) {
	return container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID: c,
			State: &container.State{
				Running:    mock.running(),
				FinishedAt: mock.finishedAt(),
			},
		},
		Config: &container.Config{Tty: true},
	}, nil
}

func (mock clientMock) ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(mock.logLine)), nil
}
