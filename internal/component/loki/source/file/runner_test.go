//go:build !race

package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/tail/watch"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestRunnerTailer(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
	l := util.TestLogger(t)
	ch1 := loki.NewLogsReceiver()
	tempDir := t.TempDir()
	logFile, err := os.CreateTemp(tempDir, "example")
	require.NoError(t, err)
	positionsFile, err := positions.New(l, positions.Config{
		SyncPeriod:        50 * time.Millisecond,
		PositionsFile:     filepath.Join(tempDir, "positions.yaml"),
		IgnoreInvalidYaml: false,
		ReadOnly:          false,
	})
	require.NoError(t, err)
	labels := model.LabelSet{
		"filename": model.LabelValue(logFile.Name()),
		"foo":      "bar",
	}
	tailer, err := newTailer(
		newMetrics(nil),
		l,
		ch1,
		positionsFile,
		logFile.Name(),
		labels,
		"",
		watch.PollingFileWatcherOptions{
			MinPollFrequency: 25 * time.Millisecond,
			MaxPollFrequency: 25 * time.Millisecond,
		},
		false,
		false,
		OnPositionsFileErrorSkip,
		func() bool { return true },
	)
	require.NoError(t, err)

	runner := &runnerReader{
		reader: tailer,
	}

	ctx, cancel := context.WithCancel(t.Context())

	cancel()
	runner.Run(ctx)
	positionsFile.Stop()
	require.NoError(t, logFile.Close())
}

func TestRunnerDecompressor(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
	l := util.TestLogger(t)
	ch1 := loki.NewLogsReceiver()
	tempDir := t.TempDir()
	positionsFile, err := positions.New(l, positions.Config{
		SyncPeriod:        50 * time.Millisecond,
		PositionsFile:     filepath.Join(tempDir, "positions.yaml"),
		IgnoreInvalidYaml: false,
		ReadOnly:          false,
	})
	require.NoError(t, err)
	filename := "testdata/onelinelog.tar.gz"
	labels := model.LabelSet{
		"filename": model.LabelValue(filename),
		"foo":      "bar",
	}
	decompressor, err := newDecompressor(
		newMetrics(nil),
		l,
		ch1,
		positionsFile,
		filename,
		labels,
		"",
		DecompressionConfig{Format: "gz"},
		OnPositionsFileErrorSkip,
		func() bool { return true },
	)
	require.NoError(t, err)

	runner := &runnerReader{
		reader: decompressor,
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	runner.Run(ctx)
	positionsFile.Stop()
}
