package syntaxtags_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax/internal/syntaxtags"
)

type textType struct{ v string }

func (t *textType) UnmarshalText(b []byte) error { t.v = string(b); return nil }

type msInner struct {
	Endpoint string `mapstructure:"endpoint"`
}

type msSquashed struct {
	Timeout time.Duration `mapstructure:"timeout"`
}

type msConfig struct {
	msSquashed `mapstructure:",squash"`

	Name       string             `mapstructure:"name"`
	Untagged   int                // No tag: the Go field name is used.
	Skipped    string             `mapstructure:"-"`
	Text       textType           `mapstructure:"text"`
	Inner      msInner            `mapstructure:"inner"`
	InnerPtr   *msInner           `mapstructure:"inner_ptr"`
	Repeated   []msInner          `mapstructure:"repeated"`
	Headers    map[string]string  `mapstructure:"headers"`
	ByName     map[string]msInner `mapstructure:"by_name"`
	Texts      []textType         `mapstructure:"texts"`
	unexported string             //nolint:unused
}

func TestGetMapstructure(t *testing.T) {
	attr := syntaxtags.FlagAttr | syntaxtags.FlagOptional
	block := syntaxtags.FlagBlock | syntaxtags.FlagOptional

	expect := []syntaxtags.Field{
		{Name: []string{"timeout"}, Index: []int{0, 0}, Flags: attr},
		{Name: []string{"name"}, Index: []int{1}, Flags: attr},
		{Name: []string{"Untagged"}, Index: []int{2}, Flags: attr},
		{Name: []string{"text"}, Index: []int{4}, Flags: attr},
		{Name: []string{"inner"}, Index: []int{5}, Flags: block},
		{Name: []string{"inner_ptr"}, Index: []int{6}, Flags: block},
		{Name: []string{"repeated"}, Index: []int{7}, Flags: block},
		{Name: []string{"headers"}, Index: []int{8}, Flags: attr},
		{Name: []string{"by_name"}, Index: []int{9}, Flags: attr},
		{Name: []string{"texts"}, Index: []int{10}, Flags: attr},
	}

	require.Equal(t, expect, syntaxtags.GetMapstructure(reflect.TypeFor[msConfig]()))

	// Struct types reached from the config are now interpreted with mapstructure
	// tags too.
	require.Equal(t, []syntaxtags.Field{
		{Name: []string{"endpoint"}, Index: []int{0}, Flags: attr},
	}, syntaxtags.Get(reflect.TypeFor[msInner]()))
}

func TestGet_SquashMapstructure(t *testing.T) {
	type wrapper struct {
		Config msInner `alloy:",squash,mapstructure"`
		Output string  `alloy:"output,attr,optional"`
	}

	require.Equal(t, []syntaxtags.Field{
		{Name: []string{"endpoint"}, Index: []int{0, 0}, Flags: syntaxtags.FlagAttr | syntaxtags.FlagOptional},
		{Name: []string{"output"}, Index: []int{1}, Flags: syntaxtags.FlagAttr | syntaxtags.FlagOptional},
	}, syntaxtags.Get(reflect.TypeFor[wrapper]()))
}

func TestGetMapstructure_InvalidIdentifier(t *testing.T) {
	type bad struct {
		Field string `mapstructure:"not-valid"`
	}
	require.PanicsWithValue(t,
		`syntaxtags: mapstructure name "not-valid" at syntaxtags_test.bad.Field is not a valid identifier`,
		func() { syntaxtags.GetMapstructure(reflect.TypeFor[bad]()) },
	)
}
