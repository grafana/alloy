package logql

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestSelectorUnmarshalText(t *testing.T) {
	type testCase struct {
		name      string
		selector  string
		expectErr bool
	}

	tests := []testCase{
		{
			name:      "empty",
			selector:  ``,
			expectErr: true,
		},
		{
			name:     "matchers only",
			selector: `{app="foo"}`,
		},
		{
			name:     "multiple matchers",
			selector: `{app="foo", env=~"prod|dev", team!="a", tier!~"b.*"}`,
		},
		{
			name:     "contains filter",
			selector: `{app="foo"} |= "error"`,
		},
		{
			name:     "not contains filter",
			selector: `{app="foo"} != "error"`,
		},
		{
			name:     "regex filter",
			selector: `{app="foo"} |~ "err.*"`,
		},
		{
			name:     "not regex filter",
			selector: `{app="foo"} !~ "err.*"`,
		},
		{
			name:     "chained filters",
			selector: `{app="foo"} |= "a" != "b" |~ "c" !~ "d"`,
		},
		{
			name:     "or filter",
			selector: `{app="foo"} |= "a" or "b"`,
		},
		{
			name:     "or filter with not contains",
			selector: `{app="foo"} != "a" or "b"`,
		},
		{
			name:      "no matchers",
			selector:  `{}`,
			expectErr: true,
		},
		{
			name:      "missing braces",
			selector:  `foo`,
			expectErr: true,
		},
		{
			name:      "unclosed brace",
			selector:  `{`,
			expectErr: true,
		},
		{
			name:      "unclosed quote",
			selector:  `{app="foo}`,
			expectErr: true,
		},
		{
			name:      "invalid regex filter",
			selector:  `{app="foo"} |~ "[]"`,
			expectErr: true,
		},
		{
			name:      "invalid not regex filter",
			selector:  `{app="foo"} !~ "[]"`,
			expectErr: true,
		},
		{
			name:      "pattern filter",
			selector:  `{app="foo"} |> "<_> error <_>"`,
			expectErr: true,
		},
		{
			name:      "not pattern filter",
			selector:  `{app="foo"} !> "<_> error <_>"`,
			expectErr: true,
		},
		{
			name:      "ip filter",
			selector:  `{app="foo"} |= ip("10.0.0.0/8")`,
			expectErr: true,
		},
		{
			name:      "not ip filter",
			selector:  `{app="foo"} != ip("10.0.0.0/8")`,
			expectErr: true,
		},
		{
			name:      "ip filter in or",
			selector:  `{app="foo"} |= "a" or ip("10.0.0.0/8")`,
			expectErr: true,
		},
		{
			name:      "json parser",
			selector:  `{app="foo"} | json`,
			expectErr: true,
		},
		{
			name:      "logfmt parser",
			selector:  `{app="foo"} | logfmt`,
			expectErr: true,
		},
		{
			name:      "label filter",
			selector:  `{app="foo"} | status >= 200`,
			expectErr: true,
		},
		{
			name:      "line format",
			selector:  `{app="foo"} | line_format "{{.msg}}"`,
			expectErr: true,
		},
		{
			name:      "line filter after parser",
			selector:  `{app="foo"} | json |= "error"`,
			expectErr: true,
		},
		{
			name:      "metric query",
			selector:  `rate({app="foo"}[1m])`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s Selector
			err := s.UnmarshalText([]byte(tt.selector))
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSelectorMarshalText(t *testing.T) {
	const text = `{app="foo"} |= "error" or "warn"`

	var s Selector
	require.NoError(t, s.UnmarshalText([]byte(text)))

	got, err := s.MarshalText()
	require.NoError(t, err)
	require.Equal(t, text, string(got))
}

func TestSelectorMatches(t *testing.T) {
	type testCase struct {
		name     string
		selector string
		labels   model.LabelSet
		line     string
		expected bool
	}

	tests := []testCase{
		{
			name:     "equal matcher matches",
			selector: `{app="foo"}`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "anything",
			expected: true,
		},
		{
			name:     "equal matcher does not match",
			selector: `{app="foo"}`,
			labels:   model.LabelSet{"app": "bar"},
			line:     "anything",
			expected: false,
		},
		{
			name:     "missing label does not match equal matcher",
			selector: `{app="foo"}`,
			labels:   model.LabelSet{},
			line:     "anything",
			expected: false,
		},
		{
			name:     "missing label matches not equal matcher",
			selector: `{app!="foo"}`,
			labels:   model.LabelSet{},
			line:     "anything",
			expected: true,
		},
		{
			name:     "regex matcher is anchored",
			selector: `{app=~"fo"}`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "anything",
			expected: false,
		},
		{
			name:     "regex matcher matches",
			selector: `{app=~"foo|bar"}`,
			labels:   model.LabelSet{"app": "bar"},
			line:     "anything",
			expected: true,
		},
		{
			name:     "not regex matcher matches",
			selector: `{app!~"foo|bar"}`,
			labels:   model.LabelSet{"app": "baz"},
			line:     "anything",
			expected: true,
		},
		{
			name:     "all matchers must match",
			selector: `{app="foo", env="prod"}`,
			labels:   model.LabelSet{"app": "foo", "env": "dev"},
			line:     "anything",
			expected: false,
		},
		{
			name:     "contains filter matches",
			selector: `{app="foo"} |= "error"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "an error happened",
			expected: true,
		},
		{
			name:     "contains filter does not match",
			selector: `{app="foo"} |= "error"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "all good",
			expected: false,
		},
		{
			name:     "line filter is not evaluated when matchers do not match",
			selector: `{app="foo"} |= "error"`,
			labels:   model.LabelSet{"app": "bar"},
			line:     "an error happened",
			expected: false,
		},
		{
			name:     "not contains filter matches",
			selector: `{app="foo"} != "error"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "all good",
			expected: true,
		},
		{
			name:     "not contains filter does not match",
			selector: `{app="foo"} != "error"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "an error happened",
			expected: false,
		},
		{
			name:     "regex filter matches",
			selector: `{app="foo"} |~ "err(or|s)"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "two errs",
			expected: true,
		},
		{
			name:     "regex filter does not match",
			selector: `{app="foo"} |~ "err(or|s)"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "all good",
			expected: false,
		},
		{
			name:     "case insensitive regex filter matches",
			selector: `{app="foo"} |~ "(?i)error"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "an ERROR happened",
			expected: true,
		},
		{
			name:     "not regex filter matches",
			selector: `{app="foo"} !~ "err.*"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "all good",
			expected: true,
		},
		{
			name:     "not regex filter does not match",
			selector: `{app="foo"} !~ "err.*"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "an error happened",
			expected: false,
		},
		{
			name:     "chained filters are anded and all pass",
			selector: `{app="foo"} |= "error" != "timeout"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "an error happened",
			expected: true,
		},
		{
			name:     "chained filters are anded and one fails",
			selector: `{app="foo"} |= "error" != "timeout"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "an error timeout happened",
			expected: false,
		},
		{
			name:     "or filter matches first",
			selector: `{app="foo"} |= "error" or "warn"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "an error happened",
			expected: true,
		},
		{
			name:     "or filter matches second",
			selector: `{app="foo"} |= "error" or "warn"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "a warn happened",
			expected: true,
		},
		{
			name:     "or filter matches none",
			selector: `{app="foo"} |= "error" or "warn"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "all good",
			expected: false,
		},
		{
			name:     "or filter with not contains excludes first",
			selector: `{app="foo"} != "error" or "warn"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "an error happened",
			expected: false,
		},
		{
			name:     "or filter with not contains excludes second",
			selector: `{app="foo"} != "error" or "warn"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "a warn happened",
			expected: false,
		},
		{
			name:     "or filter with not contains matches neither",
			selector: `{app="foo"} != "error" or "warn"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "all good",
			expected: true,
		},
		{
			name:     "empty line",
			selector: `{app="foo"} |= "error"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "",
			expected: false,
		},
		{
			name:     "empty line with not contains",
			selector: `{app="foo"} != "error"`,
			labels:   model.LabelSet{"app": "foo"},
			line:     "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s Selector
			require.NoError(t, s.UnmarshalText([]byte(tt.selector)))
			require.Equal(t, tt.expected, s.Matches(tt.labels, tt.line))
		})
	}
}
