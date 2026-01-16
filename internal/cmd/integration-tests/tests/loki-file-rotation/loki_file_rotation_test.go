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

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
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
			return nil, fmt.Errorf("failed to created new file: %w", err)
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
			return nil, fmt.Errorf("failed initial seek: %w\n", err)
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
		t.Errorf("failed to get test directory: %v\n", err)
	}

	defer cleanup(t, testDir)

	var wg sync.WaitGroup
	var results = [numWriters][]common.ExpectedLogResult{}

	for id := range numWriters {
		wg.Go(func() {
			w, err := newWriter(common.SanitizeTestName(t), id, testDir, fn)
			if err != nil {
				t.Errorf("failed to create writer: %v\n", err)
			}
			if err := w.run(); err != nil {
				t.Errorf("failed to run writer: %v\n", err)
			}
			results[id] = append(results[id], w.expected()...)
		})
	}

	wg.Wait()

	var expected []common.ExpectedLogResult
	for _, r := range results {
		expected = append(expected, r...)
	}

	common.AssertLogsPresent(t, expected...)
}

func cleanup(t *testing.T, testDir string) {
	entries, err := os.ReadDir(filepath.Join(testDir, "mount"))
	if err != nil {
		t.Errorf("failed to cleanup mount dir: %v\n", err)
	}

	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(testDir, "mount", e.Name())); err != nil {
			t.Errorf("failed to cleanup mount dir: %v\n", err)
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

	fileWritten  int
	totalWritten int

	rotateFn rotateFn

	results []common.ExpectedLogResult
}

func (w *writer) run() error {
	// We need an initial sleep so that alloy has time to discover the first set of files.
	time.Sleep(5 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)

	defer func() {
		ticker.Stop()
		w.file.Close()
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

		if w.totalWritten == stopAfter {
			return nil
		}
	}
}

func (w *writer) log(lineNum int) error {
	_, err := fmt.Fprintf(w.file, "id=%d generation=%d num=%d test_name=\"%s\"\n", w.id, w.generation, lineNum, w.testName)
	if err != nil {
		return err
	}
	w.fileWritten += 1
	w.totalWritten += 1
	return nil
}

func (w *writer) rotate() error {
	var found bool
	mountPath := filepath.Join("/etc/alloy/mount", filepath.Base(w.file.Name()))

	for i := range w.results {
		if w.results[i].Labels["filename"] == mountPath {
			w.results[i].EntryCount += w.fileWritten
			found = true
		}
	}

	if !found {
		w.results = append(w.results, common.ExpectedLogResult{
			Labels: map[string]string{
				"filename": mountPath,
			},
			EntryCount: w.fileWritten,
		})
	}

	w.fileWritten = 0

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
