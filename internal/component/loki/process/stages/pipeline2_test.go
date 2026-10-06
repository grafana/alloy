package stages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
)

// TestPipelineConsumerConcurrent runs every migrated stage in one pipeline from several
// goroutines at once to check for data races under -race.
func TestPipelineConsumerConcurrent(t *testing.T) {
	const (
		consumers          = 4
		batchesPerConsumer = 10
		streamsPerBatch    = 4
	)

	// One line shape per parser.
	const (
		ansiLine       = "\x1b[31mred\x1b[0m plain text"
		arrayLine      = `[{"a":1},{"b":2}]`
		criPartial     = "2026-09-18T10:00:00.000000001Z stderr P a partial fragment "
		criFull        = "2026-09-18T10:00:00.000000001Z stderr F user=bob token=4539148803436467 dur=1.5"
		dockerLine     = `{"log":"user=bob dur=1.5\n","stream":"stderr","time":"2026-09-18T10:00:00Z"}`
		logfmtLine     = "level=info user=bob token=4539148803436467 dur=1.5"
		jsonLine       = `{"level":"info","ip":"1.2.3.4","ts":"2026-09-18T10:00:00Z","evt":"yay","msg":"user=bob token=4539148803436467 dur=1.5"}`
		multilineStart = "START user=bob token=4539148803436467 dur=1.5"
		multilineCont  = "\tat com.example.Foo.bar(Foo.java:42)"
	)

	lines := []string{criPartial, criFull, dockerLine, jsonLine, arrayLine, logfmtLine, ansiLine, multilineStart, multilineCont}

	cfg := loadConfig(`
		stage.cri {}

		stage.docker {}

		stage.multiline {
			firstline     = "^START"
			max_lines     = 3
			max_wait_time = "10ms"
		}

		stage.json {
			expressions = {
				level = "level",
				ip    = "ip",
				ts    = "ts",
				evt   = "evt",
				msg   = "msg",
			}
		}

		stage.logfmt {
			source  = "msg"
			mapping = { "user" = "", "token" = "", "dur" = "" }
		}

		stage.regex {
			source     = "msg"
			expression = "dur=(?P<dur_seconds>[0-9.]+)"
		}

		stage.pattern {
			source  = "msg"
			pattern = "user=<pattern_user> token=<pattern_token> <pattern_rest>"
		}

		stage.luhn {
			source = "msg"
		}

		stage.replace {
			source     = "msg"
			expression = "(bob)"
			replace    = "redacted-user"
		}

		stage.truncate {
			rule {
				limit       = "1000B"
				suffix      = "..."
				sources     = ["msg"]
				source_type = "extracted"
			}
		}

		stage.geoip {
			db      = "testdata/geoip_maxmind_city.mmdb"
			source  = "ip"
			db_type = "city"
		}

		stage.template {
			source   = "level"
			template = "{{ .Value }}-templated"
		}

		stage.timestamp {
			source                        = "ts"
			format                        = "RFC3339"
			action_on_duplicate_timestamp = "fudge"
		}

		stage.windowsevent {
			source              = "evt"
			drop_invalid_labels = true
			overwrite_existing  = true
		}

		stage.eventlogmessage {
			source              = "evt"
			drop_invalid_labels = true
			overwrite_existing  = true
		}

		stage.split_json {}

		stage.labels {
			values = { "level" = "" }
		}

		stage.static_labels {
			values = { "env" = "race-test", "temporary" = "dropped-below" }
		}

		stage.label_drop {
			values = ["temporary"]
		}

		stage.label_keep {
			values = ["app", "instance", "level", "env"]
		}

		stage.structured_metadata {
			values = { "user" = "" }
		}

		stage.structured_metadata_drop {
			values = ["user"]
		}

		stage.tenant {
			source = "level"
		}

		stage.limit {
			rate                = 1000000
			burst               = 1000000
			drop                = true
			by_label_name       = "app"
			max_distinct_labels = 10000
		}

		stage.sampling {
			rate = 1.0
		}

		stage.drop {
			source = "never_extracted"
			value  = "never_matches"
		}

		stage.metrics {
			metric.counter {
				name   = "race_test_lines"
				action = "inc"
				source = "level"
			}
			metric.gauge {
				name   = "race_test_duration"
				action = "set"
				source = "dur"
			}
			metric.histogram {
				name    = "race_test_duration_hist"
				source  = "dur"
				buckets = [0.5, 1, 2]
			}
		}

		stage.output {
			source = "msg"
		}

		stage.decolorize {}

		stage.pack {
			labels           = ["env"]
			ingest_timestamp = true
		}
	`)

	pc, err := NewPipelineConsumer(
		slog.New(slog.DiscardHandler),
		prometheus.NewRegistry(),
		featuregate.StabilityGenerallyAvailable,
		cfg,
		loki.NewNopConsumer(),
	)
	require.NoError(t, err)

	var (
		wg   sync.WaitGroup
		errs = make([]error, consumers)
	)

	for i := range consumers {
		wg.Go(func() {
			for range batchesPerConsumer {
				batch := loki.NewBatch()
				for s := range streamsPerBatch {
					// app is capped to half the consumer count so goroutines contend for the
					// same rate limiter in limit and the same fingerprint in cri and timestamp.
					labels := model.LabelSet{
						"app":      model.LabelValue(fmt.Sprintf("app-%d", (i+s)%consumers/2)),
						"instance": model.LabelValue(fmt.Sprintf("stream-%d", s)),
					}

					for _, line := range lines {
						batch.AddEntry(labels, time.Now().UnixMicro(), push.Entry{
							Timestamp: time.Now(),
							Line:      line,
						})
					}
				}

				if err := pc.Consume(context.Background(), batch); err != nil {
					errs[i] = err
					return
				}
			}
		})
	}

	wg.Wait()
	pc.Stop()
	require.NoError(t, errors.Join(errs...))
}
