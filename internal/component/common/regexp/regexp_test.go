package regexp

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

func TestRegexpUnmarshalText(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{name: "valid", input: `^(?P<ts>[0-9]+)`},
		{name: "empty", input: ``},
		{name: "invalid", input: `[unclosed`, expectErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var re Regexp
			err := re.UnmarshalText([]byte(tt.input))
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.input, re.String())
		})
	}
}

func TestRegexpMarshalText(t *testing.T) {
	text, err := MustCompile(`^(?P<ts>[0-9]+)`).MarshalText()
	require.NoError(t, err)
	require.Equal(t, `^(?P<ts>[0-9]+)`, string(text))

	text, err = Regexp{}.MarshalText()
	require.NoError(t, err)
	require.Empty(t, text)
}

func TestRegexpIsEmpty(t *testing.T) {
	var nilPtr *Regexp
	require.True(t, nilPtr.IsEmpty())
	require.True(t, (&Regexp{}).IsEmpty())
	require.True(t, MustCompile(``).IsEmpty())
	require.False(t, MustCompile(`a`).IsEmpty())
}

func TestMustCompilePanics(t *testing.T) {
	require.Panics(t, func() { MustCompile(`[unclosed`) })
}

func TestRegexpAlloyDecode(t *testing.T) {
	type config struct {
		Expression *Regexp `alloy:"expression,attr"`
	}

	var cfg config
	require.NoError(t, syntax.Unmarshal([]byte(`expression = "^(?P<ts>[0-9]+)"`), &cfg))
	require.Equal(t, []string{"", "ts"}, cfg.Expression.SubexpNames())

	err := syntax.Unmarshal([]byte(`expression = "[unclosed"`), &cfg)
	require.Error(t, err)
}
