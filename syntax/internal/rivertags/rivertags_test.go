package rivertags_test

import (
	"reflect"
	"testing"

	"github.com/grafana/river/internal/rivertags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Get(t *testing.T) {
	type Struct struct {
		IgnoreMe bool

		ReqAttr  string     `river:"req_attr,attr"`
		OptAttr  string     `river:"opt_attr,attr,optional"`
		ReqBlock struct{}   `river:"req_block,block"`
		OptBlock struct{}   `river:"opt_block,block,optional"`
		ReqEnum  []struct{} `river:"req_enum,enum"`
		OptEnum  []struct{} `river:"opt_enum,enum,optional"`
		Label    string     `river:",label"`
	}

	fs := rivertags.Get(reflect.TypeOf(Struct{}))

	expect := []rivertags.Field{
		{[]string{"req_attr"}, []int{1}, rivertags.FlagAttr},
		{[]string{"opt_attr"}, []int{2}, rivertags.FlagAttr | rivertags.FlagOptional},
		{[]string{"req_block"}, []int{3}, rivertags.FlagBlock},
		{[]string{"opt_block"}, []int{4}, rivertags.FlagBlock | rivertags.FlagOptional},
		{[]string{"req_enum"}, []int{5}, rivertags.FlagEnum},
		{[]string{"opt_enum"}, []int{6}, rivertags.FlagEnum | rivertags.FlagOptional},
		{[]string{""}, []int{7}, rivertags.FlagLabel},
	}

	require.Equal(t, expect, fs)
}

func TestEmbedded(t *testing.T) {
	type InnerStruct struct {
		InnerField1 string `river:"inner_field_1,attr"`
		InnerField2 string `river:"inner_field_2,attr"`
	}

	type Struct struct {
		Field1 string `river:"parent_field_1,attr"`
		InnerStruct
		Field2 string `river:"parent_field_2,attr"`
	}
	require.PanicsWithValue(t, "river: anonymous fields not supported rivertags_test.Struct.InnerStruct", func() { rivertags.Get(reflect.TypeOf(Struct{})) })
}

func TestSquash(t *testing.T) {
	type InnerStruct struct {
		InnerField1 string `river:"inner_field_1,attr"`
		InnerField2 string `river:"inner_field_2,attr"`
	}

	type Struct struct {
		Field1 string      `river:"parent_field_1,attr"`
		Inner  InnerStruct `river:",squash"`
		Field2 string      `river:"parent_field_2,attr"`
	}

	type StructWithPointer struct {
		Field1 string       `river:"parent_field_1,attr"`
		Inner  *InnerStruct `river:",squash"`
		Field2 string       `river:"parent_field_2,attr"`
	}

	expect := []rivertags.Field{
		{
			Name:  []string{"parent_field_1"},
			Index: []int{0},
			Flags: rivertags.FlagAttr,
		},
		{
			Name:  []string{"inner_field_1"},
			Index: []int{1, 0},
			Flags: rivertags.FlagAttr,
		},
		{
			Name:  []string{"inner_field_2"},
			Index: []int{1, 1},
			Flags: rivertags.FlagAttr,
		},
		{
			Name:  []string{"parent_field_2"},
			Index: []int{2},
			Flags: rivertags.FlagAttr,
		},
	}

	structActual := rivertags.Get(reflect.TypeOf(Struct{}))
	assert.Equal(t, expect, structActual)

	structPointerActual := rivertags.Get(reflect.TypeOf(StructWithPointer{}))
	assert.Equal(t, expect, structPointerActual)
}

func TestDeepSquash(t *testing.T) {
	type Inner2Struct struct {
		InnerField1 string `river:"inner_field_1,attr"`
		InnerField2 string `river:"inner_field_2,attr"`
	}

	type InnerStruct struct {
		Inner2Struct Inner2Struct `river:",squash"`
	}

	type Struct struct {
		Inner InnerStruct `river:",squash"`
	}

	expect := []rivertags.Field{
		{
			Name:  []string{"inner_field_1"},
			Index: []int{0, 0, 0},
			Flags: rivertags.FlagAttr,
		},
		{
			Name:  []string{"inner_field_2"},
			Index: []int{0, 0, 1},
			Flags: rivertags.FlagAttr,
		},
	}

	structActual := rivertags.Get(reflect.TypeOf(Struct{}))
	assert.Equal(t, expect, structActual)
}

func Test_Get_Panics(t *testing.T) {
	expectPanic := func(t *testing.T, expect string, v interface{}) {
		t.Helper()
		require.PanicsWithValue(t, expect, func() {
			_ = rivertags.Get(reflect.TypeOf(v))
		})
	}

	t.Run("Tagged fields must be exported", func(t *testing.T) {
		type Struct struct {
			attr string `river:"field,attr"` // nolint:unused //nolint:rivertags
		}
		expect := `river: river tag found on unexported field at rivertags_test.Struct.attr`
		expectPanic(t, expect, Struct{})
	})

	t.Run("Options are required", func(t *testing.T) {
		type Struct struct {
			Attr string `river:"field"` //nolint:rivertags
		}
		expect := `river: field rivertags_test.Struct.Attr tag is missing options`
		expectPanic(t, expect, Struct{})
	})

	t.Run("Field names must be unique", func(t *testing.T) {
		type Struct struct {
			Attr  string `river:"field1,attr"`
			Block string `river:"field1,block,optional"` //nolint:rivertags
		}
		expect := `river: field name field1 already used by rivertags_test.Struct.Attr`
		expectPanic(t, expect, Struct{})
	})

	t.Run("Name is required for non-label field", func(t *testing.T) {
		type Struct struct {
			Attr string `river:",attr"` //nolint:rivertags
		}
		expect := `river: non-empty field name required at rivertags_test.Struct.Attr`
		expectPanic(t, expect, Struct{})
	})

	t.Run("Only one label field may exist", func(t *testing.T) {
		type Struct struct {
			Label1 string `river:",label"`
			Label2 string `river:",label"`
		}
		expect := `river: label field already used by rivertags_test.Struct.Label2`
		expectPanic(t, expect, Struct{})
	})
}
