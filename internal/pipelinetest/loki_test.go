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
		entryPoints  []string
		inputEntries []loki.Entry
		assertions   []harness.LokiAssertion
	}

	tests := []testCase{
		{
			name: "rename foo label",
			config: `
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
			entryPoints: []string{"loki.relabel.default.receiver"},
			inputEntries: []loki.Entry{
				loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{
					Timestamp: time.Date(2026, time.April, 14, 12, 53, 51, 470999516, time.Local),
					Line:      "test",
				}),
			},
			assertions: []harness.LokiAssertion{
				harness.LokiEntryMatch(loki.NewEntry(model.LabelSet{"replaced": "bar"}, push.Entry{
					Timestamp: time.Date(2026, time.April, 14, 12, 53, 51, 470999516, time.Local),
					Line:      "test",
				})),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alloy := harness.NewAlloy(t, harness.Options{
				Config:          tt.config,
				LogsEntryPoints: tt.entryPoints,
			})
			alloy.SendEntries(tt.inputEntries...)
			alloy.AssertLoki(tt.assertions...)
		})
	}
}
