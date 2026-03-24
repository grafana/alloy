package equality

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeepEqual(t *testing.T) {
	type args struct {
		x any
		y any
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "numbers equal",
			args: args{x: 123, y: 123},
			want: true,
		},
		{
			name: "strings equal",
			args: args{x: "123", y: "123"},
			want: true,
		},
		{
			name: "strings unequal",
			args: args{x: "not 123", y: "123"},
			want: false,
		},
		{
			name: "structs equal",
			args: args{x: justAStruct{123, 13.6}, y: justAStruct{123, 13.6}},
			want: true,
		},
		{
			name: "structs unequal",
			args: args{x: justAStruct{123, 13.6}, y: justAStruct{123, 3.14}},
			want: false,
		},
		{
			name: "strings unequal",
			args: args{x: "not 123", y: "123"},
			want: false,
		},
		{
			name: "strings slice equal",
			args: args{x: []string{"123", "567"}, y: []string{"123", "567"}},
			want: true,
		},
		{
			name: "strings slice unequal",
			args: args{x: []string{"123", "567"}, y: []string{"1234", "5678"}},
			want: false,
		},
		{
			name: "custom struct equal",
			args: args{x: equalsTester{"same__"}, y: equalsTester{"length"}},
			want: true, // note: equalsTester defined equality as length of its string matching!
		},
		{
			name: "custom struct unequal",
			args: args{x: equalsTester{"different"}, y: equalsTester{"lengths"}},
			want: false,
		},
		{
			name: "custom struct pointer equal",
			args: args{x: &equalsTester{"same__"}, y: &equalsTester{"length"}},
			want: true,
		},
		{
			name: "custom struct pointer unequal",
			args: args{x: &equalsTester{"different"}, y: &equalsTester{"lengths"}},
			want: false,
		},
		{
			name: "custom struct pointer unequal with nil",
			args: args{x: &equalsTester{"different"}, y: nil},
			want: false,
		},
		{
			// Even though values are equal, we're comparing different types and this is not supported. This behaviour
			// matches reflect.DeepEqual.
			name: "custom struct and pointer cannot be equal",
			args: args{x: &equalsTester{"same__"}, y: equalsTester{"length"}},
			want: false,
		},
		{
			name: "custom struct array equal",
			args: args{x: [2]equalsTester{{"1"}, {"2"}}, y: [2]equalsTester{{"a"}, {"b"}}},
			want: true,
		},
		{
			name: "custom struct array unequal",
			args: args{x: [2]equalsTester{{"1"}, {"2"}}, y: [2]equalsTester{{"different_length"}, {"b"}}},
			want: false,
		},
		{
			name: "custom pointer array equal",
			args: args{x: [2]*equalsTester{{"1"}, {"2"}}, y: [2]*equalsTester{{"a"}, {"b"}}},
			want: true,
		},
		{
			name: "custom pointer array unequal",
			args: args{x: [2]*equalsTester{{"1"}, {"2"}}, y: [2]*equalsTester{{"different_length"}, {"b"}}},
			want: false,
		},
		{
			name: "custom struct slice equal",
			args: args{x: []equalsTester{{"1"}, {"2"}}, y: []equalsTester{{"a"}, {"b"}}},
			want: true,
		},
		{
			name: "custom struct slice unequal",
			args: args{x: []equalsTester{{"1"}, {"2"}}, y: []equalsTester{{"different_length"}, {"b"}}},
			want: false,
		},
		{
			name: "custom pointer slice equal",
			args: args{x: []*equalsTester{{"1"}, {"2"}}, y: []*equalsTester{{"a"}, {"b"}}},
			want: true,
		},
		{
			name: "custom pointer slice unequal",
			args: args{x: []*equalsTester{{"1"}, {"2"}}, y: []*equalsTester{{"different_length"}, {"b"}}},
			want: false,
		},
		{
			name: "custom pointer slice unequal lengths",
			args: args{x: []*equalsTester{{"1"}, {"2"}}, y: []*equalsTester{}},
			want: false,
		},
		{
			name: "custom pointer slice with nil",
			args: args{x: []*equalsTester{{"1"}, {"2"}}, y: nil},
			want: false,
		},
		{
			name: "mixed fields structs are not supported",
			args: args{
				x: mixedFieldsStruct{
					i:  123,
					e1: equalsTester{"1234"},
					e2: equalsTester{"ab"},
					s:  justAStruct{567, 3.14},
				},
				y: mixedFieldsStruct{
					i:  123,
					e1: equalsTester{"abcd"},
					e2: equalsTester{"12"},
					s:  justAStruct{567, 3.14},
				},
			},
			want: false,
		},
		{
			name: "all custom unexported fields struct not supported",
			args: args{
				x: allCustomFieldsStructUnexported{
					e1: equalsTester{"1234"},
					e2: equalsTester{"ab"},
				},
				y: allCustomFieldsStructUnexported{
					e1: equalsTester{"abcd"},
					e2: equalsTester{"12"},
				},
			},
			want: false,
		},
		{
			name: "all custom exported fields struct equal",
			args: args{
				x: allCustomFieldsStructExported{
					E1: equalsTester{"1234"},
					E2: equalsTester{"ab"},
				},
				y: allCustomFieldsStructExported{
					E1: equalsTester{"abcd"},
					E2: equalsTester{"12"},
				},
			},
			want: true,
		},
		{
			name: "all custom exported fields struct unequal",
			args: args{
				x: allCustomFieldsStructExported{
					E1: equalsTester{"1234"},
					E2: equalsTester{"ab"},
				},
				y: allCustomFieldsStructExported{
					E1: equalsTester{"abcd"},
					E2: equalsTester{"this is too long"},
				},
			},
			want: false,
		},
		{
			name: "custom struct map equal",
			args: args{
				x: map[string]equalsTester{"a": {"1"}, "b": {"12"}},
				y: map[string]equalsTester{"a": {"a"}, "b": {"ab"}},
			},
			want: true,
		},
		{
			name: "custom struct map unequal",
			args: args{
				x: map[string]equalsTester{"a": {"1"}, "b": {"12"}},
				y: map[string]equalsTester{"a": {"a"}, "b": {"too long"}},
			},
			want: false,
		},
		{
			name: "custom struct map unequal different keys",
			args: args{
				x: map[string]equalsTester{"a": {"1"}, "b": {"12"}},
				y: map[string]equalsTester{"a": {"a"}, "c": {"ab"}},
			},
			want: false,
		},
		{
			name: "custom struct map unequal different length",
			args: args{
				x: map[string]equalsTester{"a": {"1"}, "b": {"12"}},
				y: map[string]equalsTester{"a": {"a"}, "b": {"ab"}, "c": {""}},
			},
			want: false,
		},
		{
			name: "custom struct map unequal nil",
			args: args{
				x: map[string]equalsTester{"a": {"1"}, "b": {"12"}},
				y: nil,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DeepEqual(tt.args.x, tt.args.y))
			assert.Equal(t, tt.want, DeepEqual(tt.args.y, tt.args.x))
		})
	}
}

type justAStruct struct {
	i int
	f float64
}

type mixedFieldsStruct struct {
	i  int
	e1 equalsTester
	e2 equalsTester
	s  justAStruct
}

type allCustomFieldsStructUnexported struct {
	e1 equalsTester
	e2 equalsTester
}

type allCustomFieldsStructExported struct {
	E1 equalsTester
	E2 equalsTester
}

type equalsTester struct {
	s string
}

func (t equalsTester) Equals(other any) bool {
	if o, ok := other.(*equalsTester); ok {
		// Special kind of equals - considered equal if lengths of strings are the same.
		return len(t.s) == len(o.s)
	}
	return false
}
