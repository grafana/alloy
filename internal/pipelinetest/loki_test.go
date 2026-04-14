package pipelinetest

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/pipelinetest/harness"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
)

func TestLokiPipeline(t *testing.T) {
	type testCase struct {
		name         string
		config       string
		inputEntries []loki.Entry
		assertions   []harness.Assertion
	}

	tests := []testCase{
		{
			name: "rename foo label",
			config: `
				pipelinetest.source "in" {
					forward_to {
						logs = [loki.relabel.default.receiver]
					}
				}

				loki.relabel "default" {
					forward_to = [loki.write.default.receiver]

					rule {
						source_labels = ["foo"]
						target_label  = "replaced"
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
			`,
			inputEntries: []loki.Entry{
				loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{
					Timestamp: time.Date(2026, time.April, 14, 12, 53, 51, 470999516, time.Local),
					Line:      "test",
				}),
			},
			assertions: []harness.Assertion{
				harness.LokiEntryCount(1),
				harness.LokiEntryMatch(loki.NewEntry(model.LabelSet{"replaced": "bar"}, push.Entry{
					Timestamp: time.Date(2026, time.April, 14, 12, 53, 51, 470999516, time.Local),
					Line:      "test",
				})),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alloy := harness.NewAlloy(t, tt.config)
			sink := harness.MustComponent[*harness.Source](t, alloy, "pipelinetest.source.in")
			sink.SendEntries(tt.inputEntries...)
			alloy.Assert(tt.assertions...)
		})
	}
}
