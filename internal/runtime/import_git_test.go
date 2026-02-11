//go:build linux

package runtime_test

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/vcs"
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

	main := `
import.git "testImport" {
	repository = "` + testRepo + `"
  	path = "math.alloy"
    pull_frequency = "1s"
}

testImport.add "cc" {
	a = 1
    b = 1
}
`
	// Create our git repository.
	initializeRepo(t, testRepo)
	runGit(t, testRepo, "checkout", "-b", "main")

	// Add the file we want.
	math := filepath.Join(testRepo, "math.alloy")
	err := os.WriteFile(math, []byte(contents), 0666)
	require.NoError(t, err)

	runGit(t, testRepo, "add", ".")

	runGit(t, testRepo, "commit", "-m \"test\"")

	defer verifyNoGoroutineLeaks(t)
	ctrl, f := setup(t, main, nil, featuregate.StabilityPublicPreview)
	err = ctrl.LoadSource(f, nil, "")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())

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
		export := getExport[map[string]any](t, ctrl, "", "testImport.add.cc")
		return export["sum"] == 2
	}, 5*time.Second, 100*time.Millisecond)

	err = os.WriteFile(math, []byte(contentsMore), 0666)
	require.NoError(t, err)

	runGit(t, testRepo, "add", ".")

	runGit(t, testRepo, "commit", "-m \"test2\"")

	// Check for final condition.
	require.Eventually(t, func() bool {
		export := getExport[map[string]any](t, ctrl, "", "testImport.add.cc")
		return export["sum"] == 3
	}, 5*time.Second, 100*time.Millisecond)
}

func TestPullUpdatingFromBranch(t *testing.T) {
	testRepo := t.TempDir()

	main := `
import.git "testImport" {
	repository = "` + testRepo + `"
  	path = "math.alloy"
    pull_frequency = "1s"
    revision = "testor"
}

testImport.add "cc" {
	a = 1
    b = 1
}
`
	initializeRepo(t, testRepo)
	runGit(t, testRepo, "checkout", "-b", "main")

	runGit(t, testRepo, "checkout", "-b", "testor")

	math := filepath.Join(testRepo, "math.alloy")
	err := os.WriteFile(math, []byte(contents), 0666)
	require.NoError(t, err)

	runGit(t, testRepo, "add", ".")

	runGit(t, testRepo, "commit", "-m \"test\"")

	defer verifyNoGoroutineLeaks(t)
	ctrl, f := setup(t, main, nil, featuregate.StabilityPublicPreview)
	err = ctrl.LoadSource(f, nil, "")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())

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
		export := getExport[map[string]any](t, ctrl, "", "testImport.add.cc")
		return export["sum"] == 2
	}, 5*time.Second, 100*time.Millisecond)

	err = os.WriteFile(math, []byte(contentsMore), 0666)
	require.NoError(t, err)

	runGit(t, testRepo, "add", ".")

	runGit(t, testRepo, "commit", "-m \"test2\"")

	// Check for final condition.
	require.Eventually(t, func() bool {
		export := getExport[map[string]any](t, ctrl, "", "testImport.add.cc")
		return export["sum"] == 3
	}, 5*time.Second, 100*time.Millisecond)
}

func TestPullUpdatingFromHash(t *testing.T) {
	testRepo := t.TempDir()

	initializeRepo(t, testRepo)
	runGit(t, testRepo, "checkout", "-b", "main")

	math := filepath.Join(testRepo, "math.alloy")
	err := os.WriteFile(math, []byte(contents), 0666)
	require.NoError(t, err)

	runGit(t, testRepo, "add", ".")

	runGit(t, testRepo, "commit", "-m \"test\"")

	getHead := exec.Command("git", "rev-parse", "HEAD")
	var stdBuffer bytes.Buffer
	getHead.Dir = testRepo
	getHead.Stdout = bufio.NewWriter(&stdBuffer)
	err = getHead.Run()
	require.NoError(t, err)
	hash := stdBuffer.String()
	hash = strings.TrimSpace(hash)

	main := `
import.git "testImport" {
	repository = "` + testRepo + `"
  	path = "math.alloy"
    pull_frequency = "1s"
    revision = "` + hash + `"
}

testImport.add "cc" {
	a = 1
    b = 1
}
`

	// After this update the sum should still be 2 and not 3 since it is pinned to the initial hash.
	err = os.WriteFile(math, []byte(contentsMore), 0666)
	require.NoError(t, err)

	runGit(t, testRepo, "add", ".")

	runGit(t, testRepo, "commit", "-m \"test2\"")

	defer verifyNoGoroutineLeaks(t)
	ctrl, f := setup(t, main, nil, featuregate.StabilityPublicPreview)
	err = ctrl.LoadSource(f, nil, "")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())

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

	// Check for final condition.
	require.Eventually(t, func() bool {
		export := getExport[map[string]any](t, ctrl, "", "testImport.add.cc")
		return export["sum"] == 2
	}, 5*time.Second, 100*time.Millisecond)
}

func TestPullUpdatingFromTag(t *testing.T) {
	testRepo := t.TempDir()

	initializeRepo(t, testRepo)
	runGit(t, testRepo, "checkout", "-b", "main")

	math := filepath.Join(testRepo, "math.alloy")
	err := os.WriteFile(math, []byte(contents), 0666)
	require.NoError(t, err)

	runGit(t, testRepo, "add", ".")

	runGit(t, testRepo, "commit", "-m \"test\"")

	runGit(t, testRepo, "tag", "-a", "tagtest", "-m", "testtag")

	main := `
import.git "testImport" {
	repository = "` + testRepo + `"
  	path = "math.alloy"
    pull_frequency = "1s"
    revision = "tagtest"
}

testImport.add "cc" {
	a = 1
    b = 1
}
`

	// After this update the sum should still be 2 and not 3 since it is pinned to the tag.
	err = os.WriteFile(math, []byte(contentsMore), 0666)
	require.NoError(t, err)

	runGit(t, testRepo, "add", ".")

	runGit(t, testRepo, "commit", "-m \"test2\"")

	defer verifyNoGoroutineLeaks(t)

	ctrl, f := setup(t, main, nil, featuregate.StabilityPublicPreview)
	err = ctrl.LoadSource(f, nil, "")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())

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

	// Check for final condition.
	require.Eventually(t, func() bool {
		export := getExport[map[string]any](t, ctrl, "", "testImport.add.cc")
		return export["sum"] == 2
	}, 5*time.Second, 100*time.Millisecond)
}

func initializeRepo(t *testing.T, testRepo string) {
	runGit(t, testRepo, "init", testRepo)
	runGit(t, testRepo, "config", "user.email", "you@example.com")
	runGit(t, testRepo, "config", "user.name", "Your Name")
}

func runGit(t *testing.T, dir string, args ...string) {
	exe := exec.Command("git", args...)
	var stdErr bytes.Buffer
	exe.Stderr = bufio.NewWriter(&stdErr)
	exe.Dir = dir
	err := exe.Run()
	errTxt := stdErr.String()
	if err != nil {
		t.Error(errTxt)
	}
	require.NoErrorf(t, err, "command git %v failed", args)
}

const contents = `declare "add" {
    argument "a" {}
    argument "b" {}

    export "sum" {
        value = argument.a.value + argument.b.value
    }
}`

const contentsMore = `declare "add" {
    argument "a" {}
    argument "b" {}

    export "sum" {
        value = argument.a.value + argument.b.value + 1
    }
}`

func TestNoPanicWithNonexistentRevision(t *testing.T) {
	testRepo := t.TempDir()

	main := `
import.git "testImport" {
	repository = "` + testRepo + `"
  	path = "math.alloy"
    pull_frequency = "1s"
    revision = "nonexistent"
}

testImport.add "cc" {
	a = 1
    b = 1
}
`
	initializeRepo(t, testRepo)
	runGit(t, testRepo, "checkout", "-b", "main")

	runGit(t, testRepo, "checkout", "-b", "testor")

	math := filepath.Join(testRepo, "math.alloy")
	err := os.WriteFile(math, []byte(contents), 0666)
	require.NoError(t, err)

	runGit(t, testRepo, "add", ".")

	runGit(t, testRepo, "commit", "-m \"test\"")

	defer verifyNoGoroutineLeaks(t)
	ctrl, f := setup(t, main, nil, featuregate.StabilityPublicPreview)
	err = ctrl.LoadSource(f, nil, "")
	expectedErr := vcs.InvalidRevisionError{
		Revision: "nonexistent",
	}
	require.ErrorContains(t, err, expectedErr.Error())

	ctx, cancel := context.WithCancel(t.Context())
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
}
