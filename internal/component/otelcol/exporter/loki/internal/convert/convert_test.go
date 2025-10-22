package convert_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/loki/internal/convert"
	"github.com/grafana/alloy/internal/component/otelcol/processor/processortest"
	"github.com/grafana/alloy/internal/util"
)

func TestConsumeLogs(t *testing.T) {
	maxTestedLogEntries := 2

	tests := []struct {
		testName        string
		inputLogJson    string
		expectedEntries []loki.Entry
	}{
		{
			testName: "LabelHint",
			inputLogJson: `{
				"resourceLogs": [{
					"scopeLogs": [{
						"log_records": [{
							"timeUnixNano": "1581452773000000111",
							"severityNumber": 9,
							"severityText": "Info",
							"name": "logA",
							"body": { "stringValue": "AUTH log message" },
							"attributes": [{
								"key": "attr.1",
								"value": { "stringValue": "12345" }
							},
							{
								"key": "attr.2",
								"value": { "stringValue": "fake_token" }
							},
							{
								"key": "loki.attribute.labels",
								"value": { "stringValue": "attr.1" }
							}]
						}]
					}]
				}]
			}`,
			expectedEntries: []loki.Entry{
				{
					Labels: map[model.LabelName]model.LabelValue{
						"exporter": model.LabelValue("OTLP"),
						"attr_1":   model.LabelValue("12345"),
						"level":    model.LabelValue("INFO"),
					},
					Entry: push.Entry{
						Timestamp:          time.Unix(0, int64(1581452773000000111)),
						Line:               `{"body":"AUTH log message","severity":"Info","attributes":{"attr.2":"fake_token"}}`,
						StructuredMetadata: nil,
					},
				},
			},
		},
		{
			testName: "NoLabelHint",
			inputLogJson: `{
				"resourceLogs": [{
					"scopeLogs": [{
						"log_records": [{
							"timeUnixNano": "1581452773000000111",
							"severityNumber": 9,
							"severityText": "Info",
							"name": "logA",
							"body": { "stringValue": "AUTH log message" },
							"attributes": [{
								"key": "attr.1",
								"value": { "stringValue": "12345" }
							},
							{
								"key": "attr.2",
								"value": { "stringValue": "fake_token" }
							}]
						}]
					}]
				}]
			}`,
			expectedEntries: []loki.Entry{
				{
					Labels: map[model.LabelName]model.LabelValue{
						"exporter": model.LabelValue("OTLP"),
						"level":    model.LabelValue("INFO"),
					},
					Entry: push.Entry{
						Timestamp:          time.Unix(0, int64(1581452773000000111)),
						Line:               `{"body":"AUTH log message","severity":"Info","attributes":{"attr.1":"12345","attr.2":"fake_token"}}`,
						StructuredMetadata: nil,
					},
				},
			},
		},
		{
			testName: "ScopeAttributeWhenResourceAttributeHasSameName",
			inputLogJson: `{
				"resourceLogs": [{
					"resource": {
						"attributes": [{
							"key": "attr.1",
							"value": { "stringValue": "77777" }
						}]
					},
					"scopeLogs": [{
						"log_records": [{
							"timeUnixNano": "1581452773000000111",
							"severityNumber": 9,
							"severityText": "Info",
							"name": "logA",
							"body": { "stringValue": "AUTH log message" },
							"attributes": [{
								"key": "attr.1",
								"value": { "stringValue": "11111" }
							},
							{
								"key": "attr.2",
								"value": { "stringValue": "fake_token" }
							},
							{
								"key": "loki.attribute.labels",
								"value": { "stringValue": "attr.1" }
							}]
						}]
					}]
				}]
			}`,
			expectedEntries: []loki.Entry{
				{
					Labels: map[model.LabelName]model.LabelValue{
						"exporter": model.LabelValue("OTLP"),
						"attr_1":   model.LabelValue("11111"),
						"level":    model.LabelValue("INFO"),
					},
					Entry: push.Entry{
						Timestamp:          time.Unix(0, int64(1581452773000000111)),
						Line:               `{"body":"AUTH log message","severity":"Info","attributes":{"attr.2":"fake_token"}}`,
						StructuredMetadata: nil,
					},
				},
			},
		},
		{
			testName: "ResourceAttributeWhenScopeAttributeHasSameName",
			inputLogJson: `{
				"resourceLogs": [{
					"resource": {
						"attributes": [{
							"key": "attr.1",
							"value": { "stringValue": "77777" }
						}]
					},
					"scopeLogs": [{
						"log_records": [{
							"timeUnixNano": "1581452773000000111",
							"severityNumber": 9,
							"severityText": "Info",
							"name": "logA",
							"body": { "stringValue": "AUTH log message" },
							"attributes": [{
								"key": "attr.1",
								"value": { "stringValue": "11111" }
							},
							{
								"key": "attr.2",
								"value": { "stringValue": "fake_token" }
							},
							{
								"key": "loki.resource.labels",
								"value": { "stringValue": "attr.1" }
							}]
						}]
					}]
				}]
			}`,
			expectedEntries: []loki.Entry{
				{
					Labels: map[model.LabelName]model.LabelValue{
						"exporter": model.LabelValue("OTLP"),
						"attr_1":   model.LabelValue("77777"),
						"level":    model.LabelValue("INFO"),
					},
					Entry: push.Entry{
						Timestamp:          time.Unix(0, int64(1581452773000000111)),
						Line:               `{"body":"AUTH log message","severity":"Info","attributes":{"attr.2":"fake_token"}}`,
						StructuredMetadata: nil,
					},
				},
			},
		},
		{
			testName: "ResourceAttributeOnly",
			inputLogJson: `{
				"resourceLogs": [{
					"resource": {
						"attributes": [{
							"key": "attr.1",
							"value": { "stringValue": "77777" }
						}]
					},
					"scopeLogs": [{
						"log_records": [{
							"timeUnixNano": "1581452773000000111",
							"severityNumber": 9,
							"severityText": "Info",
							"name": "logA",
							"body": { "stringValue": "AUTH log message" },
							"attributes": [{
								"key": "attr.2",
								"value": { "stringValue": "fake_token" }
							},
							{
								"key": "loki.resource.labels",
								"value": { "stringValue": "attr.1" }
							}]
						}]
					}]
				}]
			}`,
			expectedEntries: []loki.Entry{
				{
					Labels: map[model.LabelName]model.LabelValue{
						"exporter": model.LabelValue("OTLP"),
						"attr_1":   model.LabelValue("77777"),
						"level":    model.LabelValue("INFO"),
					},
					Entry: push.Entry{
						Timestamp:          time.Unix(0, int64(1581452773000000111)),
						Line:               `{"body":"AUTH log message","severity":"Info","attributes":{"attr.2":"fake_token"}}`,
						StructuredMetadata: nil,
					},
				},
			},
		},
		{
			testName: "MultipleLogs",
			inputLogJson: `{
				"resourceLogs": [{
					"scopeLogs": [{
						"log_records": [{
							"timeUnixNano": "1581452773000000111",
							"severityNumber": 9,
							"severityText": "Info",
							"name": "logA",
							"body": { "stringValue": "AUTH log message" },
							"attributes": [{
								"key": "attr.1",
								"value": { "stringValue": "12345" }
							},
							{
								"key": "attr.2",
								"value": { "stringValue": "fake_token" }
							}]
						}]
					},
					{
						"log_records": [{
							"timeUnixNano": "1581452773000000211",
							"severityNumber": 9,
							"severityText": "Info",
							"name": "logA",
							"body": { "stringValue": "Another AUTH log message" },
							"attributes": [{
								"key": "attr.1",
								"value": { "stringValue": "12345" }
							},
							{
								"key": "attr.2",
								"value": { "stringValue": "fake_token" }
							}]
						}]
					}]
				}]
			}`,
			expectedEntries: []loki.Entry{
				{
					Labels: map[model.LabelName]model.LabelValue{
						"exporter": model.LabelValue("OTLP"),
						"level":    model.LabelValue("INFO"),
					},
					Entry: push.Entry{
						Timestamp:          time.Unix(0, int64(1581452773000000111)),
						Line:               `{"body":"AUTH log message","severity":"Info","attributes":{"attr.1":"12345","attr.2":"fake_token"}}`,
						StructuredMetadata: nil,
					},
				},
				{
					Labels: map[model.LabelName]model.LabelValue{
						"exporter": model.LabelValue("OTLP"),
						"level":    model.LabelValue("INFO"),
					},
					Entry: push.Entry{
						Timestamp:          time.Unix(0, int64(1581452773000000211)),
						Line:               `{"body":"Another AUTH log message","severity":"Info","attributes":{"attr.1":"12345","attr.2":"fake_token"}}`,
						StructuredMetadata: nil,
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			logger := util.TestAlloyLogger(t)
			promReg := prometheus.NewRegistry()
			receiver := loki.NewLogsReceiverWithChannel(make(chan loki.Entry, maxTestedLogEntries))

			converter := convert.New(logger, promReg, []loki.LogsReceiver{receiver})

			ctx := t.Context()

			log := processortest.CreateTestLogs(tc.inputLogJson)

			require.NoError(t, converter.ConsumeLogs(ctx, log))
			ctx.Done()
			close(receiver.Chan())

			receivedEntries := 0
			for _, expectedEntry := range tc.expectedEntries {
				for entry := range receiver.Chan() {
					compareLokiEntries(t, &expectedEntry, &entry)
					receivedEntries += 1
					break
				}
			}
			require.Equal(t, receivedEntries, len(tc.expectedEntries))
		})
	}
}

// Compare two loki entries by converting them to json strings.
func compareLokiEntries(t *testing.T, expectedEntry, actualEntry *loki.Entry) {
	expectedStream := entryToStream(expectedEntry)
	expectedBuf, err := json.Marshal(expectedStream)
	require.NoError(t, err)

	actualStream := entryToStream(actualEntry)
	actualBuf, err := json.Marshal(actualStream)
	require.NoError(t, err)

	require.JSONEq(t, string(expectedBuf), string(actualBuf))
}

// Convert loki entry to a loki stream.
func entryToStream(entry *loki.Entry) push.Stream {
	return push.Stream{
		Labels: entry.Labels.String(),
		Entries: []push.Entry{
			entry.Entry,
		},
	}
}
