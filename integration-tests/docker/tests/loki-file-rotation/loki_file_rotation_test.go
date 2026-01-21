//go:build alloyintegrationtests

package main

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

// TestFileRotationRename will perform writes to files, when rotation happens the original
// files are renamed and new ones are created.
func TestFileRotationRename(t *testing.T) {
	runFileRotationTest(t, func(f *os.File, generation int) (*os.File, error) {
		path := f.Name()

		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("failed to close file: %w", err)
		}

		if err := os.Rename(path, fmt.Sprintf("%s.%d", path, generation)); err != nil {
			return nil, fmt.Errorf("failed to move file: %w", err)
		}

		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to create new file: %w", err)
		}

		return file, nil
	})
}

// TestFileRotationDelete will perform writes to files, when rotation happens the original
// files are deleted and new ones are created.
func TestFileRotationDelete(t *testing.T) {
	runFileRotationTest(t, func(f *os.File, _ int) (*os.File, error) {
		path := f.Name()

		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("failed to close file: %w", err)
		}

		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("failed to delete file: %w", err)
		}

		// Intentional delay to simulate the real-world gap between file deletion and recreation.
		time.Sleep(time.Duration(5+rand.Intn(46)) * time.Millisecond)

		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to create new file: %w", err)
		}

		return file, nil
	})
}

// TestFileRotationCopyTruncate will perform writes to files, when rotation happens the content
// of the original files are copied to new files and the original files are truncated.
func TestFileRotationCopyTruncate(t *testing.T) {
	runFileRotationTest(t, func(f *os.File, generation int) (*os.File, error) {
		path := f.Name()

		copyTo, err := os.OpenFile(fmt.Sprintf("%s.%d", path, generation), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to create new file: %w", err)
		}

		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed initial seek: %w", err)
		}

		if _, err := io.Copy(copyTo, f); err != nil {
			return nil, fmt.Errorf("failed to copy into new file: %w", err)
		}

		if err := copyTo.Sync(); err != nil {
			return nil, fmt.Errorf("failed to sync new file: %w", err)
		}

		if err := copyTo.Close(); err != nil {
			return nil, fmt.Errorf("failed to close new file: %w", err)
		}

		// CopyTruncate is an unforgiving strategy. If the last write is too close
		// to the time we truncate the file it is likely that we miss some logs.
		// This sleep is here to ensure that we pass the test because without it we will
		// miss some logs. We ingest around 4950 out of 5000 without this sleep.
		time.Sleep(2 * time.Second)
		if err := f.Truncate(0); err != nil {
			return nil, fmt.Errorf("failed to truncate file: %w", err)
		}

		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to seek: %w", err)
		}

		return f, nil
	})
}

// TestFileRotationNew will perform writes to files, when rotation happens new files
// are created and old files persist.
func TestFileRotationNew(t *testing.T) {
	runFileRotationTest(t, func(f *os.File, generation int) (*os.File, error) {
		path := f.Name()
		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("failed to close file: %w", err)
		}

		dir, filename := filepath.Dir(path), filepath.Base(path)
		// File is named [id].log and we want to append the generation passed directly
		// after [id] so alloy will discover it.
		newName := fmt.Sprintf("%s%d.log", strings.TrimSuffix(filename, ".log"), generation)
		file, err := os.OpenFile(filepath.Join(dir, newName), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to create new file: %w", err)
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
		t.Errorf("failed to get test directory: %v", err)
		return
	}

	defer cleanup(t, testDir)

	var (
		wg      sync.WaitGroup
		errors  = [numWriters]error{}
		results = [numWriters][]common.ExpectedLogResult{}
	)

	for i := range numWriters {
		wg.Go(func() {
			w, err := newWriter(common.SanitizeTestName(t), i, testDir, fn)
			if err != nil {
				errors[i] = err
				return
			}
			if err := w.run(); err != nil {
				errors[i] = err
				return
			}
			results[i] = append(results[i], w.expected()...)
		})
	}
	wg.Wait()

	var hasErrors bool
	for i, err := range errors {
		if err != nil {
			hasErrors = true
			t.Logf("failed to perform writes: %d %v", i, err)
		}
	}

	if hasErrors {
		t.Fail()
		// return here so we still perform cleanup but do not run assertions.
		return
	}

	var expected []common.ExpectedLogResult
	for _, r := range results {
		expected = append(expected, r...)
	}

	common.AssertLogsPresent(t, expected...)
}

func cleanup(t *testing.T, testDir string) {
	entries, err := os.ReadDir(filepath.Join(testDir, "mount"))
	if err != nil {
		t.Errorf("failed to cleanup mount dir: %v", err)
		return
	}

	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(testDir, "mount", e.Name())); err != nil {
			t.Errorf("failed to cleanup mount dir: %v", err)
			return
		}
	}
}

func newWriter(testName string, id int, dir string, fn rotateFn) (*writer, error) {
	filePath := filepath.Join(dir, "mount", fmt.Sprintf("%d.log", id))
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &writer{
		id:         id,
		testName:   testName,
		generation: 1,
		file:       file,
		rotateFn:   fn,
	}, nil
}

type rotateFn func(f *os.File, generation int) (*os.File, error)

type writer struct {
	id         int
	generation int
	testName   string

	file *os.File

	// linesWritten is the number of lines written for this generation. It will reset to 0
	// on every rotation.
	linesWritten int
	// totalLinesWritten is the total number of lines written and keeps increasing
	// even if file is rotated.
	totalLinesWritten int

	rotateFn rotateFn

	results []common.ExpectedLogResult
}

func (w *writer) run() error {
	ticker := time.NewTicker(50 * time.Millisecond)

	defer func() {
		ticker.Stop()
		w.file.Close()
	}()

	first := true

	for {
		<-ticker.C

		if err := w.log(); err != nil {
			return err
		}
		// Because our rotations are done pretty fast we need to ensure that alloy have discovered
		// the first set of files before we initiate rotations. So we log one line and then wait for
		// that to be ingested into loki.
		if first {
			first = false
			if err := common.WaitForInitalLogs(w.testName); err != nil {
				return err
			}
		}

		if w.linesWritten == rotateEvery {
			if err := w.rotate(); err != nil {
				return err
			}
		}

		if w.totalLinesWritten == stopAfter {
			return nil
		}
	}
}

func (w *writer) log() error {
	w.linesWritten += 1
	w.totalLinesWritten += 1

	_, err := fmt.Fprintf(w.file, "id=%d generation=%d num=%d test_name=\"%s\"\n", w.id, w.generation, w.linesWritten, w.testName)
	if err != nil {
		return err
	}
	return nil
}

func (w *writer) rotate() error {
	var found bool
	mountPath := filepath.Join("/etc/alloy/mount", filepath.Base(w.file.Name()))

	for i := range w.results {
		if w.results[i].Labels["filename"] == mountPath {
			w.results[i].EntryCount += w.linesWritten
			found = true
		}
	}

	if !found {
		w.results = append(w.results, common.ExpectedLogResult{
			Labels: map[string]string{
				"filename": mountPath,
			},
			EntryCount: w.linesWritten,
		})
	}

	w.linesWritten = 0

	// Make sure to sync all written data before we rotate.
	if err := w.file.Sync(); err != nil {
		return err
	}

	f, err := w.rotateFn(w.file, w.generation)
	if err != nil {
		return err
	}

	w.file = f
	w.generation += 1

	return nil
}

func (w *writer) expected() []common.ExpectedLogResult {
	return w.results
}
