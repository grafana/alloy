package process

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/lokitest"
	"github.com/grafana/alloy/internal/component/loki/process/stages"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/service/lokipipelinev2"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

func TestComponent(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	type testCase struct {
		name          string
		cfg           string
		inputs        []loki.Entry
		expected      []loki.Entry
		unorderedFrom *int
	}

	var (
		// Use entryTs for push.Entry.Timestamp values set directly on test inputs
		// and for expected outputs from stages that do not rewrite timestamps.
		entryTs    = time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
		getEntryTS = func(pos int) time.Time { return entryTs.Add(time.Duration(pos) * time.Second) }

		// Use ts for timestamps encoded into log lines and later parsed back out
		// by stages such as cri, docker, and multiline.
		ts       = time.Date(2025, time.February, 3, 4, 5, 6, 0, time.UTC)
		getTS    = func(pos int) time.Time { return ts.Add(time.Duration(pos) * time.Second) }
		formatTs = func(pos int, layout string) string { return getTS(pos).Format(layout) }
	)

	tests := []testCase{
		{
			name: "cri pipeline",
			cfg: `
			forward_to = []

			stage.cri {}

			stage.structured_metadata {
					values = {
						filename = "",
						stream = "",
					}
				}

			stage.static_labels {
				values = {
					foo = "bar",
				}
			}
			`,
			inputs: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "file1"}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line:      formatTs(0, time.RFC3339Nano) + " stdout P partial for file 1 stdout",
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "file1"}, 0, push.Entry{
					Timestamp: getEntryTS(1),
					Line:      formatTs(0, time.RFC3339Nano) + " stderr P partial for file 1 stderr",
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "file2"}, 0, push.Entry{
					Timestamp: getEntryTS(2),
					Line:      formatTs(0, time.RFC3339Nano) + " stdout P partial for file2",
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "file1"}, 0, push.Entry{
					Timestamp: getEntryTS(3),
					Line:      formatTs(1, time.RFC3339Nano) + " stderr F full file 1 stderr",
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "file2"}, 0, push.Entry{
					Timestamp: getEntryTS(4),
					Line:      formatTs(1, time.RFC3339Nano) + " stderr F full file 2",
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "file1"}, 0, push.Entry{
					Timestamp: getEntryTS(5),
					Line:      formatTs(1, time.RFC3339Nano) + " stderr F full file 1 stdout",
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "file3"}, 0, push.Entry{
					Timestamp: getEntryTS(6),
					Line:      "not a cri line",
				}),
			},
			expected: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"foo": "bar"}, 0, push.Entry{
					Timestamp: getTS(1),
					Line:      "partial for file 1 stderrfull file 1 stderr",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "file1"},
						{Name: "stream", Value: "stderr"},
					},
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"foo": "bar"}, 0, push.Entry{
					Timestamp: getTS(1),
					Line:      "full file 2",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "file2"},
						{Name: "stream", Value: "stderr"},
					},
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"foo": "bar"}, 0, push.Entry{
					Timestamp: getTS(1),
					Line:      "full file 1 stdout",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "file1"},
						{Name: "stream", Value: "stderr"},
					},
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"foo": "bar"}, 0, push.Entry{
					Timestamp: getEntryTS(6),
					Line:      "not a cri line",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "file3"},
					},
				}),
			},
		},
		{
			name: "docker pipeline",
			cfg: `
			forward_to = []

			stage.docker {}

			stage.structured_metadata {
					values = {
						filename = "",
						stream = "",
					}
				}

			stage.structured_metadata_drop {
				values = ["stream"]
			}

			stage.static_labels {
				values = {
					foo = "bar",
				}
			}
			`,
			inputs: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "docker.log"}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line: mustMarshalJSON(t, map[string]string{
						"log":    "docker stdout line\n",
						"stream": "stdout",
						"time":   formatTs(0, time.RFC3339Nano),
					}),
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "docker.log"}, 0, push.Entry{
					Timestamp: getEntryTS(1),
					Line: mustMarshalJSON(t, map[string]string{
						"log":    "docker stderr line\n",
						"stream": "stderr",
						"time":   formatTs(1, time.RFC3339Nano),
					}),
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "docker.log"}, 0, push.Entry{
					Timestamp: getEntryTS(2),
					Line:      `{"msg":"not docker format"}`,
				}),
			},
			expected: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"foo": "bar"}, 0, push.Entry{
					Timestamp: getTS(0),
					Line:      "docker stdout line\n",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "docker.log"},
					},
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"foo": "bar"}, 0, push.Entry{
					Timestamp: getTS(1),
					Line:      "docker stderr line\n",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "docker.log"},
					},
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"foo": "bar"}, 0, push.Entry{
					Timestamp: getEntryTS(2),
					Line:      `{"msg":"not docker format"}`,
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "docker.log"},
					},
				}),
			},
		},
		{
			name: "json pipeline",
			cfg: `
			forward_to = []

			stage.json {
				expressions = {
					msg = "",
					app = "",
					service_name = "",
				}
				drop_malformed = true
			}

			stage.labels {
				values = {
					service_name = "",
				}
			}

			stage.structured_metadata {
				values = {
					filename = "",
					app = "",
				}
			}

			stage.static_labels {
				values = {
					foo = "bar",
				}
			}

			stage.output {
				source = "msg"
			}
			`,
			inputs: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "json.log"}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line: mustMarshalJSON(t, map[string]string{
						"msg":          "json line 1",
						"app":          "api",
						"service_name": "service-a",
					}),
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "json.log"}, 0, push.Entry{
					Timestamp: getEntryTS(1),
					Line:      `not json`,
				}),
			},
			expected: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"foo": "bar", "service_name": "service-a"}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line:      "json line 1",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "json.log"},
						{Name: "app", Value: "api"},
					},
				}),
			},
		},
		{
			name: "template pipeline",
			cfg: `
			forward_to = []

			stage.json {
				expressions = {
					app = "",
					level = "",
					msg = "",
				}
			}

			stage.template {
				source   = "app"
				template = "{{ .Value | ToUpper }} doki"
			}

			stage.template {
				source   = "level"
				template = "{{ if eq .Value \"WARN\" }}{{ Replace .Value \"WARN\" \"OK\" -1 }}{{ else }}{{ .Value }}{{ end }}"
			}

			stage.template {
				source   = "type"
				template = "{{ .app }}-{{ .level }}"
			}

			stage.labels {
				values = {
					app = "",
					level = "",
					type = "",
				}
			}

			stage.output {
				source = "msg"
			}
			`,
			inputs: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "template.log"}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line: mustMarshalJSON(t, map[string]string{
						"app":   "loki",
						"level": "WARN",
						"msg":   "template line",
					}),
				}),
			},
			expected: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{
					"filename": "template.log",
					"app":      "LOKI doki",
					"level":    "OK",
					"type":     "LOKI doki-OK",
				}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line:      "template line",
				}),
			},
		},
		{
			name: "match pipeline",
			cfg: `
			forward_to = []

			stage.match {
				selector = "{filename=\"match-1.log\"}"

				stage.static_labels {
					values = {
						match = "one",
					}
				}
			}

			stage.match {
				selector = "{filename=\"match-2.log\"} |= \"svc=\""

				stage.logfmt {
					mapping = {
						msg = "",
						svc = "",
					}
				}

				stage.labels {
					values = {
						svc = "",
					}
				}

				stage.output {
					source = "msg"
				}
			}

			stage.match {
				selector = "{filename=\"match-3.log\"} |= \"prefix: \""

				stage.regex {
					expression = "^prefix: (?P<body>.*)$"
				}

				stage.structured_metadata {
					values = {
						body = "",
					}
				}
			}
			`,
			inputs: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "match-1.log"}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line:      `first`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "match-2.log"}, 0, push.Entry{
					Timestamp: getEntryTS(1),
					Line:      `msg="second" svc=api`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "match-3.log"}, 0, push.Entry{
					Timestamp: getEntryTS(2),
					Line:      `prefix: third`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "match-3.log"}, 0, push.Entry{
					Timestamp: getEntryTS(3),
					Line:      `no match`,
				}),
			},
			expected: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "match-1.log", "match": "one"}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line:      `first`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "match-2.log", "svc": "api"}, 0, push.Entry{
					Timestamp: getEntryTS(1),
					Line:      `second`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "match-3.log"}, 0, push.Entry{
					Timestamp: getEntryTS(2),
					Line:      `prefix: third`,
					StructuredMetadata: push.LabelsAdapter{
						{Name: "body", Value: "third"},
					},
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "match-3.log"}, 0, push.Entry{
					Timestamp: getEntryTS(3),
					Line:      `no match`,
				}),
			},
			// NOTE: we cannot guarantee ordering between different sub-pipelines.
			unorderedFrom: new(0),
		},
		{
			name: "logfmt pipeline",
			cfg: `
			forward_to = []

			stage.logfmt {
				mapping = {
					msg = "",
					app = "",
					service_name = "",
				}
			}

			stage.labels {
				values = {
					service_name = "",
				}
			}

			stage.structured_metadata {
				values = {
					filename = "",
					app = "",
				}
			}

			stage.static_labels {
				values = {
					foo = "bar",
				}
			}

			stage.output {
				source = "msg"
			}
			`,
			inputs: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "logfmt.log"}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line:      `msg="logfmt line 1" app=api service_name=service-b`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "logfmt.log"}, 0, push.Entry{
					Timestamp: getEntryTS(1),
					Line:      `not logfmt`,
				}),
			},
			expected: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"foo": "bar", "service_name": "service-b"}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line:      "logfmt line 1",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "logfmt.log"},
						{Name: "app", Value: "api"},
					},
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"foo": "bar"}, 0, push.Entry{
					Timestamp: getEntryTS(1),
					Line:      `not logfmt`,
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "logfmt.log"},
					},
				}),
			},
		},
		{
			name: "label_drop and label_keep pipeline",
			cfg: `
			forward_to = []

			stage.static_labels {
				values = { "foo" = "fooval", "bar" = "barval", "baz" = "bazval", "qux" = "quxval" }
			}

			stage.label_drop {
				values = [ "foo", "bar" ]
			}

			stage.label_keep {
				values = [ "foo", "baz", "filename" ]
			}
			`,
			inputs: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "/var/log/pods/agent/agent/1.log", "env": "dev"}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line:      "line",
				}),
			},
			expected: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "/var/log/pods/agent/agent/1.log", "baz": "bazval"}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line:      "line",
				}),
			},
		},
		{
			name: "regex pipeline",
			cfg: `
			forward_to = []

			stage.regex {
				expression = "^(?s)(?P<time>\\S+?) (?P<stream>stdout|stderr) (?P<content>.*)$"
			}

			stage.timestamp {
				source = "time"
				format = "RFC3339"
			}

			stage.output {
				source = "content"
			}

			stage.labels {
				values = { src = "stream" }
			}
			`,
			inputs: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "/var/log/pods/agent/agent/1.log", "foo": "bar"}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line:      formatTs(0, time.RFC3339) + ` stderr somewhere, somehow, an error occurred`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "/var/log/pods/agent/agent/2.log", "foo": "bar"}, 0, push.Entry{
					Timestamp: getEntryTS(1),
					Line:      `not matching`,
				}),
			},
			expected: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "/var/log/pods/agent/agent/1.log", "foo": "bar", "src": "stderr"}, 0, push.Entry{
					Timestamp: mustParseTime(t, time.RFC3339, formatTs(0, time.RFC3339)),
					Line:      `somewhere, somehow, an error occurred`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "/var/log/pods/agent/agent/2.log", "foo": "bar"}, 0, push.Entry{
					Timestamp: getEntryTS(1),
					Line:      `not matching`,
				}),
			},
		},
		{
			name: "drop pipeline",
			cfg: `
			forward_to = []

			stage.drop {
				expression = "drop"
			}

			stage.match {
				selector = "{filename=\"drop.log\"} |= \"match me\""
				action   = "drop"
			}

			stage.limit {
				rate  = 1
				burst = 1
				drop  = true
			}

			`,
			inputs: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "drop.log"}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line:      `drop me`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "drop.log"}, 0, push.Entry{
					Timestamp: getEntryTS(1),
					Line:      `match me`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "drop.log"}, 0, push.Entry{
					Timestamp: getEntryTS(2),
					Line:      `keep me`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "drop.log"}, 0, push.Entry{
					Timestamp: getEntryTS(3),
					Line:      `limit me`,
				}),
			},
			expected: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "drop.log"}, 0, push.Entry{
					Timestamp: getEntryTS(2),
					Line:      `keep me`,
				}),
			},
		},
		{
			name: "multiline pipeline",
			cfg: `
			forward_to = []

			stage.multiline {
				firstline     = "^\\[\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}\\]"
				max_wait_time = "2s"
			}

			stage.regex {
				expression = "^\\[(?P<time>\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2})\\] (?P<msg>(?s:.*))$"
			}

			stage.timestamp {
				source = "time"
				format = "2006-01-02 15:04:05"
			}

			stage.structured_metadata {
				values = {
					filename = "",
				}
			}

			stage.static_labels {
				values = {
					foo = "bar",
				}
			}

			stage.output {
				source = "msg"
			}
			`,
			inputs: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "multiline.log"}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line:      `not a multiline start`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "multiline.log"}, 0, push.Entry{
					Timestamp: getEntryTS(1),
					Line:      "[" + formatTs(1, "2006-01-02 15:04:05") + `] "GET /one HTTP/1.1" 200 -`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "multiline.log"}, 0, push.Entry{
					Timestamp: getEntryTS(2),
					Line:      "[" + formatTs(2, "2006-01-02 15:04:05") + `] "POST /two HTTP/1.1" 201 -`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "multiline.log"}, 0, push.Entry{
					Timestamp: getEntryTS(3),
					Line:      `line 1`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "other-multiline.log"}, 0, push.Entry{
					Timestamp: getEntryTS(4),
					Line:      "[" + formatTs(4, "2006-01-02 15:04:05") + `] "PATCH /other HTTP/1.1" 204 -`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "multiline.log"}, 0, push.Entry{
					Timestamp: getEntryTS(5),
					Line:      `line 2`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "multiline.log"}, 0, push.Entry{
					Timestamp: getEntryTS(6),
					Line:      "[" + formatTs(6, "2006-01-02 15:04:05") + `] "PUT /three HTTP/1.1" 202 -`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "other-multiline.log"}, 0, push.Entry{
					Timestamp: getEntryTS(7),
					Line:      `other line 1`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "multiline.log"}, 0, push.Entry{
					Timestamp: getEntryTS(8),
					Line:      `line 1`,
				}),
			},
			expected: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"foo": "bar"}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line:      `not a multiline start`,
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "multiline.log"},
					},
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"foo": "bar"}, 0, push.Entry{
					Timestamp: getTS(1).Truncate(time.Second),
					Line:      `"GET /one HTTP/1.1" 200 -`,
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "multiline.log"},
					},
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"foo": "bar"}, 0, push.Entry{
					Timestamp: getTS(2).Truncate(time.Second),
					Line:      "\"POST /two HTTP/1.1\" 201 -\nline 1\nline 2",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "multiline.log"},
					},
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"foo": "bar"}, 0, push.Entry{
					Timestamp: getTS(4).Truncate(time.Second),
					Line:      "\"PATCH /other HTTP/1.1\" 204 -\nother line 1",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "other-multiline.log"},
					},
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"foo": "bar"}, 0, push.Entry{
					Timestamp: getTS(6).Truncate(time.Second),
					Line:      "\"PUT /three HTTP/1.1\" 202 -\nline 1",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "multiline.log"},
					},
				}),
			},
			unorderedFrom: new(3),
		},
		{
			name: "pattern pipeline parsing nginx",
			cfg: `
			forward_to = []

			stage.pattern {
				pattern = "<_> <_> <user> [<timestamp>] \"<method> <path> <protocol>\" <status> <size> \"<referer>\" \"<useragent>\""
			}

			stage.timestamp {
				source = "timestamp"
				format = "02/Jan/2006:15:04:05 -0700"
			}

			stage.labels {
				values = {
					method = "",
					status = "",
				}
			}

			stage.structured_metadata {
				values = {
					filename = "",
					path = "",
					user = "",
				}
			}

			stage.output {
				source = "path"
			}
			`,
			inputs: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "access.log"}, 0, push.Entry{
					Timestamp: getEntryTS(0),
					Line:      `11.11.11.11 - frank [` + formatTs(0, "02/Jan/2006:15:04:05 -0700") + `] "GET /index.html HTTP/1.1" 200 932 "-" "test-agent"`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "access.log"}, 0, push.Entry{
					Timestamp: getEntryTS(1),
					Line:      `22.22.22.22 - mary [` + formatTs(1, "02/Jan/2006:15:04:05 -0700") + `] "POST /api/users HTTP/1.1" 201 128 "-" "api-client"`,
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"filename": "access.log"}, 0, push.Entry{
					Timestamp: getEntryTS(2),
					Line:      `33.33.33.33 - jane [` + formatTs(2, "02/Jan/2006:15:04:05 -0700") + `] "DELETE /api/users/42 HTTP/2.0" 204 0 "-" "cleanup-bot"`,
				}),
			},
			expected: []loki.Entry{
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"method": "GET", "status": "200"}, 0, push.Entry{
					Timestamp: mustParseTime(t, "02/Jan/2006:15:04:05 -0700", formatTs(0, "02/Jan/2006:15:04:05 -0700")),
					Line:      "/index.html",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "access.log"},
						{Name: "path", Value: "/index.html"},
						{Name: "user", Value: "frank"},
					},
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"method": "POST", "status": "201"}, 0, push.Entry{
					Timestamp: mustParseTime(t, "02/Jan/2006:15:04:05 -0700", formatTs(1, "02/Jan/2006:15:04:05 -0700")),
					Line:      "/api/users",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "access.log"},
						{Name: "path", Value: "/api/users"},
						{Name: "user", Value: "mary"},
					},
				}),
				loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"method": "DELETE", "status": "204"}, 0, push.Entry{
					Timestamp: mustParseTime(t, "02/Jan/2006:15:04:05 -0700", formatTs(2, "02/Jan/2006:15:04:05 -0700")),
					Line:      "/api/users/42",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "filename", Value: "access.log"},
						{Name: "path", Value: "/api/users/42"},
						{Name: "user", Value: "jane"},
					},
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lokitest.Test(t, func(t *testing.T, env lokitest.Env) {
				recv1, recv2 := env.NewReceiver(t), env.NewReceiver(t)

				var args Arguments
				require.NoError(t, syntax.Unmarshal([]byte(tt.cfg), &args))
				args.ForwardTo = []loki.LogsReceiver{recv1, recv2}

				ctrl, stop := env.Run(t, "loki.process", args)
				defer stop()

				// One send per entry keeps the input order on both pipelines, a v2
				// batch would group entries by stream.
				recv := ctrl.Exports().(Exports).Receiver
				for _, input := range tt.inputs {
					require.NoError(t, env.Send(t.Context(), recv, input))
				}

				if tt.unorderedFrom == nil {
					tt.unorderedFrom = new(len(tt.expected))
				}

				assert := func(t *testing.T, unorderedFrom int, expected, got []loki.Entry) {
					for i := range unorderedFrom {
						assertEntry(t, expected[i], got[i])
					}
					assertEntriesUnordered(t, expected[unorderedFrom:], got[unorderedFrom:])
				}

				assert(t, *tt.unorderedFrom, tt.expected, recv1.Next(t, len(tt.expected)))
				assert(t, *tt.unorderedFrom, tt.expected, recv2.Next(t, len(tt.expected)))
			})
		})
	}
}

func mustMarshalJSON(t *testing.T, v any) string {
	t.Helper()

	data, err := json.Marshal(v)
	require.NoError(t, err)

	return string(data)
}

func mustParseTime(t *testing.T, layout, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(layout, value)
	require.NoError(t, err)

	return parsed
}

func assertEntry(t *testing.T, expected, actual loki.Entry) {
	t.Helper()

	require.Equal(t, expected.Line, actual.Line)
	require.True(t, expected.Timestamp.Equal(actual.Timestamp))
	require.EqualValues(t, expected.Labels, actual.Labels)
	require.ElementsMatch(t, expected.StructuredMetadata, actual.StructuredMetadata)
}

func assertEntriesUnordered(t *testing.T, expected, actual []loki.Entry) {
	t.Helper()

	require.Len(t, actual, len(expected))

	entriesEqual := func(expected, actual loki.Entry) bool {
		if expected.Line != actual.Line {
			return false
		}
		if !expected.Timestamp.Equal(actual.Timestamp) {
			return false
		}
		if !reflect.DeepEqual(expected.Labels, actual.Labels) {
			return false
		}

		var (
			actualStructured   = slices.Clone(actual.StructuredMetadata)
			expectedStructured = slices.Clone(expected.StructuredMetadata)
		)

		slices.SortFunc(expectedStructured, func(a, b push.LabelAdapter) int {
			if a.Name != b.Name {
				return strings.Compare(a.Name, b.Name)
			}
			return strings.Compare(a.Value, b.Value)
		})

		slices.SortFunc(actualStructured, func(a, b push.LabelAdapter) int {
			if a.Name != b.Name {
				return strings.Compare(a.Name, b.Name)
			}
			return strings.Compare(a.Value, b.Value)
		})

		return reflect.DeepEqual(expectedStructured, actualStructured)
	}

	remaining := append([]loki.Entry(nil), actual...)
	for _, exp := range expected {
		found := -1
		for i, got := range remaining {
			if entriesEqual(exp, got) {
				found = i
				break
			}
		}

		require.NotEqual(t, -1, found)
		remaining = append(remaining[:found], remaining[found+1:]...)
	}
}

func TestComponentV2(t *testing.T) {
	lokitest.TestV2(t, func(t *testing.T, env lokitest.Env) {
		t.Run("stop cancels call blocked downstream", func(t *testing.T) {
			entered := make(chan struct{})
			out := loki.NewLogsReceiverV2(lokitest.ConsumerFunc(func(ctx context.Context, _ loki.Batch) error {
				close(entered)
				<-ctx.Done()
				return ctx.Err()
			}), "test")

			ctrl, stop := env.Run(t, "loki.process", Arguments{ForwardTo: []loki.LogsReceiver{out}})
			recv := ctrl.Exports().(Exports).Receiver

			errc := make(chan error, 1)
			go func() {
				errc <- env.Send(t.Context(), recv, loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"app": "test"}, 0, push.Entry{
					Timestamp: time.Now(),
					Line:      "line",
				}))
			}()
			<-entered

			stop()

			require.ErrorIs(t, <-errc, loki.ErrConsumerStopped)
		})

		t.Run("stage change cancels call blocked downstream", func(t *testing.T) {
			entered := make(chan struct{})
			blocking := loki.NewLogsReceiverV2(lokitest.ConsumerFunc(func(ctx context.Context, _ loki.Batch) error {
				close(entered)
				<-ctx.Done()
				return ctx.Err()
			}), "test")

			ctrl, _ := env.Run(t, "loki.process", Arguments{ForwardTo: []loki.LogsReceiver{blocking}})
			recv := ctrl.Exports().(Exports).Receiver

			errc := make(chan error, 1)
			go func() {
				errc <- env.Send(t.Context(), recv, loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"app": "test"}, 0, push.Entry{
					Timestamp: time.Now(),
					Line:      "line",
				}))
			}()
			<-entered

			out := env.NewReceiver(t)
			var args Arguments
			require.NoError(t, syntax.Unmarshal([]byte(`
			forward_to = []
			stage.static_labels {
				values = { lbl = "new" }
			}
			`), &args))
			args.ForwardTo = []loki.LogsReceiver{out}

			// Update stops the old pipeline, which must cancel the call blocked
			// downstream instead of waiting for it.
			require.NoError(t, ctrl.Update(args))
			require.ErrorIs(t, <-errc, loki.ErrConsumerStopped)

			require.NoError(t, env.Send(t.Context(), recv, loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"app": "test"}, 0, push.Entry{
				Timestamp: time.Now(),
				Line:      "line",
			})))

			got := out.Next(t, 1)
			require.Equal(t, model.LabelSet{"app": "test", "lbl": "new"}, got[0].Labels)
		})

		t.Run("forward_to change does not cancel call blocked downstream", func(t *testing.T) {
			var (
				entered = make(chan struct{})
				release = make(chan struct{})
			)
			blocking := loki.NewLogsReceiverV2(lokitest.ConsumerFunc(func(ctx context.Context, _ loki.Batch) error {
				close(entered)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-release:
					return nil
				}
			}), "test")

			var args Arguments
			require.NoError(t, syntax.Unmarshal([]byte(`
			forward_to = []
			stage.static_labels {
				values = { lbl = "a" }
			}
			`), &args))
			args.ForwardTo = []loki.LogsReceiver{blocking}

			ctrl, _ := env.Run(t, "loki.process", args)
			recv := ctrl.Exports().(Exports).Receiver

			errc := make(chan error, 1)
			go func() {
				errc <- env.Send(t.Context(), recv, loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"app": "test"}, 0, push.Entry{
					Timestamp: time.Now(),
					Line:      "line",
				}))
			}()
			<-entered

			// Same stages, so the pipeline is kept and the blocked call is not cancelled.
			out := env.NewReceiver(t)
			args.ForwardTo = []loki.LogsReceiver{out}
			require.NoError(t, ctrl.Update(args))

			close(release)
			require.NoError(t, <-errc)

			require.NoError(t, env.Send(t.Context(), recv, loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"app": "test"}, 0, push.Entry{
				Timestamp: time.Now(),
				Line:      "line",
			})))

			got := out.Next(t, 1)
			require.Equal(t, model.LabelSet{"app": "test", "lbl": "a"}, got[0].Labels)
		})

		t.Run("frequent updates do not deadlock", func(t *testing.T) {
			var delivered atomic.Int64
			out1 := loki.NewLogsReceiverV2(lokitest.ConsumerFunc(func(_ context.Context, _ loki.Batch) error {
				delivered.Inc()
				return nil
			}), "out1")
			out2 := loki.NewLogsReceiverV2(lokitest.ConsumerFunc(func(_ context.Context, _ loki.Batch) error {
				delivered.Inc()
				return nil
			}), "out2")

			var args1, args2 Arguments
			require.NoError(t, syntax.Unmarshal([]byte(`
			forward_to = []
			stage.static_labels {
				values = { lbl = "a" }
			}
			`), &args1))
			require.NoError(t, syntax.Unmarshal([]byte(`
			forward_to = []
			stage.static_labels {
				values = { lbl = "b" }
			}
			`), &args2))
			args1.ForwardTo = []loki.LogsReceiver{out1}
			args2.ForwardTo = []loki.LogsReceiver{out2}

			ctrl, _ := env.Run(t, "loki.process", args1)
			recv := ctrl.Exports().(Exports).Receiver

			var (
				wg   sync.WaitGroup
				done atomic.Bool
			)

			wg.Go(func() {
				for !done.Load() {
					// Calls in flight during an Update are cancelled, so errors are expected.
					_ = env.Send(t.Context(), recv, loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"app": "test"}, 0, push.Entry{
						Timestamp: time.Now(),
						Line:      "line",
					}))
				}
			})

			wg.Go(func() {
				for !done.Load() {
					_ = ctrl.Update(args2)
					_ = ctrl.Update(args1)
				}
			})

			time.Sleep(500 * time.Millisecond)
			done.Store(true)
			wg.Wait()

			require.NoError(t, env.Send(t.Context(), recv, loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"app": "test"}, 0, push.Entry{
				Timestamp: time.Now(),
				Line:      "line",
			})))
			require.Positive(t, delivered.Load())
		})
	})
}

func TestComponent_UpdateInvalidConfig(t *testing.T) {
	lokitest.Test(t, func(t *testing.T, env lokitest.Env) {
		out := env.NewReceiver(t)
		ctrl, stop := env.Run(t, "loki.process", Arguments{ForwardTo: []loki.LogsReceiver{out}})
		defer stop()

		recv := ctrl.Exports().(Exports).Receiver
		require.NoError(t, env.Send(t.Context(), recv, loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"app": "test"}, 0, push.Entry{
			Timestamp: time.Now(),
			Line:      "before",
		})))

		err := ctrl.Update(Arguments{
			ForwardTo: []loki.LogsReceiver{out},
			Stages: []stages.StageConfig{
				{
					// label_drop requires at least one value.
					LabelDropConfig: &stages.LabelDropConfig{},
				},
			},
		})
		require.Error(t, err)

		// The old pipeline must keep working after a failed update.
		require.NoError(t, env.Send(t.Context(), recv, loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"app": "test"}, 0, push.Entry{
			Timestamp: time.Now(),
			Line:      "after",
		})))
		out.Next(t, 2)
	})
}

func TestEntrySentToTwoProcessComponents(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	// Two loki.process components get the same entry and set the same label to
	// different values, so neither must see the other's change.
	cfg1 := `
	forward_to = []
	stage.static_labels {
		values = { "lbl" = "foo" }
	}
	`
	cfg2 := `
	forward_to = []
	stage.static_labels {
		values = { "lbl" = "bar" }
	}
	`

	// setup runs both components and returns their outputs and their receivers.
	setup := func(t *testing.T, env lokitest.Env) (*lokitest.Receiver, *lokitest.Receiver, []loki.LogsReceiver) {
		out1, out2 := env.NewReceiver(t), env.NewReceiver(t)

		var args1, args2 Arguments
		require.NoError(t, syntax.Unmarshal([]byte(cfg1), &args1))
		require.NoError(t, syntax.Unmarshal([]byte(cfg2), &args2))
		args1.ForwardTo = []loki.LogsReceiver{out1}
		args2.ForwardTo = []loki.LogsReceiver{out2}

		ctrl1, _ := env.Run(t, "loki.process", args1)
		ctrl2, _ := env.Run(t, "loki.process", args2)

		return out1, out2, []loki.LogsReceiver{
			ctrl1.Exports().(Exports).Receiver,
			ctrl2.Exports().(Exports).Receiver,
		}
	}

	assert := func(t *testing.T, out1, out2 *lokitest.Receiver) {
		got1, got2 := out1.Next(t, 1), out2.Next(t, 1)
		require.Equal(t, "line", got1[0].Line)
		require.Equal(t, model.LabelSet{"app": "test", "lbl": "foo"}, got1[0].Labels)
		require.Equal(t, "line", got2[0].Line)
		require.Equal(t, model.LabelSet{"app": "test", "lbl": "bar"}, got2[0].Labels)
	}

	lokitest.TestV1(t, func(t *testing.T, env lokitest.Env) {
		out1, out2, receivers := setup(t, env)

		fanout := loki.NewFanout(receivers)
		require.NoError(t, fanout.Send(t.Context(), loki.NewEntryWithCreatedUnixMicro(model.LabelSet{"app": "test"}, 0, push.Entry{
			Timestamp: time.Now(),
			Line:      "line",
		})))

		assert(t, out1, out2)
	})

	lokitest.TestV2(t, func(t *testing.T, env lokitest.Env) {
		out1, out2, receivers := setup(t, env)

		batch := loki.NewBatch()
		batch.AddEntry(model.LabelSet{"app": "test"}, 0, push.Entry{
			Timestamp: time.Now(),
			Line:      "line",
		})

		fanout := loki.NewFanoutV2(receivers)
		require.NoError(t, fanout.Consume(t.Context(), batch))

		assert(t, out1, out2)
	})
}

const logline = `{"log":"log message\n","stream":"stderr","time":"2019-04-30T02:12:41.8443515Z","extra":"{\"user\":\"smith\"}"}`

type testFrequentUpdate struct {
	t *testing.T
	c *Component

	receiver1 loki.LogsReceiver
	receiver2 loki.LogsReceiver

	keepSending   atomic.Bool
	stopReceiving chan struct{}
	keepUpdating  atomic.Bool

	wgSend    sync.WaitGroup
	wgReceive sync.WaitGroup
	wgRun     sync.WaitGroup
	wgUpdate  sync.WaitGroup

	lastSend atomic.Value

	stop func()
}

func startTestFrequentUpdate(t *testing.T, cfg string) *testFrequentUpdate {
	res := testFrequentUpdate{
		t:             t,
		receiver1:     loki.NewLogsReceiver(),
		receiver2:     loki.NewLogsReceiver(),
		stopReceiving: make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(t.Context())

	res.keepSending.Store(true)
	res.keepUpdating.Store(true)

	res.stop = func() {
		res.keepUpdating.Store(false)
		res.wgUpdate.Wait()

		res.keepSending.Store(false)
		res.wgSend.Wait()

		cancel()
		res.wgRun.Wait()

		close(res.stopReceiving)
		res.wgReceive.Wait()

		close(res.receiver1.Chan())
		close(res.receiver2.Chan())
	}

	var args Arguments
	err := syntax.Unmarshal([]byte(cfg), &args)
	require.NoError(t, err)

	args.ForwardTo = []loki.LogsReceiver{res.receiver1, res.receiver2}

	logger := util.TestAlloyLogger(t)
	opts := component.Options{
		Logger:        logger.Slog(),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
		GetServiceData: func(name string) (any, error) {
			switch name {
			case livedebugging.ServiceName:
				return livedebugging.NewLiveDebugging(), nil
			case lokipipelinev2.ServiceName:
				return lokipipelinev2.New(lokipipelinev2.Options{}), nil
			default:
				return nil, fmt.Errorf("service not found %s", name)
			}
		},
	}

	res.c, err = New(opts, args)
	require.NoError(t, err)

	res.wgRun.Go(func() {
		res.c.Run(ctx)
	})

	return &res
}

// Continuously receive the logs from both channels
func (r *testFrequentUpdate) drainLogs() {
	drainLogs := func() {
		r.lastSend.Store(time.Now())
	}

	r.wgReceive.Add(1)
	go func() {
		for {
			select {
			case <-r.stopReceiving:
				r.wgReceive.Done()
				return
			case <-r.receiver1.Chan():
				drainLogs()
			case <-r.receiver2.Chan():
				drainLogs()
			}
		}
	}()
}

// Continuously send entries to both channels
func (r *testFrequentUpdate) sendLogs() {
	r.wgSend.Go(func() {
		for r.keepSending.Load() {
			ts := time.Now()
			logEntry := loki.Entry{
				Labels:    model.LabelSet{"filename": "/var/log/pods/agent/agent/1.log", "foo": "bar"},
				Timestamp: ts,
				Line:      logline,
			}
			select {
			case r.c.receiver.Chan() <- logEntry:
			default:
				// continue
			}
		}
	})
}

func (r *testFrequentUpdate) updateContinuously(cfg1, cfg2 string) {
	var args1 Arguments
	err := syntax.Unmarshal([]byte(cfg1), &args1)
	require.NoError(r.t, err)
	args1.ForwardTo = []loki.LogsReceiver{r.receiver1}

	var args2 Arguments
	err = syntax.Unmarshal([]byte(cfg2), &args2)
	require.NoError(r.t, err)
	args2.ForwardTo = []loki.LogsReceiver{r.receiver2}

	r.wgUpdate.Go(func() {
		for r.keepUpdating.Load() {
			r.c.Update(args1)
			r.c.Update(args2)
		}
	})
}

func TestDeadlockWithFrequentUpdatesV1(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	cfg1 := `stage.json {
			    expressions    = {"output" = "log", stream = "stream", timestamp = "time", "extra" = "" }
				drop_malformed = true
		    }
			stage.json {
			    expressions = { "user" = "" }
				source      = "extra"
			}
			stage.labels {
			    values = {
				  stream = "",
				  user   = "",
				  ts     = "timestamp",
			    }
			}
			forward_to = []`

	cfg2 := `stage.json {
			    expressions    = {"output" = "log", stream = "stream", timestamp = "time", "extra" = "" }
				drop_malformed = true
		    }
			stage.labels {
			    values = {
				  stream = "",
				  ts     = "timestamp",
			    }
			}
			forward_to = []`

	r := startTestFrequentUpdate(t, `forward_to = []`)
	defer r.stop()

	// Continuously send entries to both channels
	r.sendLogs()

	// Continuously receive entries on both channels
	r.drainLogs()

	// Call Updates
	r.updateContinuously(cfg1, cfg2)

	// Run everything for a while
	time.Sleep(1 * time.Second)
	require.WithinDuration(t, time.Now(), r.lastSend.Load().(time.Time), 300*time.Millisecond)
}

func TestMetricsStageRefresh(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	forwardArgs := `
	// This will be filled later
	forward_to = []`

	numLogsToSend := 3

	cfg := `
	stage.match {
		selector = "{ test = \"\" }"

		stage.metrics {
			metric.counter {
			  name = "inner_counter"
			  action = "inc"
			  match_all = true
			}
		}
	}

	stage.metrics {
        metric.counter {
          name = "outer_counter"
          action = "inc"
          match_all = true
        }
	}` + forwardArgs

	expectedMetrics := `
	# HELP loki_process_custom_inner_counter
	# TYPE loki_process_custom_inner_counter counter
	loki_process_custom_inner_counter{filename="/var/log/pods/agent/agent/1.log",foo="bar"} %d
	# HELP loki_process_custom_outer_counter
	# TYPE loki_process_custom_outer_counter counter
	loki_process_custom_outer_counter{filename="/var/log/pods/agent/agent/1.log",foo="bar"} %d
	`

	// Use a config which has a metric with a different name,
	// as well as a metric with the same name as the one in the previous config.
	// We try having a metric with the same name as before so that we can see if there
	// is some sort of double registration error for that metric.
	updatedCfg := `
	stage.match {
		selector = "{ test = \"\" }"

		stage.metrics {
			metric.counter {
			  name = "inner_counter"
			  action = "inc"
			  match_all = true
			}
		}
	}

	stage.metrics {
		metric.counter {
		  name = "outer_counter"
		  action = "inc"
		  match_all = true
		}

        metric.counter {
          name = "outer_counter_2"
          action = "inc"
          match_all = true
        }
	}` + forwardArgs

	expectedMetrics3 := `
	# HELP loki_process_custom_inner_counter
	# TYPE loki_process_custom_inner_counter counter
	loki_process_custom_inner_counter{filename="/var/log/pods/agent/agent/1.log",foo="bar"} %d
	# HELP loki_process_custom_outer_counter
	# TYPE loki_process_custom_outer_counter counter
	loki_process_custom_outer_counter{filename="/var/log/pods/agent/agent/1.log",foo="bar"} %d
	# HELP loki_process_custom_outer_counter_2
	# TYPE loki_process_custom_outer_counter_2 counter
	loki_process_custom_outer_counter_2{filename="/var/log/pods/agent/agent/1.log",foo="bar"} %d
	`

	lokitest.Test(t, func(t *testing.T, env lokitest.Env) {
		logTimestamp := time.Now()
		logEntry := loki.Entry{
			Labels:    model.LabelSet{"filename": "/var/log/pods/agent/agent/1.log", "foo": "bar"},
			Timestamp: logTimestamp,
			Line:      logline,
		}

		// updateAndTest applies cfg, checks the metrics, sends numLogs entries,
		// receives them and checks the metrics again.
		updateAndTest := func(t *testing.T, ctrl *componenttest.Controller, out *lokitest.Receiver, numLogs int, cfg, before, after string) {
			var args Arguments
			require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
			args.ForwardTo = []loki.LogsReceiver{out}
			require.NoError(t, ctrl.Update(args))

			require.NoError(t, testutil.GatherAndCompare(ctrl.PromRegistry, strings.NewReader(before)))

			recv := ctrl.Exports().(Exports).Receiver
			for range numLogs {
				require.NoError(t, env.Send(t.Context(), recv, logEntry))
			}

			for _, e := range out.Next(t, numLogs) {
				require.True(t, logTimestamp.Equal(e.Timestamp))
				require.Equal(t, logline, e.Line)
				require.Equal(t, model.LabelSet{"filename": "/var/log/pods/agent/agent/1.log", "foo": "bar"}, e.Labels)
			}

			require.NoError(t, testutil.GatherAndCompare(ctrl.PromRegistry, strings.NewReader(after)))
		}

		t.Run("refresh", func(t *testing.T) {
			out := env.NewReceiver(t)
			ctrl, _ := env.Run(t, "loki.process", Arguments{ForwardTo: []loki.LogsReceiver{out}})

			// The component will be reconfigured so that it has a metric.
			t.Run("config with a metric", func(t *testing.T) {
				updateAndTest(t, ctrl, out, numLogsToSend, cfg,
					"",
					fmt.Sprintf(expectedMetrics, numLogsToSend, numLogsToSend))
			})

			// The component will be "updated" with the same config.
			// We expect the metric to stay the same before logs are sent - the component should be smart enough to
			// know that the new config is the same as the old one and it should just keep running as it is.
			// If it resets the metric, this could cause issues with some users who have a sidecar "autoreloader"
			// which reloads the collector config every X seconds.
			// Those users wouldn't expect their metrics to be reset every time the config is reloaded.
			t.Run("config with the same metric", func(t *testing.T) {
				updateAndTest(t, ctrl, out, numLogsToSend, cfg,
					fmt.Sprintf(expectedMetrics, numLogsToSend, numLogsToSend),
					fmt.Sprintf(expectedMetrics, 2*numLogsToSend, 2*numLogsToSend))
			})

			// Use a config which has no metrics stage.
			// This should cause the metric to disappear.
			t.Run("config with no metrics stage", func(t *testing.T) {
				updateAndTest(t, ctrl, out, numLogsToSend, forwardArgs, "", "")
			})

			t.Run("config with a new and old metric", func(t *testing.T) {
				updateAndTest(t, ctrl, out, numLogsToSend, updatedCfg,
					"",
					fmt.Sprintf(expectedMetrics3, numLogsToSend, numLogsToSend, numLogsToSend))
			})
		})

		// Alternate between two metrics configs many times, so leaked goroutines or
		// series left behind by old pipelines show up.
		t.Run("alternating metrics configs", func(t *testing.T) {
			out := env.NewReceiver(t)
			ctrl, _ := env.Run(t, "loki.process", Arguments{ForwardTo: []loki.LogsReceiver{out}})

			cfg1 := `
			stage.metrics {
				metric.counter {
					name = "paulin_test1"
					action = "inc"
					match_all = true
				}
			}` + forwardArgs

			cfg2 := `
			stage.metrics {
				metric.counter {
					name = "paulin_test2"
					action = "inc"
					match_all = true
				}
			}` + forwardArgs

			metrics1 := `
			# HELP loki_process_custom_paulin_test1
			# TYPE loki_process_custom_paulin_test1 counter
			loki_process_custom_paulin_test1{filename="/var/log/pods/agent/agent/1.log",foo="bar"} 1
			`

			metrics2 := `
			# HELP loki_process_custom_paulin_test2
			# TYPE loki_process_custom_paulin_test2 counter
			loki_process_custom_paulin_test2{filename="/var/log/pods/agent/agent/1.log",foo="bar"} 1
			`

			for range 101 {
				updateAndTest(t, ctrl, out, 1, cfg1, "", metrics1)
				updateAndTest(t, ctrl, out, 1, cfg2, "", metrics2)
			}
		})
	})
}
