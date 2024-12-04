package process

import (
	"context"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/loki/pkg/push"
	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/prometheus/common/model"
	"time"
)

var _ loki.LogsReceiver = (*lokiReceiver)(nil)

type lokiReceiver struct {
	batchSize int
	interval  time.Duration
	wasm      *WasmPlugin
	channel   chan loki.Entry
	logs      []loki.Entry
	forwardto []loki.LogsReceiver
	stop      chan struct{}
}

func (l *lokiReceiver) Chan() chan loki.Entry {
	return l.channel
}

func (l *lokiReceiver) Stop() {
	l.stop <- struct{}{}
}

func (l *lokiReceiver) Start(ctx context.Context) {
	go l.run(ctx)
}

func (l *lokiReceiver) run(ctx context.Context) {
	for {
		select {
		case <-l.stop:
			return
		case <-ctx.Done():
			return
		case <-time.After(l.interval):
			l.batch()
		// Batch because we dont want each individual message
		// to trigger a process..
		case lr := <-l.channel:
			l.logs = append(l.logs, lr)
			if len(l.logs) >= l.batchSize {
				l.batch()
			}
		}
	}
}

func (l *lokiReceiver) batch() {
	if len(l.logs) == 0 {
		return
	}
	defer func() {
		l.logs = l.logs[:0]
	}()
	pt := &Passthrough{
		Lokilogs: make([]*LokiLog, len(l.logs)),
	}
	for i, lr := range l.logs {

		pt.Lokilogs[i] = toLokiLog(lr)
	}
	// TODO handle error
	out, err := l.wasm.Process(pt)
	if err != nil {
		return
	}
	l.send(out.Lokilogs)
}

func (l *lokiReceiver) send(logs []*LokiLog) {
	for _, lg := range logs {
		le := toLokiEntry(lg)
		for _, forward := range l.forwardto {
			forward.Chan() <- le
		}
	}
}

func toLokiEntry(ll *LokiLog) loki.Entry {
	labels := make(model.LabelSet)
	for _, v := range ll.Labels {
		labels[model.LabelName(v.Name)] = model.LabelValue(v.Value)
	}
	metadata := make([]push.LabelAdapter, 0, len(ll.Metadata))
	for _, v := range ll.Metadata {
		newLbl := push.LabelAdapter{
			Name:  v.Name,
			Value: v.Value,
		}
		metadata = append(metadata, newLbl)
	}
	parsed := make([]push.LabelAdapter, 0, len(ll.Parsed))
	for _, v := range ll.Parsed {
		newLbl := push.LabelAdapter{
			Name:  v.Name,
			Value: v.Value,
		}
		parsed = append(parsed, newLbl)
	}
	le := loki.Entry{}
	le.Labels = labels
	le.Entry = logproto.Entry{
		Line:               ll.Line,
		Parsed:             parsed,
		StructuredMetadata: metadata,
		Timestamp:          time.Unix(ll.Timestamp, 0),
	}
	return le
}

func toLokiLog(lr loki.Entry) *LokiLog {
	labels := make([]*Label, 0, len(lr.Labels))
	for k, v := range lr.Labels {
		newLbl := &Label{
			Name:  string(k),
			Value: string(v),
		}
		labels = append(labels, newLbl)
	}
	metadata := make([]*Label, 0, len(lr.StructuredMetadata))
	for _, v := range lr.StructuredMetadata {
		newLbl := &Label{
			Name:  v.Name,
			Value: v.Value,
		}
		metadata = append(metadata, newLbl)
	}
	parsed := make([]*Label, 0, len(lr.Parsed))
	for _, v := range lr.Parsed {
		newLbl := &Label{
			Name:  v.Name,
			Value: v.Value,
		}
		parsed = append(parsed, newLbl)
	}
	return &LokiLog{
		Line:     lr.Line,
		Labels:   labels,
		Metadata: metadata,
		Parsed:   parsed,
	}
}
