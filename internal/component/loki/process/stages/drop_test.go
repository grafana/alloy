package stages

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alecthomas/units"
	dskit "github.com/grafana/dskit/server"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/util"
)

// Not all these are tested but are here to make sure the different types marshal without error
var testDropAlloy = `
stage.json {
		expressions = { "app" = "", "msg" = "" }
}

stage.drop {
		source      = "src"
		expression  = ".*test.*"
		older_than  = "24h"
		longer_than = "8KB"
}

stage.drop {
		expression = ".*app1.*"
}

stage.drop {
		source = "app"
		value  = "loki"
}

stage.drop {
		longer_than = "10000B"
}
`

func Test_dropStage_Process(t *testing.T) {
	// Enable debug logging
	cfg := &dskit.Config{}
	require.Nil(t, cfg.LogLevel.Set("debug"))
	Debug = true

	tenBytes, _ := units.ParseBase2Bytes("10B")
	oneHour := 1 * time.Hour

	tests := []struct {
		name       string
		config     *DropConfig
		labels     model.LabelSet
		extracted  map[string]any
		t          time.Time
		entry      string
		shouldDrop bool
	}{
		{
			name: "Longer Than Should Drop",
			config: &DropConfig{
				LongerThan: tenBytes,
			},
			labels:     model.LabelSet{},
			extracted:  map[string]any{},
			entry:      "12345678901",
			shouldDrop: true,
		},
		{
			name: "Longer Than Should Not Drop When Equal",
			config: &DropConfig{
				LongerThan: tenBytes,
			},
			labels:     model.LabelSet{},
			extracted:  map[string]any{},
			entry:      "1234567890",
			shouldDrop: false,
		},
		{
			name: "Longer Than Should Not Drop When Less",
			config: &DropConfig{
				LongerThan: tenBytes,
			},
			labels:     model.LabelSet{},
			extracted:  map[string]any{},
			entry:      "123456789",
			shouldDrop: false,
		},
		{
			name: "Older than Should Drop",
			config: &DropConfig{
				OlderThan: oneHour,
			},
			labels:     model.LabelSet{},
			extracted:  map[string]any{},
			t:          time.Now().Add(-2 * time.Hour),
			shouldDrop: true,
		},
		{
			name: "Older than Should Not Drop",
			config: &DropConfig{
				OlderThan: oneHour,
			},
			labels:     model.LabelSet{},
			extracted:  map[string]any{},
			t:          time.Now().Add(-5 * time.Minute),
			shouldDrop: false,
		},
		{
			name: "Matched Source",
			config: &DropConfig{
				Source: "key",
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"key": "",
			},
			shouldDrop: true,
		},
		{
			name: "Did not match Source",
			config: &DropConfig{
				Source: "key1",
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"key": "val1",
			},
			shouldDrop: false,
		},
		{
			name: "Matched Source and Value",
			config: &DropConfig{
				Source: "key",
				Value:  "val1",
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"key": "val1",
			},
			shouldDrop: true,
		},
		{
			name: "Did not match Source and Value",
			config: &DropConfig{
				Source: "key",
				Value:  "val1",
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"key": "VALRUE1",
			},
			shouldDrop: false,
		},
		{
			name: "Matched Source(int) and Value(string)",
			config: &DropConfig{
				Source: "level",
				Value:  "50",
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"level": 50,
			},
			shouldDrop: true,
		},
		{
			name: "Matched Source(string) and Value(string)",
			config: &DropConfig{
				Source: "level",
				Value:  "50",
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"level": "50",
			},
			shouldDrop: true,
		},
		{
			name: "Did not match Source(int) and Value(string)",
			config: &DropConfig{
				Source: "level",
				Value:  "50",
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"level": 100,
			},
			shouldDrop: false,
		},
		{
			name: "Did not match Source(string) and Value(string)",
			config: &DropConfig{
				Source: "level",
				Value:  "50",
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"level": "100",
			},
			shouldDrop: false,
		},
		{
			name: "Matched Source and Value with multiple sources",
			config: &DropConfig{
				Source: "key1,key2",
				Value:  `val1;val200.*`,
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"key1": "val1",
				"key2": "val200.*",
			},
			shouldDrop: true,
		},
		{
			name: "Matched Source and Value with multiple sources and custom separator",
			config: &DropConfig{
				Source:    "key1,key2",
				Separator: "|",
				Value:     `val1|val200[a]`,
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"key1": "val1",
				"key2": "val200[a]",
			},
			shouldDrop: true,
		},
		{
			name: "Regex Matched Source(int) and Expression",
			config: &DropConfig{
				Source:     "key",
				Expression: "50",
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"key": 50,
			},
			shouldDrop: true,
		},
		{
			name: "Regex Matched Source(string) and Expression",
			config: &DropConfig{
				Source:     "key",
				Expression: "50",
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"key": "50",
			},
			shouldDrop: true,
		},
		{
			name: "Regex Matched Source and Expression with multiple sources",
			config: &DropConfig{
				Source:     "key1,key2",
				Expression: `val\d{1};val\d{3}$`,
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"key1": "val1",
				"key2": "val200",
			},
			shouldDrop: true,
		},
		{
			name: "Regex Matched Source and Expression with multiple sources and custom separator",
			config: &DropConfig{
				Source:     "key1,key2",
				Separator:  "#",
				Expression: `val\d{1}#val\d{3}$`,
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"key1": "val1",
				"key2": "val200",
			},
			shouldDrop: true,
		},
		{
			name: "Regex Did not match Source and Expression",
			config: &DropConfig{
				Source:     "key",
				Expression: ".*val.*",
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"key": "pal1",
			},
			shouldDrop: false,
		},
		{
			name: "Regex Did not match Source and Expression with multiple sources",
			config: &DropConfig{
				Source:     "key1,key2",
				Expression: `match\d+;match\d+`,
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"key1": "match1",
				"key2": "notmatch2",
			},
			shouldDrop: false,
		},
		{
			name: "Regex Did not match Source and Expression with multiple sources and custom separator",
			config: &DropConfig{
				Source:     "key1,key2",
				Separator:  "#",
				Expression: `match\d;match\d`,
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"key1": "match1",
				"key2": "match2",
			},
			shouldDrop: false,
		},
		{
			name: "Regex No Matching Source",
			config: &DropConfig{
				Source:     "key",
				Expression: ".*val.*",
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"pokey": "pal1",
			},
			shouldDrop: false,
		},
		{
			name: "Regex Did Not Match Line",
			config: &DropConfig{
				Expression: ".*val.*",
			},
			labels:     model.LabelSet{},
			entry:      "this is a line which does not match the regex",
			extracted:  map[string]any{},
			shouldDrop: false,
		},
		{
			name: "Regex Matched Line",
			config: &DropConfig{
				Expression: ".*val.*",
			},
			labels:     model.LabelSet{},
			entry:      "this is a line with the word value in it",
			extracted:  map[string]any{},
			shouldDrop: true,
		},
		{
			name: "Match Source and Length Both Match",
			config: &DropConfig{
				Source:     "key",
				LongerThan: tenBytes,
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"key": "pal1",
			},
			entry:      "12345678901",
			shouldDrop: true,
		},
		{
			name: "Match Source and Length Only First Matches",
			config: &DropConfig{
				Source:     "key",
				LongerThan: tenBytes,
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"key": "pal1",
			},
			entry:      "123456789",
			shouldDrop: false,
		},
		{
			name: "Match Source and Length Only Second Matches",
			config: &DropConfig{
				Source:     "key",
				LongerThan: tenBytes,
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"WOOOOOOOOOOOOOO": "pal1",
			},
			entry:      "123456789012",
			shouldDrop: false,
		},
		{
			name: "Everything Must Match",
			config: &DropConfig{
				Source:     "key",
				Expression: ".*val.*",
				OlderThan:  oneHour,
				LongerThan: tenBytes,
			},
			labels: model.LabelSet{},
			extracted: map[string]any{
				"key": "must contain value to match",
			},
			t:          time.Now().Add(-2 * time.Hour),
			entry:      "12345678901",
			shouldDrop: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateDropConfig(tt.config)
			if err != nil {
				t.Error(err)
			}
			logger := util.TestAlloyLogger(t)
			m, err := newDropStage(logger, *tt.config, prometheus.DefaultRegisterer)
			require.NoError(t, err)
			out := processEntries(m, newEntry(tt.extracted, tt.labels, tt.entry, tt.t))
			if tt.shouldDrop {
				assert.Len(t, out, 0)
			} else {
				assert.Len(t, out, 1)
			}
		})
	}
}

func TestDropPipeline(t *testing.T) {
	registry := prometheus.NewRegistry()
	plName := "test_drop_pipeline"
	logger := util.TestAlloyLogger(t)
	pl, err := NewPipeline(logger, loadConfig(testDropAlloy), &plName, registry, featuregate.StabilityGenerallyAvailable)
	require.NoError(t, err)
	out := processEntries(pl,
		newEntry(nil, nil, testMatchLogLineApp1, time.Now()),
		newEntry(nil, nil, testMatchLogLineApp2, time.Now()),
	)

	// Only the second line will go through.
	assert.Len(t, out, 1)
	assert.Equal(t, out[0].Line, testMatchLogLineApp2)
}

func Test_validateDropConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *DropConfig
		wantErr error
	}{
		{
			name:    "ErrEmpty",
			config:  &DropConfig{},
			wantErr: errors.New(ErrDropStageEmptyConfig),
		},
		{
			name: "Invalid Regex",
			config: &DropConfig{
				Expression: "(?P<ts[0-9]+).*",
			},
			wantErr: fmt.Errorf(ErrDropStageInvalidRegex, "error parsing regexp: invalid named capture: `(?P<ts[0-9]+).*`"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := validateDropConfig(tt.config); ((err != nil) && (err.Error() != tt.wantErr.Error())) || (err == nil && tt.wantErr != nil) {
				t.Errorf("validateDropConfig() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
