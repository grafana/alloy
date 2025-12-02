package save

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
)

func TestLogsReceiver(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	
	// Create component with temporary output location
	args := Arguments{
		OutputLocation: filepath.Join(tempDir, "telemetry/save/"),
	}
	
	opts := component.Options{
		Logger: log.NewNopLogger(),
		OnStateChange: func(exports component.Exports) {
			// No-op for testing
		},
	}
	
	// Create the component
	c, err := NewComponent(opts, args)
	require.NoError(t, err)
	
	// Start the component
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	go func() {
		_ = c.Run(ctx)
	}()
	
	// Give component a moment to start
	time.Sleep(100 * time.Millisecond)
	
	// Create a test log entry
	testEntry := loki.Entry{
		Labels: model.LabelSet{
			"service": "test-service",
			"level":   "info",
		},
		Entry: push.Entry{
			Timestamp: time.Now(),
			Line:      "This is a test log message",
			StructuredMetadata: push.LabelsAdapter{
				{Name: "trace_id", Value: "abc123"},
				{Name: "span_id", Value: "def456"},
			},
		},
	}
	
	// Send the log entry
	c.logsReceiver.Chan() <- testEntry
	
	// Give some time for the entry to be added to batch
	time.Sleep(100 * time.Millisecond)
	
	// Force flush the batch immediately for testing
	c.logsHandler.flushLogBatch()
	
	// Verify the log file was created and contains our entry
	logFilePath := filepath.Join(c.lokiLogsFolder, "logs.json")
	require.FileExists(t, logFilePath)
	
	// Read and verify the content
	content, err := os.ReadFile(logFilePath)
	require.NoError(t, err)
	
	var logEntry LogEntry
	err = json.Unmarshal(content[:len(content)-1], &logEntry) // Remove trailing newline
	require.NoError(t, err)
	
	// Verify the log entry fields
	require.Equal(t, "This is a test log message", logEntry.Line)
	require.Equal(t, "test-service", logEntry.Labels["service"])
	require.Equal(t, "info", logEntry.Labels["level"])
	require.Len(t, logEntry.StructuredMetadata, 2)
	require.Equal(t, "trace_id", logEntry.StructuredMetadata[0].Name)
	require.Equal(t, "abc123", logEntry.StructuredMetadata[0].Value)
	require.Equal(t, "span_id", logEntry.StructuredMetadata[1].Name)
	require.Equal(t, "def456", logEntry.StructuredMetadata[1].Value)
}

func TestComponentExports(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	
	// Create component with temporary output location
	args := Arguments{
		OutputLocation: filepath.Join(tempDir, "telemetry/save/"),
	}
	
	var exports Exports
	opts := component.Options{
		Logger: log.NewNopLogger(),
		OnStateChange: func(e component.Exports) {
			exports = e.(Exports)
		},
	}
	
	// Create the component
	c, err := NewComponent(opts, args)
	require.NoError(t, err)
	
	// Verify exports are set correctly
	require.NotNil(t, exports.Receiver)
	require.NotNil(t, exports.LogsReceiver)
	
	// Verify the metrics receiver is the component itself
	require.Equal(t, c, exports.Receiver)
	
	// Verify the logs receiver has a channel
	require.NotNil(t, exports.LogsReceiver.Chan())
}

func TestLogsBatching(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	
	// Create component with temporary output location
	args := Arguments{
		OutputLocation: filepath.Join(tempDir, "telemetry/save/"),
	}
	
	opts := component.Options{
		Logger: log.NewNopLogger(),
		OnStateChange: func(exports component.Exports) {
			// No-op for testing
		},
	}
	
	// Create the component
	c, err := NewComponent(opts, args)
	require.NoError(t, err)
	
	// Start the component
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	go func() {
		_ = c.Run(ctx)
	}()
	
	// Give component a moment to start
	time.Sleep(100 * time.Millisecond)
	
	// Send multiple log entries
	for i := 0; i < 5; i++ {
		testEntry := loki.Entry{
			Labels: model.LabelSet{
				"service": model.LabelValue(fmt.Sprintf("test-service-%d", i)),
				"level":   "info",
			},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      fmt.Sprintf("Test log message %d", i),
			},
		}
		c.logsReceiver.Chan() <- testEntry
	}
	
	// Give some time for entries to be batched
	time.Sleep(200 * time.Millisecond)
	
	// Force flush the batch
	c.logsHandler.flushLogBatch()
	
	// Verify the log file was created and contains all entries
	logFilePath := filepath.Join(c.lokiLogsFolder, "logs.json")
	require.FileExists(t, logFilePath)
	
	// Read and verify the content
	content, err := os.ReadFile(logFilePath)
	require.NoError(t, err)
	
	// Split by lines and verify we have 5 entries
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	require.Len(t, lines, 5)
	
	// Verify each line is valid JSON
	for i, line := range lines {
		var logEntry LogEntry
		err = json.Unmarshal([]byte(line), &logEntry)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("Test log message %d", i), logEntry.Line)
		require.Equal(t, fmt.Sprintf("test-service-%d", i), logEntry.Labels["service"])
	}
}