package reflectutil_test

import (
	"reflect"
	"testing"

	"github.com/grafana/river/internal/reflectutil"
	"github.com/grafana/river/internal/rivertags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeeplyNested_Access(t *testing.T) {
	type Struct struct {
		Field1 struct {
			Field2 struct {
				Field3 struct {
					Value string
				}
			}
		}
	}

	var s Struct
	s.Field1.Field2.Field3.Value = "Hello, world!"

	rv := reflect.ValueOf(&s).Elem()
	innerValue := reflectutil.GetOrAlloc(rv, rivertags.Field{Index: []int{0, 0, 0, 0}})
	assert.True(t, innerValue.CanSet())
	assert.Equal(t, reflect.String, innerValue.Kind())
}

func TestDeeplyNested_Allocate(t *testing.T) {
	type Struct struct {
		Field1 *struct {
			Field2 *struct {
				Field3 *struct {
					Value string
				}
			}
		}
	}

	var s Struct

	rv := reflect.ValueOf(&s).Elem()
	innerValue := reflectutil.GetOrAlloc(rv, rivertags.Field{Index: []int{0, 0, 0, 0}})
	require.True(t, innerValue.CanSet())
	require.Equal(t, reflect.String, innerValue.Kind())

	innerValue.Set(reflect.ValueOf("Hello, world!"))
	require.Equal(t, "Hello, world!", s.Field1.Field2.Field3.Value)
}

func TestDeeplyNested_NoAllocate(t *testing.T) {
	type Struct struct {
		Field1 *struct {
			Field2 *struct {
				Field3 *struct {
					Value string
				}
			}
		}
	}

	var s Struct

	rv := reflect.ValueOf(&s).Elem()
	innerValue := reflectutil.Get(rv, rivertags.Field{Index: []int{0, 0, 0, 0}})
	assert.False(t, innerValue.CanSet())
	assert.Equal(t, reflect.String, innerValue.Kind())
}
