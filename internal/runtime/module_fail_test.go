package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/internal/controller"
	"github.com/stretchr/testify/require"
)

func TestIDRemovalIfFailedToLoad(t *testing.T) {
	f := New(testOptions(t))

	fullContent := "test.fail.module \"t1\" { content = \"\" }"
	fl, err := ParseSource("test", []byte(fullContent))
	require.NoError(t, err)
	err = f.LoadSource(fl, nil, "")
	require.NoError(t, err)
	ctx := t.Context()
	ctx, cnc := context.WithTimeout(ctx, 600*time.Second)

	go f.Run(ctx)
	var t1 *componenttest.TestFailModule
	require.Eventually(t, func() bool {
		t1 = f.loader.Components()[0].(*controller.BuiltinComponentNode).Component().(*componenttest.TestFailModule)
		return t1 != nil
	}, 10*time.Second, 100*time.Millisecond)
	require.Eventually(t, func() bool {
		f.loadMut.RLock()
		defer f.loadMut.RUnlock()
		// This should be one due to t1.
		return len(f.modules.List()) == 1
	}, 10*time.Second, 100*time.Millisecond)
	badContent :=
		`test.fail.module "bad" {
content=""
fail=true
}`
	err = t1.UpdateContent(badContent)
	// Because we have bad content this should fail, but the ids should be removed.
	require.Error(t, err)
	require.Eventually(t, func() bool {
		f.loadMut.RLock()
		defer f.loadMut.RUnlock()
		// Only one since the bad one never should have been added.
		rightLength := len(f.modules.List()) == 1
		_, foundT1 := f.modules.Get("test.fail.module.t1")
		return rightLength && foundT1
	}, 10*time.Second, 100*time.Millisecond)
	// fail a second time to ensure the once is done again.
	err = t1.UpdateContent(badContent)
	require.Error(t, err)

	goodContent :=
		`test.fail.module "good" { 
content=""
fail=false
}`
	err = t1.UpdateContent(goodContent)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		f.loadMut.RLock()
		defer f.loadMut.RUnlock()
		modT1, foundT1 := f.modules.Get("test.fail.module.t1")
		modGood, foundGood := f.modules.Get("test.fail.module.t1/test.fail.module.good")
		return modT1 != nil && modGood != nil && foundT1 && foundGood
	}, 10*time.Second, 100*time.Millisecond)
	cnc()
	require.Eventually(t, func() bool {
		f.loadMut.RLock()
		defer f.loadMut.RUnlock()
		// All should be cleaned up.
		return len(f.modules.List()) == 0
	}, 10*time.Second, 100*time.Millisecond)
}
