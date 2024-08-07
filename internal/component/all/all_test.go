package all

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetDefault_NoPointerReuse ensures that calls to SetDefault do not re-use
// pointers. The test iterates through all registered components, and then
// recursively traverses through its Arguments type to guarantee that no two
// calls to SetDefault result in pointer reuse.
//
// Nested types that also implement syntax.Defaulter are also checked.
func TestSetDefault_NoPointerReuse(t *testing.T) {
	allComponents := component.AllNames()
	for _, componentName := range allComponents {
		reg, ok := component.Get(componentName)
		require.True(t, ok, "Expected component %q to exist", componentName)

		t.Run(reg.Name, func(t *testing.T) {
			testNoReusePointer(t, reg)
		})
	}
}

func testNoReusePointer(t *testing.T, reg component.Registration) {
	t.Helper()

	var (
		args1 = reg.CloneArguments()
		args2 = reg.CloneArguments()
	)

	if args1, ok := args1.(syntax.Defaulter); ok {
		args1.SetToDefault()
	}
	if args2, ok := args2.(syntax.Defaulter); ok {
		args2.SetToDefault()
	}

	rv1, rv2 := reflect.ValueOf(args1), reflect.ValueOf(args2)
	ty := rv1.Type().Elem()

	// Edge case: if the component's arguments type is an empty struct, skip.
	// Not skipping causes the test to fail, due to an optimization in
	// reflect.New where initializing the same zero-length object results in the
	// same pointer.
	if rv1.Elem().NumField() == 0 {
		return
	}

	if path, shared := util.SharePointer(rv1, rv2, true); shared {
		fullPath := fmt.Sprintf("%s.%s.%s", ty.PkgPath(), ty.Name(), path)

		assert.Fail(t,
			fmt.Sprintf("Detected SetToDefault pointer reuse at %s", fullPath),
			"Types implementing syntax.Defaulter must not reuse pointers across multiple calls. Doing so leads to default values being changed when unmarshaling configuration files. If you're seeing this error, check the path above and ensure that copies are being made of any pointers in all instances of SetToDefault calls where that field is used.",
		)
	}
}
