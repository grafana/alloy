package stages

import (
	"testing"
	"time"

	"github.com/alecthomas/units"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/regexp"
	"github.com/grafana/alloy/syntax"
)

func TestDropStage(t *testing.T) {
	var (
		oneHour     = 1 * time.Hour
		tenBytes, _ = units.ParseBase2Bytes("10B")
		now         = time.Now()
	)

	type testCase struct {
		name     string
		cfg      DropConfig
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "longer than",
			cfg:  DropConfig{LongerThan: tenBytes},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "12345678901", now),
				newEntry(map[string]any{}, model.LabelSet{}, "1234567890", now),
				newEntry(map[string]any{}, model.LabelSet{}, "123456789", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "1234567890", now),
				newEntry(map[string]any{}, model.LabelSet{}, "123456789", now),
			},
		},
		{
			name: "older than",
			cfg:  DropConfig{OlderThan: oneHour},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "", now.Add(-2*time.Hour)),
				newEntry(map[string]any{}, model.LabelSet{}, "", now.Add(-5*time.Minute)),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "", now.Add(-5*time.Minute)),
			},
		},
		{
			name: "source",
			cfg:  DropConfig{Source: "key"},
			entries: []Entry{
				newEntry(map[string]any{"key": ""}, model.LabelSet{}, "", now),
				newEntry(map[string]any{"other": "val1"}, model.LabelSet{}, "", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"other": "val1"}, model.LabelSet{}, "", now),
			},
		},
		{
			name: "source and value",
			cfg:  DropConfig{Source: "key", Value: "val1"},
			entries: []Entry{
				newEntry(map[string]any{"key": "val1"}, model.LabelSet{}, "", now),
				newEntry(map[string]any{"key": "VALRUE1"}, model.LabelSet{}, "", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"key": "VALRUE1"}, model.LabelSet{}, "", now),
			},
		},
		{
			name: "source and value with int and string extracted types",
			cfg:  DropConfig{Source: "level", Value: "50"},
			entries: []Entry{
				newEntry(map[string]any{"level": 50}, model.LabelSet{}, "", now),
				newEntry(map[string]any{"level": "50"}, model.LabelSet{}, "", now),
				newEntry(map[string]any{"level": 100}, model.LabelSet{}, "", now),
				newEntry(map[string]any{"level": "100"}, model.LabelSet{}, "", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"level": 100}, model.LabelSet{}, "", now),
				newEntry(map[string]any{"level": "100"}, model.LabelSet{}, "", now),
			},
		},
		{
			name: "source and value with multiple sources",
			cfg:  DropConfig{Source: "key1,key2", Separator: defaultSeparator, Value: `val1;val200.*`},
			entries: []Entry{
				newEntry(map[string]any{"key1": "val1", "key2": "val200.*"}, model.LabelSet{}, "", now),
			},
			expected: []Entry{},
		},
		{
			name: "source and value with multiple sources and custom separator",
			cfg:  DropConfig{Source: "key1,key2", Separator: "|", Value: `val1|val200[a]`},
			entries: []Entry{
				newEntry(map[string]any{"key1": "val1", "key2": "val200[a]"}, model.LabelSet{}, "", now),
			},
			expected: []Entry{},
		},
		{
			name: "source and expression with int and string extracted types",
			cfg:  DropConfig{Source: "key", Expression: *regexp.MustCompile("50")},
			entries: []Entry{
				newEntry(map[string]any{"key": 50}, model.LabelSet{}, "", now),
				newEntry(map[string]any{"key": "50"}, model.LabelSet{}, "", now),
			},
			expected: []Entry{},
		},
		{
			name: "source and expression with multiple sources",
			cfg:  DropConfig{Source: "key1,key2", Separator: defaultSeparator, Expression: *regexp.MustCompile(`val\d{1};val\d{3}$`)},
			entries: []Entry{
				newEntry(map[string]any{"key1": "val1", "key2": "val200"}, model.LabelSet{}, "", now),
			},
			expected: []Entry{},
		},
		{
			name: "source and expression with multiple sources and custom separator",
			cfg:  DropConfig{Source: "key1,key2", Separator: "#", Expression: *regexp.MustCompile(`val\d{1}#val\d{3}$`)},
			entries: []Entry{
				newEntry(map[string]any{"key1": "val1", "key2": "val200"}, model.LabelSet{}, "", now),
			},
			expected: []Entry{},
		},
		{
			name: "source and expression not matching",
			cfg:  DropConfig{Source: "key", Expression: *regexp.MustCompile(".*val.*")},
			entries: []Entry{
				newEntry(map[string]any{"key": "pal1"}, model.LabelSet{}, "", now),
				newEntry(map[string]any{"pokey": "pal1"}, model.LabelSet{}, "", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"key": "pal1"}, model.LabelSet{}, "", now),
				newEntry(map[string]any{"pokey": "pal1"}, model.LabelSet{}, "", now),
			},
		},
		{
			name: "source and expression not matching with multiple sources",
			cfg:  DropConfig{Source: "key1,key2", Separator: defaultSeparator, Expression: *regexp.MustCompile(`match\d+;match\d+`)},
			entries: []Entry{
				newEntry(map[string]any{"key1": "match1", "key2": "notmatch2"}, model.LabelSet{}, "", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"key1": "match1", "key2": "notmatch2"}, model.LabelSet{}, "", now),
			},
		},
		{
			name: "source and expression not matching with multiple sources and custom separator",
			cfg:  DropConfig{Source: "key1,key2", Separator: "#", Expression: *regexp.MustCompile(`match\d;match\d`)},
			entries: []Entry{
				newEntry(map[string]any{"key1": "match1", "key2": "match2"}, model.LabelSet{}, "", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"key1": "match1", "key2": "match2"}, model.LabelSet{}, "", now),
			},
		},
		{
			name: "expression only",
			cfg:  DropConfig{Expression: *regexp.MustCompile(".*val.*")},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "this is a line which does not match the regex", now),
				newEntry(map[string]any{}, model.LabelSet{}, "this is a line with the word value in it", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "this is a line which does not match the regex", now),
			},
		},
		{
			name: "source and length",
			cfg:  DropConfig{Source: "key", LongerThan: tenBytes},
			entries: []Entry{
				newEntry(map[string]any{"key": "pal1"}, model.LabelSet{}, "12345678901", now),
				newEntry(map[string]any{"key": "pal1"}, model.LabelSet{}, "123456789", now),
				newEntry(map[string]any{"WOOOOOOOOOOOOOO": "pal1"}, model.LabelSet{}, "123456789012", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"key": "pal1"}, model.LabelSet{}, "123456789", now),
				newEntry(map[string]any{"WOOOOOOOOOOOOOO": "pal1"}, model.LabelSet{}, "123456789012", now),
			},
		},
		{
			name: "everything must match",
			cfg: DropConfig{
				Source:     "key",
				Expression: *regexp.MustCompile(".*val.*"),
				OlderThan:  oneHour,
				LongerThan: tenBytes,
			},
			entries: []Entry{
				newEntry(map[string]any{"key": "must contain value to match"}, model.LabelSet{}, "12345678901", now.Add(-2*time.Hour)),
			},
			expected: []Entry{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, []StageConfig{{DropConfig: &tt.cfg}}, tt.entries, tt.expected)
		})
	}
}

func TestUnmarshalDropConfig(t *testing.T) {
	type testCase struct {
		name      string
		cfg       string
		expectErr bool
	}

	tests := []testCase{
		{
			name: "valid source",
			cfg: `
				source = "key"
			`,
		},
		{
			name: "valid source and value",
			cfg: `
				source = "key"
				value  = "val"
			`,
		},
		{
			name: "valid expression",
			cfg: `
				expression = "(?P<ts>[0-9]+).*"
			`,
		},
		{
			name: "valid older than",
			cfg: `
				older_than = "1h"
			`,
		},
		{
			name: "valid longer than",
			cfg: `
				longer_than = "10B"
			`,
		},
		{
			name:      "empty config",
			cfg:       ``,
			expectErr: true,
		},
		{
			name: "empty expression",
			cfg: `
				expression = ""
			`,
			expectErr: true,
		},
		{
			name: "invalid expression",
			cfg: `
				expression = "(?P<ts[0-9]+).*"
			`,
			expectErr: true,
		},
		{
			name: "value and expression",
			cfg: `
				source     = "key"
				value      = "val"
				expression = ".*"
			`,
			expectErr: true,
		},
		{
			name: "value without source",
			cfg: `
				value = "val"
			`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg DropConfig
			err := syntax.Unmarshal([]byte(tt.cfg), &cfg)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
