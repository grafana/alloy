package pipelinetest

import (
	"os"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/pipelinetest/harness"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestLokiPipeline(t *testing.T) {
	type testCase struct {
		name       string
		config     string
		produce    func(t *testing.T, a *harness.Alloy)
		assertions []harness.Assertion
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
			produce: func(t *testing.T, a *harness.Alloy) {
				sink := harness.MustComponent[*harness.Source](t, a, "pipelinetest.source.in")
				sink.SendEntries(
					loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{
						Timestamp: time.Date(2026, time.April, 14, 12, 53, 51, 470999516, time.Local),
						Line:      "test",
					}),
				)
			},
			assertions: []harness.Assertion{
				harness.LokiEntryCount(1),
				harness.LokiEntryMatch(loki.NewEntry(model.LabelSet{"replaced": "bar"}, push.Entry{
					Timestamp: time.Date(2026, time.April, 14, 12, 53, 51, 470999516, time.Local),
					Line:      "test",
				})),
			},
		},
		{
			name: "rename foo label",
			config: `
				loki.source.file "file" {
					targets = [{__path__ = "./*.log"}]

					file_match {
						enabled     = true
						sync_period = "50ms"
					}	

					forward_to = [loki.process.default.receiver]
				}

				loki.process "default" {
					forward_to = [loki.write.default.receiver]

					stage.static_labels {
						values = {
							test = "test",
						}
					}

					stage.label_drop {
						values = ["filename"]
					}
				}

				loki.write "default" {
					endpoint {
						url        = pipelinetest.sink.out.loki_push_url
						batch_wait = "10ms"
					}
				}
			`,
			produce: func(t *testing.T, a *harness.Alloy) {
				require.NoError(t, os.WriteFile("test.log", []byte("line 1\nline 2\n"), 0644))
				t.Cleanup(func() {
					require.NoError(t, os.Remove("test.log"))
				})
			},
			assertions: []harness.Assertion{
				harness.LokiEntryCount(2),
				harness.LokiHasEntry(
					harness.LokiEntryLabels(model.LabelSet{"test": "test"}),
					harness.LokiEntryLine("line 1"),
				),
				harness.LokiHasEntry(
					harness.LokiEntryLabels(model.LabelSet{"test": "test"}),
					harness.LokiEntryLine("line 2"),
				),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alloy := harness.NewAlloy(t, tt.config)
			tt.produce(t, alloy)
			alloy.Assert(tt.assertions...)
		})
	}
}
