package syntaxtags_test

import (
	"reflect"
	"testing"

	"github.com/grafana/alloy/syntax/internal/syntaxtags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Get(t *testing.T) {
	type Struct struct {
		IgnoreMe bool

		ReqAttr  string     `alloy:"req_attr,attr"`
		OptAttr  string     `alloy:"opt_attr,attr,optional"`
		ReqBlock struct{}   `alloy:"req_block,block"`
		OptBlock struct{}   `alloy:"opt_block,block,optional"`
		ReqEnum  []struct{} `alloy:"req_enum,enum"`
		OptEnum  []struct{} `alloy:"opt_enum,enum,optional"`
		Label    string     `alloy:",label"`
	}

	fs := syntaxtags.Get(reflect.TypeOf(Struct{}))

	expect := []syntaxtags.Field{
		{[]string{"req_attr"}, []int{1}, syntaxtags.FlagAttr},
		{[]string{"opt_attr"}, []int{2}, syntaxtags.FlagAttr | syntaxtags.FlagOptional},
		{[]string{"req_block"}, []int{3}, syntaxtags.FlagBlock},
		{[]string{"opt_block"}, []int{4}, syntaxtags.FlagBlock | syntaxtags.FlagOptional},
		{[]string{"req_enum"}, []int{5}, syntaxtags.FlagEnum},
		{[]string{"opt_enum"}, []int{6}, syntaxtags.FlagEnum | syntaxtags.FlagOptional},
		{[]string{""}, []int{7}, syntaxtags.FlagLabel},
	}

	require.Equal(t, expect, fs)
}

func TestEmbedded(t *testing.T) {
	type InnerStruct struct {
		InnerField1 string `alloy:"inner_field_1,attr"`
		InnerField2 string `alloy:"inner_field_2,attr"`
	}

	type Struct struct {
		Field1 string `alloy:"parent_field_1,attr"`
		InnerStruct
		Field2 string `alloy:"parent_field_2,attr"`
	}
	require.PanicsWithValue(t, "syntax: anonymous fields not supported syntaxtags_test.Struct.InnerStruct", func() { syntaxtags.Get(reflect.TypeOf(Struct{})) })
}

func TestSquash(t *testing.T) {
	type InnerStruct struct {
		InnerField1 string `alloy:"inner_field_1,attr"`
		InnerField2 string `alloy:"inner_field_2,attr"`
	}

	type Struct struct {
		Field1 string      `alloy:"parent_field_1,attr"`
		Inner  InnerStruct `alloy:",squash"`
		Field2 string      `alloy:"parent_field_2,attr"`
	}

	type StructWithPointer struct {
		Field1 string       `alloy:"parent_field_1,attr"`
		Inner  *InnerStruct `alloy:",squash"`
		Field2 string       `alloy:"parent_field_2,attr"`
	}

	expect := []syntaxtags.Field{
		{
			Name:  []string{"parent_field_1"},
			Index: []int{0},
			Flags: syntaxtags.FlagAttr,
		},
		{
			Name:  []string{"inner_field_1"},
			Index: []int{1, 0},
			Flags: syntaxtags.FlagAttr,
		},
		{
			Name:  []string{"inner_field_2"},
			Index: []int{1, 1},
			Flags: syntaxtags.FlagAttr,
		},
		{
			Name:  []string{"parent_field_2"},
			Index: []int{2},
			Flags: syntaxtags.FlagAttr,
		},
	}

	structActual := syntaxtags.Get(reflect.TypeOf(Struct{}))
	assert.Equal(t, expect, structActual)

	structPointerActual := syntaxtags.Get(reflect.TypeOf(StructWithPointer{}))
	assert.Equal(t, expect, structPointerActual)
}

func TestDeepSquash(t *testing.T) {
	type Inner2Struct struct {
		InnerField1 string `alloy:"inner_field_1,attr"`
		InnerField2 string `alloy:"inner_field_2,attr"`
	}

	type InnerStruct struct {
		Inner2Struct Inner2Struct `alloy:",squash"`
	}

	type Struct struct {
		Inner InnerStruct `alloy:",squash"`
	}

	expect := []syntaxtags.Field{
		{
			Name:  []string{"inner_field_1"},
			Index: []int{0, 0, 0},
			Flags: syntaxtags.FlagAttr,
		},
		{
			Name:  []string{"inner_field_2"},
			Index: []int{0, 0, 1},
			Flags: syntaxtags.FlagAttr,
		},
	}

	structActual := syntaxtags.Get(reflect.TypeOf(Struct{}))
	assert.Equal(t, expect, structActual)
}

func Test_Get_Panics(t *testing.T) {
	expectPanic := func(t *testing.T, expect string, v any) {
		t.Helper()
		require.PanicsWithValue(t, expect, func() {
			_ = syntaxtags.Get(reflect.TypeOf(v))
		})
	}

	t.Run("Tagged fields must be exported", func(t *testing.T) {
		type Struct struct {
			attr string `alloy:"field,attr"` // nolint:unused //nolint:syntaxtags
		}
		expect := `syntax: alloy tag found on unexported field at syntaxtags_test.Struct.attr`
		expectPanic(t, expect, Struct{})
	})

	t.Run("Options are required", func(t *testing.T) {
		type Struct struct {
			Attr string `alloy:"field"` //nolint:syntaxtags
		}
		expect := `syntax: field syntaxtags_test.Struct.Attr tag is missing options`
		expectPanic(t, expect, Struct{})
	})

	t.Run("Field names must be unique", func(t *testing.T) {
		type Struct struct {
			Attr  string `alloy:"field1,attr"`
			Block string `alloy:"field1,block,optional"` //nolint:syntaxtags
		}
		expect := `syntax: field name field1 already used by syntaxtags_test.Struct.Attr`
		expectPanic(t, expect, Struct{})
	})

	t.Run("Name is required for non-label field", func(t *testing.T) {
		type Struct struct {
			Attr string `alloy:",attr"` //nolint:syntaxtags
		}
		expect := `syntaxtags: non-empty field name required at syntaxtags_test.Struct.Attr`
		expectPanic(t, expect, Struct{})
	})

	t.Run("Only one label field may exist", func(t *testing.T) {
		type Struct struct {
			Label1 string `alloy:",label"`
			Label2 string `alloy:",label"`
		}
		expect := `syntax: label field already used by syntaxtags_test.Struct.Label2`
		expectPanic(t, expect, Struct{})
	})
}
