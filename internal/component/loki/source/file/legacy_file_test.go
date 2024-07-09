//go:build linux

package file

// NOTE(rfratto): the following test needs to be rewritten to not rely on the
// removed static mode code (logs.New)

/*
import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/loki/v3/pkg/loghttp/push"
	"gopkg.in/yaml.v2"

	logkit "github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)


// TestFullEndToEndLegacyConversion tests using static mode to tail a log, then converting and then writing new entries and ensuring
// the previously tailed data does not come through.
func TestFullEndToEndLegacyConversion(t *testing.T) {
	//
	// Create a temporary file to tail
	//
	positionsDir := t.TempDir()
	tmpFileDir := t.TempDir()

	tmpFile, err := os.CreateTemp(tmpFileDir, "*.log")
	require.NoError(t, err)

	//
	// Listen for push requests and pass them through to a channel
	//
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, lis.Close())
	})
	var written atomic.Bool
	go func() {
		_ = http.Serve(lis, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			_, err := push.ParseRequest(logkit.NewNopLogger(), "user_id", r, nil, nil, push.ParseLokiRequest)
			require.NoError(t, err)
			_, _ = rw.Write(nil)
			written.Store(true)
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

	var cfg logs.Config
	dec := yaml.NewDecoder(strings.NewReader(cfgText))
	dec.SetStrict(true)
	require.NoError(t, dec.Decode(&cfg))
	require.NoError(t, cfg.ApplyDefaults())
	logger := logkit.NewNopLogger()
	l, err := logs.New(prometheus.NewRegistry(), &cfg, logger, false)
	require.NoError(t, err)
	//
	// Write a log line and wait for it to come through.
	//
	fmt.Fprintf(tmpFile, "Hello, world!\n")
	// Ensure we have written and received the data.
	require.Eventually(t, func() bool {
		return written.Load()
	}, 10*time.Second, 100*time.Millisecond)

	// Stop the tailer.
	l.Stop()

	// Read the legacy file so we can ensure it is the same as the the new file logically.
	oldPositions, err := os.ReadFile(filepath.Join(positionsDir, "default.yml"))
	require.NoError(t, err)

	legacy := positions.LegacyFile{Positions: make(map[string]string)}
	err = yaml.UnmarshalStrict(oldPositions, legacy)
	require.NoError(t, err)

	opts := component.Options{
		Logger:        util.TestFlowLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
		DataPath:      t.TempDir(),
	}

	// Create the Logs receiver component which will convert the legacy positions file into the new format.
	ch1 := loki.NewLogsReceiver()
	args := Arguments{}
	args.LegacyPositionsFile = filepath.Join(positionsDir, "default.yml")
	args.ForwardTo = []loki.LogsReceiver{ch1}
	args.Targets = []discovery.Target{
		{
			"__path__": tmpFile.Name(),
		},
	}

	// New will do the actual conversion
	c, err := New(opts, args)
	require.NoError(t, err)
	require.NotNil(t, c)

	// Before we actually start the component check to see if the legacy file and new file are logically the same.
	buf, err := os.ReadFile(filepath.Join(opts.DataPath, "positions.yml"))
	require.NoError(t, err)
	newPositions := positions.File{Positions: make(map[positions.Entry]string)}
	err = yaml.UnmarshalStrict(buf, newPositions)
	require.NoError(t, err)

	require.Len(t, newPositions.Positions, 1)
	require.Len(t, legacy.Positions, 1)

	for k, v := range newPositions.Positions {
		val, found := legacy.Positions[k.Path]
		require.True(t, found)
		require.True(t, val == v)
	}

	// Write some data, we should see this data but not old data.
	fmt.Fprintf(tmpFile, "new thing!\n")
	ctx := context.Background()
	ctx, cncl := context.WithTimeout(ctx, 10*time.Second)
	defer cncl()
	go func() {
		runErr := c.Run(ctx)
		require.NoError(t, runErr)
	}()

	// Check for the new data ensuring that we do not see the old data.
	require.Eventually(t, func() bool {
		entry := <-ch1.Chan()
		// We don't want to reread the hello world so if something went wrong then the conversion didnt work.
		require.False(t, strings.Contains(entry.Line, "Hello, world!"))
		return strings.Contains(entry.Line, "new thing!")
	}, 5*time.Second, 100*time.Millisecond)
}
*/
