package stages

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
)

const criTestTS = "2019-05-07T18:57:50.904275087+00:00"

type recordingConsumer struct {
	mut sync.Mutex

	calls [][]loki.Entry

	failOnCall int
	failErr    error
}

var _ loki.Consumer = (*recordingConsumer)(nil)

func (r *recordingConsumer) Consume(_ context.Context, batch loki.Batch) error {
	r.mut.Lock()
	defer r.mut.Unlock()

	var got []loki.Entry
	_ = batch.ConsumeStreams(func(stream loki.Stream, created int64) error {
		for _, e := range stream.Entries {
			got = append(got, loki.NewEntryWithCreatedUnixMicro(stream.Labels.Clone(), created, e))
		}
		return nil
	})
	r.calls = append(r.calls, got)

	if r.failOnCall != 0 && len(r.calls) == r.failOnCall {
		return r.failErr
	}
	return nil
}

func (r *recordingConsumer) allLines() []string {
	r.mut.Lock()
	defer r.mut.Unlock()

	var out []string
	for _, call := range r.calls {
		for _, e := range call {
			out = append(out, e.Line)
		}
	}
	return out
}

func (r *recordingConsumer) callCount() int {
	r.mut.Lock()
	defer r.mut.Unlock()
	return len(r.calls)
}

func newCRIPipelineConsumer(t testing.TB, cfg CRIConfig, consumer loki.Consumer) *PipelineConsumer {
	t.Helper()
	pc, err := NewPipelineConsumer(
		logging.NewSlogNop(),
		prometheus.NewRegistry(),
		featuregate.StabilityGenerallyAvailable,
		[]StageConfig{{CRIConfig: &cfg}},
		consumer,
	)
	require.NoError(t, err)
	return pc
}

func criTestStream(labels model.LabelSet, lines ...string) loki.Stream {
	entries := make([]push.Entry, 0, len(lines))
	for _, l := range lines {
		entries = append(entries, push.Entry{Timestamp: time.Now(), Line: l})
	}
	return loki.NewStream(labels, entries...)
}

func TestPipelineConsumer_MultiStreamBatch(t *testing.T) {
	rc := &recordingConsumer{}
	pc := newCRIPipelineConsumer(t, defaultCRIConfig, rc)

	batch := loki.NewBatch()
	batch.Add(criTestStream(model.LabelSet{"app": "a"},
		criTestTS+" stdout F a-one",
		criTestTS+" stdout F a-two",
	))
	batch.Add(criTestStream(model.LabelSet{"app": "b"},
		criTestTS+" stderr F b-one",
	))

	require.NoError(t, pc.Consume(context.Background(), batch))

	require.ElementsMatch(t, []string{"a-one", "a-two", "b-one"}, rc.allLines(),
		"every full line should be parsed and forwarded")

	var sawStdout, sawStderr bool
	for _, call := range rc.calls {
		for _, e := range call {
			switch e.Line {
			case "a-one", "a-two":
				require.Equal(t, model.LabelValue("a"), e.Labels["app"])
				require.Equal(t, model.LabelValue("stdout"), e.Labels[criStream])
				sawStdout = true
			case "b-one":
				require.Equal(t, model.LabelValue("b"), e.Labels["app"])
				require.Equal(t, model.LabelValue("stderr"), e.Labels[criStream])
				sawStderr = true
			}
		}
	}
	require.True(t, sawStdout && sawStderr, "expected both streams to be observed")
}

func TestPipelineConsumer_EmptyBatch(t *testing.T) {
	rc := &recordingConsumer{}
	pc := newCRIPipelineConsumer(t, defaultCRIConfig, rc)

	require.NoError(t, pc.Consume(context.Background(), loki.NewBatch()))
	require.Zero(t, rc.callCount(), "an empty batch should not reach the downstream consumer")
}

func TestPipelineConsumer_NonCRILinesPassThrough(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
	}{
		{name: "cri line last", lines: []string{"plain line", criTestTS + " stdout F cri-line"}},
		{name: "cri line first", lines: []string{criTestTS + " stdout F cri-line", "plain line"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rc := &recordingConsumer{}
			pc := newCRIPipelineConsumer(t, defaultCRIConfig, rc)

			batch := loki.NewBatch()
			batch.Add(criTestStream(model.LabelSet{"app": "a"}, tc.lines...))

			require.NoError(t, pc.Consume(context.Background(), batch))
			require.ElementsMatch(t, []string{"plain line", "cri-line"}, rc.allLines())

			for _, call := range rc.calls {
				for _, e := range call {
					require.Equal(t, model.LabelValue("a"), e.Labels["app"])
					switch e.Line {
					case "cri-line":
						require.Equal(t, model.LabelValue("stdout"), e.Labels[criStream],
							"the CRI line should gain the stream label")
					case "plain line":
						_, ok := e.Labels[criStream]
						require.False(t, ok,
							"the non-CRI line must not gain the stream label from its neighbour")
					}
				}
			}
		})
	}
}

func TestPipelineConsumer_PartialLinesAcrossCalls(t *testing.T) {
	rc := &recordingConsumer{}
	pc := newCRIPipelineConsumer(t, defaultCRIConfig, rc)

	labels := model.LabelSet{"app": "a"}

	first := loki.NewBatch()
	first.Add(criTestStream(labels.Clone(), criTestTS+" stdout P part-one "))
	require.NoError(t, pc.Consume(context.Background(), first))
	require.Zero(t, rc.callCount(), "a lone partial line should be buffered, not forwarded")

	second := loki.NewBatch()
	second.Add(criTestStream(labels.Clone(), criTestTS+" stdout F part-two"))
	require.NoError(t, pc.Consume(context.Background(), second))

	require.Equal(t, []string{"part-one part-two"}, rc.allLines(),
		"the buffered partial line should be merged with the full line from the next call")
}

func TestPipelineConsumer_StopFlushesBufferedPartialLines(t *testing.T) {
	rc := &recordingConsumer{}
	pc := newCRIPipelineConsumer(t, defaultCRIConfig, rc)

	batch := loki.NewBatch()
	batch.Add(criTestStream(model.LabelSet{"app": "a"}, criTestTS+" stdout P never-finished "))
	require.NoError(t, pc.Consume(context.Background(), batch))
	require.Zero(t, rc.callCount())

	pc.Stop()

	require.Equal(t, []string{"never-finished "}, rc.allLines(),
		"Stop should flush partial lines still held in memory")
}

func TestPipelineConsumer_OneDownstreamBatchPerStream(t *testing.T) {
	rc := &recordingConsumer{}
	pc := newCRIPipelineConsumer(t, defaultCRIConfig, rc)

	const streams = 4
	batch := loki.NewBatch()
	for i := range streams {
		batch.Add(criTestStream(
			model.LabelSet{"s": model.LabelValue(fmt.Sprintf("%d", i))},
			criTestTS+fmt.Sprintf(" stdout F line-%d", i),
		))
	}
	require.Equal(t, streams, batch.StreamLen())

	require.NoError(t, pc.Consume(context.Background(), batch))

	require.Len(t, rc.allLines(), streams, "no entry should be lost")
	require.Equal(t, streams, rc.callCount(),
		"one input batch of %d streams currently produces %d downstream batches", streams, streams)
}

func TestPipelineConsumer_ErrorDropsRemainingStreams(t *testing.T) {
	wantErr := errors.New("downstream is unhappy")
	rc := &recordingConsumer{failOnCall: 1, failErr: wantErr}
	pc := newCRIPipelineConsumer(t, defaultCRIConfig, rc)

	const streams = 4
	batch := loki.NewBatch()
	for i := range streams {
		batch.Add(criTestStream(
			model.LabelSet{"s": model.LabelValue(fmt.Sprintf("%d", i))},
			criTestTS+fmt.Sprintf(" stdout F line-%d", i),
		))
	}

	err := pc.Consume(context.Background(), batch)
	require.ErrorIs(t, err, wantErr, "the downstream error should be propagated")

	require.Len(t, rc.allLines(), streams,
		"every stream in the batch should be offered downstream even when an earlier stream fails, "+
			"otherwise the remaining streams are dropped silently and the batch has already been reset")
}
