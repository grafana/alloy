package stages

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

var testTimestampLogLine = `
{
	"time":"2012-11-01T22:08:41-04:00",
	"app":"loki",
	"component": ["parser","type"],
	"level" : "WARN"
}
`

var testTimestampLogLineWithMissingKey = `
{
	"app":"loki",
	"component": ["parser","type"],
	"level" : "WARN"
}
`

var (
	invalidLocationString = "America/Canada"
	validLocationString   = "America/New_York"
	validLocation, _      = time.LoadLocation(validLocationString)
)

func TestTimestampStage(t *testing.T) {
	now := time.Now()

	type testCase struct {
		name     string
		config   string
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "set success",
			config: `
			stage.timestamp {
				source = "ts"
				format = "2006-01-02T15:04:05Z07:00"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"somethigelse": "notimportant",
					"ts":           "2106-01-02T23:04:05-04:00",
				}, model.LabelSet{}, "hello world", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"somethigelse": "notimportant",
					"ts":           "2106-01-02T23:04:05-04:00",
				}, model.LabelSet{}, "hello world", time.Date(2106, 01, 02, 23, 04, 05, 0, time.FixedZone("", -4*60*60))),
			},
		},
		{
			name: "unix success",
			config: `
			stage.timestamp {
				source = "ts"
				format = "Unix"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"somethigelse": "notimportant",
					"ts":           "1562708916",
				}, model.LabelSet{}, "hello world", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"somethigelse": "notimportant",
					"ts":           "1562708916",
				}, model.LabelSet{}, "hello world", time.Date(2019, 7, 9, 21, 48, 36, 0, time.UTC)),
			},
		},
		{
			name: "unix fractions ms success",
			config: `
			stage.timestamp {
				source = "ts"
				format = "Unix"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"somethigelse": "notimportant",
					"ts":           "1562708916.414123",
				}, model.LabelSet{}, "hello world", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"somethigelse": "notimportant",
					"ts":           "1562708916.414123",
				}, model.LabelSet{}, "hello world", time.Date(2019, 7, 9, 21, 48, 36, 414123*1000, time.UTC)),
			},
		},
		{
			name: "unix fractions ns success",
			config: `
			stage.timestamp {
				source = "ts"
				format = "Unix"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"somethigelse": "notimportant",
					"ts":           "1562708916.000000123",
				}, model.LabelSet{}, "hello world", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"somethigelse": "notimportant",
					"ts":           "1562708916.000000123",
				}, model.LabelSet{}, "hello world", time.Date(2019, 7, 9, 21, 48, 36, 123, time.UTC)),
			},
		},
		{
			name: "unix millisecond success",
			config: `
			stage.timestamp {
				source = "ts"
				format = "UnixMs"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"somethigelse": "notimportant",
					"ts":           "1562708916414",
				}, model.LabelSet{}, "hello world", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"somethigelse": "notimportant",
					"ts":           "1562708916414",
				}, model.LabelSet{}, "hello world", time.Date(2019, 7, 9, 21, 48, 36, 414*1000000, time.UTC)),
			},
		},
		{
			name: "unix microsecond success",
			config: `
			stage.timestamp {
				source = "ts"
				format = "UnixUs"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"somethigelse": "notimportant",
					"ts":           "1562708916414123",
				}, model.LabelSet{}, "hello world", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"somethigelse": "notimportant",
					"ts":           "1562708916414123",
				}, model.LabelSet{}, "hello world", time.Date(2019, 7, 9, 21, 48, 36, 414123*1000, time.UTC)),
			},
		},
		{
			name: "unix nano success",
			config: `
			stage.timestamp {
				source = "ts"
				format = "UnixNs"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"somethigelse": "notimportant",
					"ts":           "1562708916000000123",
				}, model.LabelSet{}, "hello world", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"somethigelse": "notimportant",
					"ts":           "1562708916000000123",
				}, model.LabelSet{}, "hello world", time.Date(2019, 7, 9, 21, 48, 36, 123, time.UTC)),
			},
		},
		{
			name: "with location success",
			config: `
			stage.timestamp {
				source   = "ts"
				format   = "2006-01-02 15:04:05"
				location = "America/New_York"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"somethigelse": "notimportant",
					"ts":           "2019-07-22 20:29:32",
				}, model.LabelSet{}, "hello world", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"somethigelse": "notimportant",
					"ts":           "2019-07-22 20:29:32",
				}, model.LabelSet{}, "hello world", time.Date(2019, 7, 22, 20, 29, 32, 0, validLocation)),
			},
		},
		{
			name: "should keep the parsed timestamp on success",
			config: `
			stage.timestamp {
				source            = "time"
				format            = "2006-01-02T15:04:05.999999999Z07:00"
				action_on_failure = "fudge"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", time.Unix(1, 0)),
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.500000000Z"}, model.LabelSet{}, "", time.Unix(1, 0)),
			},
			expected: []Entry{
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000000Z")),
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.500000000Z"}, model.LabelSet{}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.500000000Z")),
			},
		},
		{
			name: "should add nanoseconds to identical parsed timestamps to preserve message order",
			config: `
			stage.timestamp {
				source                        = "time"
				format                        = "2006-01-02T15:04:05.999999999Z07:00"
				action_on_failure             = "fudge"
				action_on_duplicate_timestamp = "fudge"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", time.Unix(1, 0)),
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", time.Unix(1, 0)),
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", time.Unix(1, 0)),
			},
			expected: []Entry{
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000000Z")),
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000001Z")),
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000002Z")),
			},
		},
		{
			name: "action_on_duplicate_timestamp=keep leaves identical timestamps unchanged",
			config: `
			stage.timestamp {
				source                        = "time"
				format                        = "2006-01-02T15:04:05.999999999Z07:00"
				action_on_failure             = "fudge"
				action_on_duplicate_timestamp = "keep"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", time.Unix(1, 0)),
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", time.Unix(1, 0)),
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", time.Unix(1, 0)),
			},
			expected: []Entry{
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000000Z")),
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000000Z")),
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000000Z")),
			},
		},
		{
			name: "should fudge the timestamp based on the last known value on timestamp parsing failure",
			config: `
			stage.timestamp {
				source = "time"
				format = "2006-01-02T15:04:05.999999999Z07:00"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", time.Unix(1, 0)),
				newEntry(map[string]any{}, model.LabelSet{}, "", time.Unix(1, 0)),
				newEntry(map[string]any{}, model.LabelSet{}, "", time.Unix(1, 0)),
			},
			expected: []Entry{
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000000Z")),
				newEntry(map[string]any{}, model.LabelSet{}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000001Z")),
				newEntry(map[string]any{}, model.LabelSet{}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000002Z")),
			},
		},
		{
			name: "should fudge the timestamp based on the last known value for the right file target",
			config: `
			stage.timestamp {
				source = "time"
				format = "2006-01-02T15:04:05.999999999Z07:00"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{"filename": "/1.log"}, "", time.Unix(1, 0)),
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.800000000Z"}, model.LabelSet{"filename": "/2.log"}, "", time.Unix(1, 0)),
				newEntry(map[string]any{}, model.LabelSet{"filename": "/1.log"}, "", time.Unix(1, 0)),
				newEntry(map[string]any{}, model.LabelSet{"filename": "/2.log"}, "", time.Unix(1, 0)),
				newEntry(map[string]any{}, model.LabelSet{"filename": "/1.log"}, "", time.Unix(1, 0)),
			},
			expected: []Entry{
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z", "filename": "/1.log"}, model.LabelSet{"filename": "/1.log"}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000000Z")),
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.800000000Z", "filename": "/2.log"}, model.LabelSet{"filename": "/2.log"}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.800000000Z")),
				newEntry(map[string]any{"filename": "/1.log"}, model.LabelSet{"filename": "/1.log"}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000001Z")),
				newEntry(map[string]any{"filename": "/2.log"}, model.LabelSet{"filename": "/2.log"}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.800000001Z")),
				newEntry(map[string]any{"filename": "/1.log"}, model.LabelSet{"filename": "/1.log"}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000002Z")),
			},
		},
		{
			name: "should keep the input timestamp if unable to fudge because there's no known valid timestamp yet",
			config: `
			stage.timestamp {
				source = "time"
				format = "2006-01-02T15:04:05.999999999Z07:00"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{"filename": "/1.log"}, "", time.Unix(1, 0)),
				newEntry(map[string]any{}, model.LabelSet{"filename": "/2.log"}, "", time.Unix(1, 0)),
			},
			expected: []Entry{
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z", "filename": "/1.log"}, model.LabelSet{"filename": "/1.log"}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000000Z")),
				newEntry(map[string]any{"filename": "/2.log"}, model.LabelSet{"filename": "/2.log"}, "", time.Unix(1, 0)),
			},
		},
		{
			name: "should keep the input timestamp on action_on_failure=skip",
			config: `
			stage.timestamp {
				source            = "time"
				format            = "2006-01-02T15:04:05.999999999Z07:00"
				action_on_failure = "skip"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", time.Unix(1, 0)),
				newEntry(map[string]any{}, model.LabelSet{}, "", time.Unix(1, 0)),
			},
			expected: []Entry{
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000000Z")),
				newEntry(map[string]any{}, model.LabelSet{}, "", time.Unix(1, 0)),
			},
		},
		{
			name: "labels with colliding fingerprints should have independent timestamps when fudging",
			config: `
			stage.timestamp {
				source            = "time"
				format            = "2006-01-02T15:04:05.999999999Z07:00"
				action_on_failure = "fudge"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z"}, model.LabelSet{"app": "m", "uniq0": "1", "uniq1": "1"}, "", time.Unix(1, 0)),
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.800000000Z"}, model.LabelSet{"app": "l", "uniq0": "0", "uniq1": "1"}, "", time.Unix(1, 0)),
				newEntry(map[string]any{}, model.LabelSet{"app": "m", "uniq0": "1", "uniq1": "1"}, "", time.Unix(1, 0)),
				newEntry(map[string]any{}, model.LabelSet{"app": "l", "uniq0": "0", "uniq1": "1"}, "", time.Unix(1, 0)),
				newEntry(map[string]any{}, model.LabelSet{"app": "m", "uniq0": "1", "uniq1": "1"}, "", time.Unix(1, 0)),
				newEntry(map[string]any{}, model.LabelSet{"app": "l", "uniq0": "0", "uniq1": "1"}, "", time.Unix(1, 0)),
			},
			expected: []Entry{
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.400000000Z", "app": "m", "uniq0": "1", "uniq1": "1"}, model.LabelSet{"app": "m", "uniq0": "1", "uniq1": "1"}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000000Z")),
				newEntry(map[string]any{"time": "2019-10-01T01:02:03.800000000Z", "app": "l", "uniq0": "0", "uniq1": "1"}, model.LabelSet{"app": "l", "uniq0": "0", "uniq1": "1"}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.800000000Z")),
				newEntry(map[string]any{"app": "m", "uniq0": "1", "uniq1": "1"}, model.LabelSet{"app": "m", "uniq0": "1", "uniq1": "1"}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000001Z")),
				newEntry(map[string]any{"app": "l", "uniq0": "0", "uniq1": "1"}, model.LabelSet{"app": "l", "uniq0": "0", "uniq1": "1"}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.800000001Z")),
				newEntry(map[string]any{"app": "m", "uniq0": "1", "uniq1": "1"}, model.LabelSet{"app": "m", "uniq0": "1", "uniq1": "1"}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.400000002Z")),
				newEntry(map[string]any{"app": "l", "uniq0": "0", "uniq1": "1"}, model.LabelSet{"app": "l", "uniq0": "0", "uniq1": "1"}, "", mustParseTime(time.RFC3339Nano, "2019-10-01T01:02:03.800000002Z")),
			},
		},
		{
			name: "pipeline with json stage extracting the timestamp source",
			config: `
			stage.json {
				expressions = { ts = "time" }
			}
			stage.timestamp {
				source = "ts"
				format = "RFC3339"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testTimestampLogLine, now),
			},
			expected: []Entry{
				newEntry(map[string]any{"ts": "2012-11-01T22:08:41-04:00"}, model.LabelSet{}, testTimestampLogLine, time.Date(2012, 11, 01, 22, 8, 41, 0, time.FixedZone("", -4*60*60))),
			},
		},
		{
			name: "missing extracted source from json expression keeps the input timestamp unchanged",
			config: `
			stage.json {
				expressions = { ts = "time" }
			}
			stage.timestamp {
				source = "ts"
				format = "RFC3339"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testTimestampLogLineWithMissingKey, now),
			},
			expected: []Entry{
				newEntry(map[string]any{"ts": nil}, model.LabelSet{}, testTimestampLogLineWithMissingKey, now),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, loadConfig(tt.config), tt.entries, tt.expected)
		})
	}
}

func TestValidateTimestampConfig(t *testing.T) {
	tests := map[string]struct {
		config *TimestampConfig
		// Note the error text validation is a little loose as it only validates with strings.HasPrefix
		// this is to work around different errors related to timezone loading on different systems
		err            error
		testString     string
		expectedTime   time.Time
		expectedConfig *TimestampConfig
	}{
		"missing source": {
			config: &TimestampConfig{},
			err:    errTimestampSourceRequired,
		},
		"missing format": {
			config: &TimestampConfig{
				Source: "source1",
			},
			err: errTimestampFormatRequired,
		},
		"invalid location": {
			config: &TimestampConfig{
				Source:   "source1",
				Format:   "2006-01-02",
				Location: &invalidLocationString,
			},
			err: fmt.Errorf(errInvalidLocation.Error(), ""),
		},
		"standard format": {
			config: &TimestampConfig{
				Source: "source1",
				Format: time.RFC3339,
			},
			err:          nil,
			testString:   "2012-11-01T22:08:41-04:00",
			expectedTime: time.Date(2012, 11, 01, 22, 8, 41, 0, time.FixedZone("", -4*60*60)),
		},
		"sets default action on failure and on duplicate timestamp": {
			config: &TimestampConfig{
				Source: "source1",
				Format: time.RFC3339,
			},
			err: nil,
			expectedConfig: &TimestampConfig{
				Source:                     "source1",
				Format:                     time.RFC3339,
				ActionOnFailure:            "fudge",
				ActionOnDuplicateTimestamp: "fudge",
			},
		},
		"custom format with year": {
			config: &TimestampConfig{
				Source: "source1",
				Format: "2006-01-02",
			},
			err:          nil,
			testString:   "2009-01-01",
			expectedTime: time.Date(2009, 01, 01, 00, 00, 00, 0, time.UTC),
		},
		"custom format without year": {
			config: &TimestampConfig{
				Source: "source1",
				Format: "Jan 02 15:04:05",
			},
			err:          nil,
			testString:   "Jul 15 01:02:03",
			expectedTime: time.Date(time.Now().Year(), 7, 15, 1, 2, 3, 0, time.UTC),
		},
		"custom format with location": {
			config: &TimestampConfig{
				Source:   "source1",
				Format:   "2006-01-02 15:04:05",
				Location: &validLocationString,
			},
			err:          nil,
			testString:   "2009-07-01 03:30:20",
			expectedTime: time.Date(2009, 7, 1, 3, 30, 20, 0, validLocation),
		},
		"unix_ms": {
			config: &TimestampConfig{
				Source: "source1",
				Format: "UnixMs",
			},
			err:          nil,
			testString:   "1562708916919",
			expectedTime: time.Date(2019, 7, 9, 21, 48, 36, 919*1000000, time.UTC),
		},
		"should fail on invalid action on failure": {
			config: &TimestampConfig{
				Source:          "source1",
				Format:          time.RFC3339,
				ActionOnFailure: "foo",
			},
			err: fmt.Errorf(errInvalidActionOnFailure.Error(), timestampActionOnFailureOptions),
		},
		"should fail on invalid action on duplicate timestamp": {
			config: &TimestampConfig{
				Source:                     "source1",
				Format:                     time.RFC3339,
				ActionOnDuplicateTimestamp: "invalid",
			},
			err: fmt.Errorf(errInvalidActionOnDuplicateTimestamp.Error(), timestampActionOnDuplicateTimestampOptions),
		},
		"fallback formats contains the format": {
			config: &TimestampConfig{
				Source:          "source1",
				Format:          "UnixMs",
				FallbackFormats: []string{"2006-01-02 03:04:05.000000000 +0000 UTC", time.RFC3339},
			},
			err:          nil,
			testString:   "2012-11-01T22:08:41-04:00",
			expectedTime: time.Date(2012, 11, 01, 22, 8, 41, 0, time.FixedZone("", -4*60*60)),
		},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			parser, err := validateTimestampConfig(test.config)
			if (err != nil) != (test.err != nil) {
				t.Errorf("validateTimestampConfig() expected error = %v, actual error = %v", test.err, err)
				return
			}
			if (err != nil) && !strings.HasPrefix(err.Error(), test.err.Error()) {
				t.Errorf("validateTimestampConfig() expected error = %v, actual error = %v", test.err, err)
				return
			}
			if test.testString != "" {
				ts, err := parser(test.testString)
				if err != nil {
					t.Errorf("validateTimestampConfig() unexpected error parsing test time: %v", err)
					return
				}
				assert.Equal(t, test.expectedTime.UnixNano(), ts.UnixNano())
			}
			if test.expectedConfig != nil {
				assert.Equal(t, test.expectedConfig, test.config)
			}
		})
	}
}
