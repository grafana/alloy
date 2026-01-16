package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
)

func TestFileRotationRename(t *testing.T) {
	runFileRotationTest(t, func(f *os.File, path string, generation int) (*os.File, error) {
		// Make sure to sync all written data before we rotate.
		if err := f.Sync(); err != nil {
			return nil, err
		}

		if err := f.Close(); err != nil {
			return nil, err
		}

		// Rename file with a "generation" suffix.
		if err := os.Rename(path, fmt.Sprintf("%s.%d", path, generation)); err != nil {
			panic(err)
		}

		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, err
		}

		return file, nil
	})
}

// Default query limit in loki is 5k so we need to make sure that total
// logs produced is not more to make the assertion simple.
const (
	numWriters  = 5
	rotateEvery = 200
	stopAfter   = rotateEvery * 5
)

func runFileRotationTest(t *testing.T, fn rotateFn) {
	// Get the test directory by going up from the current working directory
	// The test runs from the test directory, so we can use relative path
	testDir, err := os.Getwd()
	if err != nil {
		t.Errorf("failed to get test directory: %v\n", err)
	}

	defer cleanup(t, testDir)

	var wg sync.WaitGroup
	var results = [numWriters]common.ExpectedLogResult{}

	for id := range numWriters {
		wg.Go(func() {
			w, err := newWriter(common.SanitizeTestName(t), id, testDir, fn)
			if err != nil {
				t.Errorf("failed to create writer: %v\n", err)
			}
			if err := w.run(); err != nil {
				t.Errorf("failed to run writer: %v\n", err)
			}
			results[id] = w.expected()
		})
	}

	wg.Wait()
	common.AssertLogsPresent(t, results[:]...)
}

func cleanup(t *testing.T, testDir string) {
	entires, err := os.ReadDir(filepath.Join(testDir, "mount"))
	if err != nil {
		t.Errorf("failed to cleanup mount dir: %v\n", err)
	}

	for _, e := range entires {
		if err := os.RemoveAll(filepath.Join(testDir, "mount", e.Name())); err != nil {
			t.Errorf("failed to cleanup mount dir: %v\n", err)
		}
	}
}

func newWriter(testName string, id int, dir string, fn rotateFn) (*writer, error) {
	// Construct the file path in the mount directory
	fileName := fmt.Sprintf("%d.log", id)
	filePath := filepath.Join(dir, "mount", fileName)
	mountedPath := filepath.Join("/etc/alloy/mount", fileName)

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &writer{
		id:          id,
		testName:    testName,
		generation:  1,
		file:        file,
		filePath:    filePath,
		mountedPath: mountedPath,
		rotateFn:    fn,
	}, nil
}

type rotateFn func(f *os.File, path string, generation int) (*os.File, error)

type writer struct {
	id         int
	generation int
	testName   string

	file        *os.File
	filePath    string
	mountedPath string

	written int

	rotateFn rotateFn
}

func (w *writer) run() error {
	// We need an initial sleep so that alloy have time to discover the first set of files.
	time.Sleep(5 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)

	defer func() {
		ticker.Stop()
		w.file.Close()

		if err := os.Remove(w.filePath); err != nil && !os.IsNotExist(err) {
			fmt.Printf("warning: failed to remove file %s: %v\n", w.filePath, err)
		}
	}()

	lineNum := 0
	for {
		<-ticker.C
		lineNum += 1

		w.log(lineNum)

		if lineNum == rotateEvery {
			lineNum = 0
			if err := w.rotate(); err != nil {
				return err
			}
		}

		if w.written == stopAfter {
			return nil
		}
	}
}

func (w *writer) log(lineNum int) error {
	_, err := fmt.Fprintf(w.file, "id=%d generation=%d num=%d test_name=\"%s\"\n", w.id, w.generation, lineNum, w.testName)
	if err != nil {
		return err
	}

	w.written += 1
	return nil
}

func (w *writer) rotate() error {
	f, err := w.rotateFn(w.file, w.filePath, w.generation)
	if err != nil {
		return err
	}

	w.generation += 1
	w.file = f
	return nil
}

func (w *writer) expected() common.ExpectedLogResult {
	return common.ExpectedLogResult{
		Labels: map[string]string{
			"filename": w.mountedPath,
		},
		EntryCount: w.written,
	}
}
