package gelf

// This code is copied from Promtail. The target package is used to
// configure and run the targets that can read gelf entries and forward them
// to other loki components.

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/go-gelf/v2/gelf"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// severityLevels maps severity levels to severity string levels.
var severityLevels = map[int32]string{
	0: "emergency",
	1: "alert",
	2: "critical",
	3: "error",
	4: "warning",
	5: "notice",
	6: "informational",
	7: "debug",
}

// tailer reads messages from upd stream
type tailer struct {
	metrics    *metrics
	logger     log.Logger
	handler    loki.LogsReceiver
	cfg        tailerConfig
	gelfReader *gelf.Reader
	encodeBuff *bytes.Buffer
	wg         sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

type tailerConfig struct {
	addr                 string
	relabel              []*relabel.Config
	useIncomingTimestamp bool
}

func newTailer(
	metrics *metrics,
	logger log.Logger,
	handler loki.LogsReceiver,
	cfg tailerConfig,
) (*tailer, error) {

	gelfReader, err := gelf.NewReader(cfg.addr)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	t := &tailer{
		metrics:    metrics,
		logger:     log.With(logger, "listen_address", cfg.addr),
		handler:    handler,
		cfg:        cfg,
		gelfReader: gelfReader,
		encodeBuff: bytes.NewBuffer(make([]byte, 0, 1024)),

		ctx:       ctx,
		ctxCancel: cancel,
	}

	t.run()

	return t, err
}

func (t *tailer) run() {
	t.wg.Go(func() {
		level.Info(t.logger).Log("msg", "listening for GELF UDP messages")
		for {
			select {
			case <-t.ctx.Done():
				return
			default:
				msg, err := t.gelfReader.ReadMessage()
				if err != nil {
					level.Error(t.logger).Log("msg", "error while reading gelf message", "error", err)
					t.metrics.errors.Inc()
					continue
				}
				if msg != nil {
					t.metrics.entries.Inc()
					t.handleMessage(msg)
				}
			}
		}
	})
}

func (t *tailer) handleMessage(msg *gelf.Message) {
	lb := labels.NewBuilder(labels.EmptyLabels())

	lb.Set("__gelf_message_level", severityLevels[msg.Level])
	lb.Set("__gelf_message_host", msg.Host)
	lb.Set("__gelf_message_version", msg.Version)
	lb.Set("__gelf_message_facility", msg.Facility)

	processed, _ := relabel.Process(lb.Labels(), t.cfg.relabel...)

	filtered := make(model.LabelSet)
	processed.Range(func(lbl labels.Label) {
		if strings.HasPrefix(lbl.Name, "__") {
			return
		}
		filtered[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	})

	var timestamp time.Time
	if t.cfg.useIncomingTimestamp && msg.TimeUnix != 0 {
		// TimeUnix is the timestamp of the message, in seconds since the UNIX epoch with decimals for fractional seconds.
		timestamp = secondsToUnixTimestamp(msg.TimeUnix)
	} else {
		timestamp = time.Now()
	}
	t.encodeBuff.Reset()
	err := msg.MarshalJSONBuf(t.encodeBuff)
	if err != nil {
		level.Error(t.logger).Log("msg", "error while marshalling gelf message", "error", err)
		t.metrics.errors.Inc()
		return
	}
	t.handler.Chan() <- loki.Entry{
		Labels: filtered,
		Entry: push.Entry{
			Timestamp: timestamp,
			Line:      t.encodeBuff.String(),
		},
	}
}

func (t *tailer) stop() {
	level.Info(t.logger).Log("msg", "Shutting down GELF UDP listener")
	t.ctxCancel()
	if err := t.gelfReader.Close(); err != nil {
		level.Error(t.logger).Log("msg", "error while closing gelf reader", "error", err)
	}
	t.wg.Wait()
}

func secondsToUnixTimestamp(seconds float64) time.Time {
	return time.Unix(0, int64(seconds*float64(time.Second)))
}
