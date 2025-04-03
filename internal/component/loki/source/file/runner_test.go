//go:build !race

package file

import (
	"context"
	"os"
	"path/filepath"
	"sync"
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
		func() bool { return true },
	)
	require.NoError(t, err)

	runner := &runnerReader{
		reader: tailer,
	}

	ctx, cancel := context.WithCancel(t.Context())

	cancel()

	// Run in a goroutine to catch any panics
	var panicErr interface{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr = r
			}
		}()
		runner.Run(ctx)
		wg.Done()
	}()
	wg.Wait()
	require.Nil(t, panicErr, "Run() should not panic when context is cancelled")
	positionsFile.Stop()
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
		func() bool { return true },
	)
	require.NoError(t, err)

	runner := &runnerReader{
		reader: decompressor,
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	// Run in a goroutine to catch any panics
	var panicErr interface{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr = r
			}
		}()
		runner.Run(ctx)
		wg.Done()
	}()
	wg.Wait()
	require.Nil(t, panicErr, "Run() should not panic when context is cancelled")
	positionsFile.Stop()
}
