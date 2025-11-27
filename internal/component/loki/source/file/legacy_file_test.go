//go:build linux

package file

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util"
)

func TestLegacyConversion(t *testing.T) {
	tmpFileDir := t.TempDir()

	// create legacy position file
	legacyPositionFilename := filepath.Join(tmpFileDir, "legacy.yaml")
	fmt.Println(legacyPositionFilename)
	legacy, err := os.Create(legacyPositionFilename)
	require.NoError(t, err)

	legacyPositions := positions.LegacyFile{
		Positions: map[string]string{
			filepath.Join(tmpFileDir, "my-log.log"): "12",
		},
	}
	err = yaml.NewEncoder(legacy).Encode(&legacyPositions)
	require.NoError(t, err)
	require.NoError(t, legacy.Close())

	// create log file
	logFilename := filepath.Join(tmpFileDir, "my-log.log")
	logFile, err := os.Create(logFilename)
	require.NoError(t, err)

	logFile.Write([]byte(util.Untab(`log 1
log 2
log 3
log 4
log 5
	`)))
	require.NoError(t, logFile.Close())

	ctx := componenttest.TestContext(t)
	ctrl, err := componenttest.NewControllerFromID(logging.NewNop(), "loki.source.file")
	require.NoError(t, err)

	rec := loki.NewLogsReceiver()

	go func() {
		err := ctrl.Run(ctx, Arguments{
			Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
				"__path__": logFilename,
				"foo":      "bar",
			})},
			LegacyPositionsFile: legacyPositionFilename,
			ForwardTo:           []loki.LogsReceiver{rec},
			FileMatch: FileMatch{
				SyncPeriod: 10 * time.Second,
			},
		})
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(10*time.Second))

	entry := <-rec.Chan()
	require.Equal(t, "log 3", entry.Line)
	entry = <-rec.Chan()
	require.Equal(t, "log 4", entry.Line)
	entry = <-rec.Chan()
	require.Equal(t, "log 5", entry.Line)
}
