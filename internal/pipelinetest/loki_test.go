package pipelinetest

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/pipelinetest/harness"
)

func TestLokiPipeline(t *testing.T) {
	alloy := harness.NewAlloy(t, `
		pipelinetest.source "in" {
			forward_to {
				logs = [loki.relabel.default.receiver]
			}
		}

		loki.relabel "default" {
			forward_to = [loki.write.default.receiver]

			rule {
				source_labels = ["foo"]
				target_label = "replaced"
			}
	
			rule {
				action = "labeldrop"	
				regex  = "^foo$"
				
			}
		}

		loki.write "default" {
			endpoint {
				url        = pipelinetest.sink.out.loki_push_url
				batch_wait = "10ms"
			}
		}

		pipelinetest.sink "out" {}
	`)

	now := time.Now()

	source := harness.MustComponent[*harness.Source](t, alloy, "pipelinetest.source.in")
	source.LokiFanout.Send(context.Background(), loki.NewEntry(
		model.LabelSet{"foo": "bar"},
		push.Entry{
			Timestamp: now,
			Line:      "test",
		},
	))

	sink := harness.MustComponent[*harness.Sink](t, alloy, "pipelinetest.sink.out")
	sink.AssertEntries(t, loki.NewEntry(model.LabelSet{"replaced": "bar"}, push.Entry{
		Timestamp: now,
		Line:      "test",
	}))
}
