package file

// This code is copied from Promtail to test their decompressor implementation
// of the reader interface.

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/alloy/internal/util"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/goleak"
)

type noopClient struct {
	noopChan chan loki.Entry
	wg       sync.WaitGroup
	once     sync.Once
}

func (n *noopClient) Chan() chan<- loki.Entry {
	return n.noopChan
}

func (n *noopClient) Stop() {
	n.once.Do(func() { close(n.noopChan) })
}

func newNoopClient() *noopClient {
	c := &noopClient{noopChan: make(chan loki.Entry)}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for range c.noopChan {
			// noop
		}
	}()
	return c
}

func BenchmarkReadlines(b *testing.B) {
	entryHandler := newNoopClient()

	scenarios := []struct {
		name string
		file string
	}{
		{
			name: "2000 lines of log .tar.gz compressed",
			file: "testdata/short-access.tar.gz",
		},
		{
			name: "100000 lines of log .gz compressed",
			file: "testdata/long-access.gz",
		},
	}

	for _, tc := range scenarios {
		b.Run(tc.name, func(b *testing.B) {
			decBase := &decompressor{
				logger:   log.NewNopLogger(),
				running:  atomic.NewBool(false),
				receiver: loki.NewLogsReceiver(),
				path:     tc.file,
			}

			for i := 0; i < b.N; i++ {
				newDec := decBase
				newDec.metrics = newMetrics(prometheus.NewRegistry())
				newDec.done = make(chan struct{})
				newDec.readLines(entryHandler)
				<-newDec.done
			}
		})
	}
}

func TestGigantiqueGunzipFile(t *testing.T) {
	file := "testdata/long-access.gz"
	handler := fake.NewClient(func() {})
	defer handler.Stop()

	d := &decompressor{
		logger:   log.NewNopLogger(),
		running:  atomic.NewBool(false),
		receiver: loki.NewLogsReceiver(),
		path:     file,
		done:     make(chan struct{}),
		metrics:  newMetrics(prometheus.NewRegistry()),
		cfg:      DecompressionConfig{Format: "gz"},
	}

	d.readLines(handler)

	<-d.done
	time.Sleep(time.Millisecond * 200)

	entries := handler.Received()
	require.Equal(t, 100000, len(entries))
}

// TestOnelineFiles test the supported formats for log lines that only contain 1 line.
//
// Based on our experience, this is the scenario with the most edge cases.
func TestOnelineFiles(t *testing.T) {
	fileContent, err := os.ReadFile("testdata/onelinelog.log")
	require.NoError(t, err)
	t.Run("gunzip file", func(t *testing.T) {
		file := "testdata/onelinelog.log.gz"
		handler := fake.NewClient(func() {})
		defer handler.Stop()

		d := &decompressor{
			logger:   log.NewNopLogger(),
			running:  atomic.NewBool(false),
			receiver: loki.NewLogsReceiver(),
			path:     file,
			done:     make(chan struct{}),
			metrics:  newMetrics(prometheus.NewRegistry()),
			cfg:      DecompressionConfig{Format: "gz"},
		}

		d.readLines(handler)

		<-d.done
		time.Sleep(time.Millisecond * 200)

		entries := handler.Received()
		require.Equal(t, 1, len(entries))
		require.Equal(t, string(fileContent), entries[0].Line)
	})

	t.Run("bzip2 file", func(t *testing.T) {
		file := "testdata/onelinelog.log.bz2"
		handler := fake.NewClient(func() {})
		defer handler.Stop()

		d := &decompressor{
			logger:   log.NewNopLogger(),
			running:  atomic.NewBool(false),
			receiver: loki.NewLogsReceiver(),
			path:     file,
			done:     make(chan struct{}),
			metrics:  newMetrics(prometheus.NewRegistry()),
			cfg:      DecompressionConfig{Format: "bz2"},
		}

		d.readLines(handler)

		<-d.done
		time.Sleep(time.Millisecond * 200)

		entries := handler.Received()
		require.Equal(t, 1, len(entries))
		require.Equal(t, string(fileContent), entries[0].Line)
	})

	t.Run("tar.gz file", func(t *testing.T) {
		file := "testdata/onelinelog.tar.gz"
		handler := fake.NewClient(func() {})
		defer handler.Stop()

		d := &decompressor{
			logger:   log.NewNopLogger(),
			running:  atomic.NewBool(false),
			receiver: loki.NewLogsReceiver(),
			path:     file,
			done:     make(chan struct{}),
			metrics:  newMetrics(prometheus.NewRegistry()),
			cfg:      DecompressionConfig{Format: "gz"},
		}

		d.readLines(handler)

		<-d.done
		time.Sleep(time.Millisecond * 200)

		entries := handler.Received()
		require.Equal(t, 1, len(entries))
		firstEntry := entries[0]
		require.Contains(t, firstEntry.Line, "onelinelog.log") // contains .tar.gz headers
		require.Contains(t, firstEntry.Line, `5.202.214.160 - - [26/Jan/2019:19:45:25 +0330] "GET / HTTP/1.1" 200 30975 "https://www.zanbil.ir/" "Mozilla/5.0 (Windows NT 6.2; WOW64; rv:21.0) Gecko/20100101 Firefox/21.0" "-"`)
	})
}

func TestDecompressor(t *testing.T) {
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
	go decompressor.Run()

	select {
	case logEntry := <-ch1.Chan():
		require.Contains(t, logEntry.Line, "onelinelog.log")
	case <-time.After(1 * time.Second):
		require.FailNow(t, "failed waiting for log line")
	}

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pos, err := positionsFile.Get(filename, labels.String())
		assert.NoError(c, err)
		assert.Equal(c, int64(1), pos)
	}, time.Second, 50*time.Millisecond)

	decompressor.Stop()

	// Run the decompressor again
	go decompressor.Run()
	select {
	case <-ch1.Chan():
		t.Fatal("no message should be sent because of the position file")
	case <-time.After(1 * time.Second):
	}

	decompressor.Stop()

	positionsFile.Stop()
}

func TestDecompressorPositionFileEntryDeleted(t *testing.T) {
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
		func() bool { return false },
	)
	go decompressor.Run()

	select {
	case logEntry := <-ch1.Chan():
		require.Contains(t, logEntry.Line, "onelinelog.log")
	case <-time.After(1 * time.Second):
		require.FailNow(t, "failed waiting for log line")
	}

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pos, err := positionsFile.Get(filename, labels.String())
		assert.NoError(c, err)
		assert.Equal(c, int64(1), pos)
	}, time.Second, 50*time.Millisecond)

	decompressor.Stop()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pos, err := positionsFile.Get(filename, labels.String())
		assert.NoError(c, err)
		assert.Equal(c, int64(0), pos)
	}, time.Second, 50*time.Millisecond)

	positionsFile.Stop()
}
