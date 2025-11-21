package dockertarget

// NOTE: This code is adapted from Promtail (90a1d4593e2d690b37333386383870865fe177bf).
// The dockertarget package is used to configure and run the targets that can
// read logs from Docker containers and forward them to other loki components.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
)

func TestDockerTarget(t *testing.T) {
	server := newDockerServer(t)
	defer server.Close()

	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)
	entryHandler := fake.NewClient(func() {})
	client, err := client.NewClientWithOpts(client.WithHost(server.URL))
	require.NoError(t, err)

	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: t.TempDir() + "/positions.yml",
	})
	require.NoError(t, err)

	tgt, err := NewTarget(
		NewMetrics(prometheus.NewRegistry()),
		logger,
		entryHandler,
		ps,
		"flog",
		model.LabelSet{"job": "docker"},
		[]*relabel.Config{},
		client,
	)
	require.NoError(t, err)
	tgt.StartIfNotRunning()

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

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, tgt.Ready())
	}, 5*time.Second, 20*time.Millisecond, "Expected target to finish processing within the time limit.")

	entryHandler.Clear()
	// restart target to simulate container restart
	tgt.Stop()
	tgt.StartIfNotRunning()
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

func TestStartStopStressTest(t *testing.T) {
	server := newDockerServer(t)
	defer server.Close()

	logger := log.NewNopLogger()
	entryHandler := fake.NewClient(func() {})

	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: t.TempDir() + "/positions.yml",
	})
	require.NoError(t, err)

	client, err := client.NewClientWithOpts(client.WithHost(server.URL))
	require.NoError(t, err)

	tgt, err := NewTarget(
		NewMetrics(prometheus.NewRegistry()),
		logger,
		entryHandler,
		ps,
		"flog",
		model.LabelSet{"job": "docker"},
		[]*relabel.Config{},
		client,
	)
	require.NoError(t, err)

	tgt.StartIfNotRunning()

	// Stress test the concurrency of StartIfNotRunning and Stop
	wg := sync.WaitGroup{}
	for range 1000 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tgt.StartIfNotRunning()
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			tgt.Stop()
		}()
	}
	wg.Wait()
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
				ContainerJSONBase: &container.ContainerJSONBase{},
				Mounts:            []container.MountPoint{},
				Config:            &container.Config{Tty: false},
				NetworkSettings:   &container.NetworkSettings{},
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
func assertExpectedLog(c *assert.CollectT, entryHandler *fake.Client, expectedLines []string) {
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
