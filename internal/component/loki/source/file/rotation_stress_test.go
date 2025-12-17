package file

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
)

// rotationType defines different file rotation strategies
type rotationType int

const (
	// rotationTypeRename renames the old file and creates a new one (most common, e.g., logrotate default)
	rotationTypeRename rotationType = iota
	// rotationTypeCopyTruncate copies the file then truncates it (used when app keeps file handle open)
	rotationTypeCopyTruncate
	// rotationTypeDelete deletes the file and creates a new one (less common but supported)
	rotationTypeDelete
)

func (r rotationType) String() string {
	switch r {
	case rotationTypeRename:
		return "rename"
	case rotationTypeCopyTruncate:
		return "copytruncate"
	case rotationTypeDelete:
		return "delete"
	default:
		return "unknown"
	}
}

// testConfig defines the parameters for a rotation stress test
type testConfig struct {
	// Number of concurrent log files to write
	numFiles int
	// How often to rotate each file
	rotationInterval time.Duration
	// Number of lines to write before each rotation
	linesPerRotation int
	// Total duration of the test
	duration time.Duration
	// Delay between writing lines (controls write rate)
	writeDelay time.Duration
	// Rotation strategy to use
	rotationType rotationType
}

// logLine represents a written log line with metadata
type logLine struct {
	fileID   int
	sequence int
	text     string
}

// logWriter manages writing logs to a single file with rotation
type logWriter struct {
	config       testConfig
	fileID       int
	testDir      string
	currentFile  *os.File
	sequence     int
	mu           sync.Mutex
	writtenLines []logLine
	ctx          context.Context
}

// newLogWriter creates a new log writer for a specific file ID
func newLogWriter(ctx context.Context, cfg testConfig, fileID int, testDir string) *logWriter {
	return &logWriter{
		config:       cfg,
		fileID:       fileID,
		testDir:      testDir,
		writtenLines: make([]logLine, 0, 1000),
		ctx:          ctx,
	}
}

// formatLogLine creates a log line with file ID and sequence number
func formatLogLine(fileID, sequence int) string {
	return fmt.Sprintf("file%d-line%08d", fileID, sequence)
}

// parseLogLine extracts file ID and sequence from a log line
func parseLogLine(line string) (fileID, sequence int, err error) {
	re := regexp.MustCompile(`^file(\d+)-line(\d+)$`)
	matches := re.FindStringSubmatch(line)
	if len(matches) != 3 {
		return 0, 0, fmt.Errorf("invalid log line format: %s", line)
	}
	fileID, err = strconv.Atoi(matches[1])
	if err != nil {
		return 0, 0, err
	}
	sequence, err = strconv.Atoi(matches[2])
	if err != nil {
		return 0, 0, err
	}
	return fileID, sequence, nil
}

// currentLogPath returns the path to the current active log file
func (w *logWriter) currentLogPath() string {
	return filepath.Join(w.testDir, fmt.Sprintf("test%d.log", w.fileID))
}

// rotatedLogPath returns the path for a rotated log file
func (w *logWriter) rotatedLogPath(timestamp int64) string {
	return filepath.Join(w.testDir, fmt.Sprintf("test%d.log.%d", w.fileID, timestamp))
}

// openFile opens the current log file for writing
func (w *logWriter) openFile() error {
	var err error
	w.currentFile, err = os.OpenFile(w.currentLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	return err
}

// rotate performs file rotation using the configured rotation strategy
func (w *logWriter) rotate() error {
	currentPath := w.currentLogPath()
	rotatedPath := w.rotatedLogPath(time.Now().UnixNano())

	switch w.config.rotationType {
	case rotationTypeRename:
		return w.rotateRename(currentPath, rotatedPath)
	case rotationTypeCopyTruncate:
		return w.rotateCopyTruncate(currentPath, rotatedPath)
	case rotationTypeDelete:
		return w.rotateDelete(currentPath)
	default:
		return fmt.Errorf("unknown rotation type: %d", w.config.rotationType)
	}
}

// rotateRename renames the old file and creates a new one (traditional rotation)
func (w *logWriter) rotateRename(currentPath, rotatedPath string) error {
	if w.currentFile != nil {
		// Sync to ensure all data is written before rotation
		if err := w.currentFile.Sync(); err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}
		if err := w.currentFile.Close(); err != nil {
			return fmt.Errorf("close failed: %w", err)
		}
	}

	// Rename current log file with timestamp
	if _, err := os.Stat(currentPath); err == nil {
		if err := os.Rename(currentPath, rotatedPath); err != nil {
			return fmt.Errorf("rename failed: %w", err)
		}
	}

	// Open new file
	return w.openFile()
}

// rotateCopyTruncate copies content to rotated file then truncates original (copytruncate strategy)
func (w *logWriter) rotateCopyTruncate(currentPath, rotatedPath string) error {
	if w.currentFile != nil {
		// Sync to ensure all data is written before rotation
		if err := w.currentFile.Sync(); err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}
	}

	// Copy current file to rotated file
	if _, err := os.Stat(currentPath); err == nil {
		if err := copyFile(currentPath, rotatedPath); err != nil {
			return fmt.Errorf("copy failed: %w", err)
		}

		// Truncate the original file (keeping the same inode)
		if err := w.currentFile.Truncate(0); err != nil {
			return fmt.Errorf("truncate failed: %w", err)
		}
		// Seek back to the beginning
		if _, err := w.currentFile.Seek(0, 0); err != nil {
			return fmt.Errorf("seek failed: %w", err)
		}
	}

	return nil
}

// rotateDelete deletes the old file and creates a new one
func (w *logWriter) rotateDelete(currentPath string) error {
	if w.currentFile != nil {
		// Sync and close
		if err := w.currentFile.Sync(); err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}
		if err := w.currentFile.Close(); err != nil {
			return fmt.Errorf("close failed: %w", err)
		}
		w.currentFile = nil
	}

	// Delete the file
	if _, err := os.Stat(currentPath); err == nil {
		if err := os.Remove(currentPath); err != nil {
			return fmt.Errorf("remove failed: %w", err)
		}
	}

	// Small delay to simulate real-world scenario
	time.Sleep(10 * time.Millisecond)

	// Open new file
	return w.openFile()
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}

// writeLine writes a single log line to the current file
func (w *logWriter) writeLine() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	line := formatLogLine(w.fileID, w.sequence)

	// Track written line
	w.writtenLines = append(w.writtenLines, logLine{
		fileID:   w.fileID,
		sequence: w.sequence,
		text:     line,
	})

	w.sequence++

	// Write to file
	_, err := fmt.Fprintf(w.currentFile, "%s\n", line)
	return err
}

// run is the main goroutine that writes logs and rotates files
func (w *logWriter) run() error {
	// Open initial file
	if err := w.openFile(); err != nil {
		return fmt.Errorf("failed to open initial file: %w", err)
	}

	rotationTicker := time.NewTicker(w.config.rotationInterval)
	defer rotationTicker.Stop()

	writeTicker := time.NewTicker(w.config.writeDelay)
	defer writeTicker.Stop()

	endTime := time.Now().Add(w.config.duration)
	linesInCurrentFile := 0

	for {
		select {
		case <-w.ctx.Done():
			// Final sync before stopping
			if w.currentFile != nil {
				w.currentFile.Sync()
				w.currentFile.Close()
			}
			return nil

		case <-rotationTicker.C:
			// Time-based rotation
			if err := w.rotate(); err != nil {
				return fmt.Errorf("rotation failed: %w", err)
			}
			linesInCurrentFile = 0

		case <-writeTicker.C:
			if time.Now().After(endTime) {
				// Test duration completed
				if w.currentFile != nil {
					w.currentFile.Sync()
					w.currentFile.Close()
				}
				return nil
			}

			// Write a line
			if err := w.writeLine(); err != nil {
				return fmt.Errorf("write failed: %w", err)
			}
			linesInCurrentFile++

			// Check if we should rotate based on line count
			if linesInCurrentFile >= w.config.linesPerRotation {
				if err := w.rotate(); err != nil {
					return fmt.Errorf("rotation failed: %w", err)
				}
				linesInCurrentFile = 0
			}
		}
	}
}

// getWrittenLines returns a copy of all written lines
func (w *logWriter) getWrittenLines() []logLine {
	w.mu.Lock()
	defer w.mu.Unlock()
	lines := make([]logLine, len(w.writtenLines))
	copy(lines, w.writtenLines)
	return lines
}

// validationResult contains the results of log validation
type validationResult struct {
	totalWritten   int
	totalReceived  int
	missingLines   []logLine
	duplicateLines []string
	gapsByFile     map[int][]int // file ID -> list of missing sequences
	passed         bool
}

// validateLogs checks that all written logs were received correctly
func validateLogs(t *testing.T, writers []*logWriter, handler *loki.CollectingHandler) validationResult {
	t.Helper()

	// Collect all written lines
	allWritten := make(map[string]logLine)
	totalWritten := 0
	for _, w := range writers {
		lines := w.getWrittenLines()
		for _, line := range lines {
			allWritten[line.text] = line
			totalWritten++
		}
	}

	// Get received lines
	received := handler.Received()
	totalReceived := len(received)

	// Track seen lines for duplicate detection
	seenLines := make(map[string]int)

	// Track sequences per file for gap detection
	sequencesByFile := make(map[int][]int)

	for _, entry := range received {
		line := entry.Line
		seenLines[line]++

		// Parse and track sequences
		fileID, seq, err := parseLogLine(line)
		if err == nil {
			sequencesByFile[fileID] = append(sequencesByFile[fileID], seq)
		}
	}

	result := validationResult{
		totalWritten:  totalWritten,
		totalReceived: totalReceived,
		gapsByFile:    make(map[int][]int),
		passed:        true,
	}

	// Check for missing lines
	for text, line := range allWritten {
		if count, exists := seenLines[text]; !exists {
			result.missingLines = append(result.missingLines, line)
			result.passed = false
		} else if count > 1 {
			// This line was received multiple times (duplicate)
			result.duplicateLines = append(result.duplicateLines, text)
			result.passed = false
		}
	}

	// Check for gaps in sequences per file
	for fileID, sequences := range sequencesByFile {
		sort.Ints(sequences)

		if len(sequences) == 0 {
			continue
		}

		// Check for gaps in the sequence
		for i := 1; i < len(sequences); i++ {
			expected := sequences[i-1] + 1
			actual := sequences[i]

			// Report any missing sequences
			for seq := expected; seq < actual; seq++ {
				result.gapsByFile[fileID] = append(result.gapsByFile[fileID], seq)
				result.passed = false
			}
		}
	}

	// Check for duplicate lines we haven't already found
	for line, count := range seenLines {
		if count > 1 {
			// Check if we already reported this
			alreadyReported := false
			for _, dup := range result.duplicateLines {
				if dup == line {
					alreadyReported = true
					break
				}
			}
			if !alreadyReported {
				result.duplicateLines = append(result.duplicateLines, line)
				result.passed = false
			}
		}
	}

	return result
}

// reportValidationResult logs validation results
func reportValidationResult(t *testing.T, result validationResult) {
	t.Helper()

	t.Logf("Validation Results:")
	t.Logf("  Total Written: %d", result.totalWritten)
	t.Logf("  Total Received: %d", result.totalReceived)
	t.Logf("  Missing: %d", len(result.missingLines))
	t.Logf("  Duplicates: %d", len(result.duplicateLines))

	if len(result.missingLines) > 0 {
		t.Logf("  Missing lines (showing first 10):")
		for i, line := range result.missingLines {
			if i >= 10 {
				t.Logf("    ... and %d more", len(result.missingLines)-10)
				break
			}
			t.Logf("    file%d-line%08d", line.fileID, line.sequence)
		}
	}

	if len(result.duplicateLines) > 0 {
		t.Logf("  Duplicate lines (showing first 10):")
		for i, line := range result.duplicateLines {
			if i >= 10 {
				t.Logf("    ... and %d more", len(result.duplicateLines)-10)
				break
			}
			t.Logf("    %s", line)
		}
	}

	if len(result.gapsByFile) > 0 {
		t.Logf("  Sequence gaps detected:")
		for fileID, gaps := range result.gapsByFile {
			t.Logf("    File %d: %d gaps", fileID, len(gaps))
			if len(gaps) <= 10 {
				t.Logf("      Missing sequences: %v", gaps)
			} else {
				t.Logf("      Missing sequences (first 10): %v", gaps[:10])
			}
		}
	}
}

// runStressTest executes a single stress test configuration
func runStressTest(t *testing.T, cfg testConfig, minSuccessRate float64) {
	t.Helper()

	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	// Create test directory
	testDir := filepath.Join(os.TempDir(), fmt.Sprintf("alloy-stress-test-%d", time.Now().UnixNano()))
	require.NoError(t, os.MkdirAll(testDir, 0755))
	defer os.RemoveAll(testDir)

	t.Logf("Test directory: %s", testDir)
	t.Logf("Config: %d files, rotation every %v, %d lines/rotation, duration %v, rotation type: %s",
		cfg.numFiles, cfg.rotationInterval, cfg.linesPerRotation, cfg.duration, cfg.rotationType)
	t.Logf("Required success rate: %.1f%%", minSuccessRate*100)

	// Create context for writers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create log writers
	writers := make([]*logWriter, cfg.numFiles)
	for i := 0; i < cfg.numFiles; i++ {
		writers[i] = newLogWriter(ctx, cfg, i, testDir)
	}

	// Start writers
	var writerWg sync.WaitGroup
	writerErrors := make(chan error, cfg.numFiles)

	for _, w := range writers {
		writerWg.Add(1)
		go func(writer *logWriter) {
			defer writerWg.Done()
			if err := writer.run(); err != nil {
				writerErrors <- fmt.Errorf("writer %d error: %w", writer.fileID, err)
			}
		}(w)
	}

	// Give writers a moment to create initial files
	time.Sleep(100 * time.Millisecond)

	// Set up loki.source.file component
	handler := loki.NewCollectingHandler()
	defer handler.Stop()

	ctrl, err := componenttest.NewControllerFromID(util.TestLogger(t), "loki.source.file")
	require.NoError(t, err)

	componentCtx, componentCancel := context.WithCancel(context.Background())
	defer componentCancel()

	// Configure component with glob pattern
	args := Arguments{
		Targets: []discovery.Target{
			discovery.NewTargetFromMap(map[string]string{
				"__path__": filepath.Join(testDir, "*.log"),
			}),
		},
		ForwardTo: []loki.LogsReceiver{handler.Receiver()},
		FileMatch: FileMatch{
			Enabled:    true,
			SyncPeriod: 250 * time.Millisecond,
		},
		FileWatch: FileWatch{
			MinPollFrequency: 25 * time.Millisecond,
			MaxPollFrequency: 25 * time.Millisecond,
		},
	}

	// Start component
	go func() {
		err := ctrl.Run(componentCtx, args)
		if err != nil {
			t.Logf("Component error: %v", err)
		}
	}()

	// Wait for component to be running
	require.NoError(t, ctrl.WaitRunning(30*time.Second))
	t.Log("Component is running")

	// Wait for writers to complete
	writerWg.Wait()
	close(writerErrors)

	// Check for writer errors
	for err := range writerErrors {
		require.NoError(t, err, "Writer encountered error")
	}

	t.Log("All writers completed")

	// Give component extra time to process remaining logs
	// This is important for rotation scenarios where logs might be in transit
	gracePeriod := cfg.rotationInterval * 2
	if gracePeriod < 2*time.Second {
		gracePeriod = 2 * time.Second
	}
	if gracePeriod > 10*time.Second {
		gracePeriod = 10 * time.Second
	}

	t.Logf("Waiting %v grace period for log processing...", gracePeriod)
	time.Sleep(gracePeriod)

	// Stop component
	componentCancel()

	// Wait a bit more for final log delivery
	time.Sleep(500 * time.Millisecond)

	// Validate results
	result := validateLogs(t, writers, handler)
	reportValidationResult(t, result)

	// Calculate success rate
	successRate := float64(result.totalReceived) / float64(result.totalWritten)
	t.Logf("Success rate: %.2f%% (%d/%d lines); Required success rate: %.1f%%", successRate*100, result.totalReceived, result.totalWritten, minSuccessRate*100)

	// Assert we meet the minimum success rate
	assert.GreaterOrEqual(t, successRate, minSuccessRate,
		"Success rate %.2f%% is below required %.1f%% (missing %d/%d lines)",
		successRate*100, minSuccessRate*100, len(result.missingLines), result.totalWritten)

	// We should never have duplicates
	assert.Empty(t, result.duplicateLines, "Some log lines were duplicated")
}

// Test scenarios with increasing volume and complexity
//
// These tests validate file rotation handling under stress with different rotation strategies.
// We currently expect 95% of log lines to be successfully processed.
// This threshold should be increased as we add more fixes to improve reliability.

// TestFileRotationStress_QuickSmoke is a quick smoke test that runs even in short mode
func TestFileRotationStress_QuickSmoke(t *testing.T) {
	// TODO: Increase this threshold to 100% as we fix remaining issues
	const minSuccessRate = 0.95 // 95%

	testCases := []struct {
		name         string
		rotationType rotationType
	}{
		{
			name:         "rename",
			rotationType: rotationTypeRename,
		},
		{
			name:         "copytruncate",
			rotationType: rotationTypeCopyTruncate,
		},
		{
			name:         "delete",
			rotationType: rotationTypeDelete,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig{
				numFiles:         2,
				rotationInterval: 500 * time.Millisecond,
				linesPerRotation: 100,
				duration:         3 * time.Second,
				writeDelay:       10 * time.Millisecond,
				rotationType:     tc.rotationType,
			}

			runStressTest(t, cfg, minSuccessRate)
		})
	}
}

func TestFileRotationStress_HighVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	// TODO: Increase this threshold to 100% as we fix remaining issues
	const minSuccessRate = 0.95 // 95%

	testCases := []struct {
		name         string
		rotationType rotationType
	}{
		{
			name:         "rename",
			rotationType: rotationTypeRename,
		},
		{
			name:         "copytruncate",
			rotationType: rotationTypeCopyTruncate,
		},
		{
			name:         "delete",
			rotationType: rotationTypeDelete,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig{
				numFiles:         5,
				rotationInterval: 500 * time.Millisecond,
				linesPerRotation: 200,
				duration:         60 * time.Second,
				writeDelay:       2 * time.Millisecond,
				rotationType:     tc.rotationType,
			}

			runStressTest(t, cfg, minSuccessRate)
		})
	}
}
