//go:build linux

package flow_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPullUpdating(t *testing.T) {
	// Previously we used fetch instead of pull, which would set the FETCH_HEAD but not HEAD
	// This caused changes not to propagate if there were changes, since HEAD was pinned to whatever it was on the initial download.
	// Switching to pull removes this problem at the expense of network bandwidth.
	// Tried switching to FETCH_HEAD but FETCH_HEAD is only set on fetch and not initial repo clone so we would need to
	// remember to always call fetch after clone.
	//
	// This test ensures we can pull the correct values down if they update no matter what, it works by creating a local
	// file based git repo then committing a file, running the component, then updating the file in the repo.
	testRepo := t.TempDir()

	contents := `declare "add" {
    argument "a" {}
    argument "b" {}

    export "sum" {
        value = argument.a.value + argument.b.value
    }
}`
	main := `
import.git "testImport" {
	repository = "` + testRepo + `"
  	path = "math.river"
    pull_frequency = "5s"
}

testImport.add "cc" {
	a = 1
    b = 1
}
`
	init := exec.Command("git", "init", testRepo)
	err := init.Run()
	require.NoError(t, err)
	math := filepath.Join(testRepo, "math.river")
	err = os.WriteFile(math, []byte(contents), 0666)
	require.NoError(t, err)
	add := exec.Command("git", "add", ".")
	add.Dir = testRepo
	err = add.Run()
	require.NoError(t, err)
	commit := exec.Command("git", "commit", "-m \"test\"")
	commit.Dir = testRepo
	err = commit.Run()
	require.NoError(t, err)

	defer verifyNoGoroutineLeaks(t)
	ctrl, f := setup(t, main)
	err = ctrl.LoadSource(f, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ctrl.Run(ctx)
	}()

	// Check for initial condition
	require.Eventually(t, func() bool {
		export := getExport[map[string]interface{}](t, ctrl, "", "testImport.add.cc")
		return export["sum"] == 2
	}, 3*time.Second, 10*time.Millisecond)

	contentsMore := `declare "add" {
    argument "a" {}
    argument "b" {}

    export "sum" {
        value = argument.a.value + argument.b.value + 1
    }
}`
	err = os.WriteFile(math, []byte(contentsMore), 0666)
	require.NoError(t, err)
	add2 := exec.Command("git", "add", ".")
	add2.Dir = testRepo
	add2.Run()

	commit2 := exec.Command("git", "commit", "-m \"test2\"")
	commit2.Dir = testRepo
	commit2.Run()

	// Check for final condition.
	require.Eventually(t, func() bool {
		export := getExport[map[string]interface{}](t, ctrl, "", "testImport.add.cc")
		return export["sum"] == 3
	}, 20*time.Second, 1*time.Millisecond)
}
