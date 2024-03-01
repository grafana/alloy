//go:build !race

package logs

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/loghttp/push"

	"github.com/go-kit/log"
	"github.com/grafana/agent/internal/util"
	"github.com/grafana/loki/pkg/logproto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestLogs_NilConfig(t *testing.T) {
	l, err := New(prometheus.NewRegistry(), nil, util.TestLogger(t), false)
	require.NoError(t, err)
	require.NoError(t, l.ApplyConfig(nil, false))

	defer l.Stop()
}

func TestLogs(t *testing.T) {
	//
	// Create a temporary file to tail
	//
	positionsDir := t.TempDir()

	tmpFile, err := os.CreateTemp(os.TempDir(), "*.log")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(tmpFile.Name())
	})

	//
	// Listen for push requests and pass them through to a channel
	//
	pushes := make(chan *logproto.PushRequest)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, lis.Close())
	})
	go func() {
		_ = http.Serve(lis, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			req, err := push.ParseRequest(log.NewNopLogger(), "user_id", r, nil, nil, push.ParseLokiRequest)
			require.NoError(t, err)

			pushes <- req
			_, _ = rw.Write(nil)
		}))
	}()

	//
	// Launch Loki so it starts tailing the file and writes to our server.
	//
	cfgText := util.Untab(fmt.Sprintf(`
positions_directory: %s
configs:
- name: default
  clients:
  - url: http://%s/loki/api/v1/push
		batchwait: 50ms
		batchsize: 1
  scrape_configs:
  - job_name: system
    static_configs:
    - targets: [localhost]
      labels:
        job: test
        __path__: %s
	`, positionsDir, lis.Addr().String(), tmpFile.Name()))

	var cfg Config
	dec := yaml.NewDecoder(strings.NewReader(cfgText))
	dec.SetStrict(true)
	require.NoError(t, dec.Decode(&cfg))
	require.NoError(t, cfg.ApplyDefaults())
	logger := log.NewSyncLogger(log.NewNopLogger())
	l, err := New(prometheus.NewRegistry(), &cfg, logger, false)
	require.NoError(t, err)
	defer l.Stop()

	//
	// Write a log line and wait for it to come through.
	//
	fmt.Fprintf(tmpFile, "Hello, world!\n")
	select {
	case <-time.After(time.Second * 30):
		require.FailNow(t, "timed out waiting for data to be pushed")
	case req := <-pushes:
		require.Equal(t, "Hello, world!", req.Streams[0].Entries[0].Line)
	}

	//
	// Apply a new config and write a new line.
	//
	cfgText = util.Untab(fmt.Sprintf(`
positions_directory: %s
configs:
- name: default
  clients:
  - url: http://%s/loki/api/v1/push
		batchwait: 50ms
		batchsize: 5
  scrape_configs:
  - job_name: system
    static_configs:
    - targets: [localhost]
      labels:
        job: test-2
        __path__: %s
	`, positionsDir, lis.Addr().String(), tmpFile.Name()))

	var newCfg Config
	dec = yaml.NewDecoder(strings.NewReader(cfgText))
	dec.SetStrict(true)
	require.NoError(t, dec.Decode(&newCfg))
	require.NoError(t, newCfg.ApplyDefaults())
	require.NoError(t, l.ApplyConfig(&newCfg, false))

	fmt.Fprintf(tmpFile, "Hello again!\n")
	select {
	case <-time.After(time.Second * 30):
		require.FailNow(t, "timed out waiting for data to be pushed")
	case req := <-pushes:
		require.Equal(t, "Hello again!", req.Streams[0].Entries[0].Line)
	}

	t.Run("update to nil", func(t *testing.T) {
		// Applying a nil config should remove all instances.
		err := l.ApplyConfig(nil, false)
		require.NoError(t, err)
		require.Len(t, l.instances, 0)
	})

	t.Run("re-apply previous config", func(t *testing.T) {
		// Applying a nil config should remove all instances.
		l.ApplyConfig(nil, false)

		// Re-Apply the previous config and write a new line.
		var newCfg Config
		dec = yaml.NewDecoder(strings.NewReader(cfgText))
		dec.SetStrict(true)
		require.NoError(t, dec.Decode(&newCfg))
		require.NoError(t, newCfg.ApplyDefaults())
		require.NoError(t, l.ApplyConfig(&newCfg, false))

		fmt.Fprintf(tmpFile, "Hello again!\n")
		select {
		case <-time.After(time.Second * 30):
			require.FailNow(t, "timed out waiting for data to be pushed")
		case req := <-pushes:
			require.Equal(t, "Hello again!", req.Streams[0].Entries[0].Line)
		}
	})
}

func TestLogs_PositionsDirectory(t *testing.T) {
	//
	// Create a temporary file to tail
	//
	positionsDir := t.TempDir()

	//
	// Launch Loki so it starts tailing the file and writes to our server.
	//
	cfgText := util.Untab(fmt.Sprintf(`
positions_directory: %[1]s/positions
configs:
- name: instance-a
  clients:
	- url: http://127.0.0.1:80/loki/api/v1/push
- name: instance-b
  positions:
	  filename: %[1]s/other-positions/instance.yml
  clients:
	- url: http://127.0.0.1:80/loki/api/v1/push
	`, positionsDir))

	var cfg Config
	dec := yaml.NewDecoder(strings.NewReader(cfgText))
	dec.SetStrict(true)
	require.NoError(t, dec.Decode(&cfg))
	require.NoError(t, cfg.ApplyDefaults())
	logger := util.TestLogger(t)
	l, err := New(prometheus.NewRegistry(), &cfg, logger, false)
	require.NoError(t, err)
	defer l.Stop()

	_, err = os.Stat(filepath.Join(positionsDir, "positions"))
	require.NoError(t, err, "default shared positions directory did not get created")
	_, err = os.Stat(filepath.Join(positionsDir, "other-positions"))
	require.NoError(t, err, "instance-specific positions directory did not get created")
}
