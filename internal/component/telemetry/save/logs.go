package save

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/component/common/loki"
)

// LogEntry represents a log entry with its metadata for JSON serialization.
type LogEntry struct {
	Timestamp          time.Time         `json:"timestamp"`
	Line               string            `json:"line"`
	Labels             map[string]string `json:"labels"`
	StructuredMetadata []LabelPair       `json:"structured_metadata,omitempty"`
	Parsed             []LabelPair       `json:"parsed,omitempty"`
}

// LabelPair represents a key-value pair for labels.
type LabelPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// LogsHandler manages the batching and writing of log entries.
type LogsHandler struct {
	component   *Component
	logsBatch   []loki.Entry
	batchMut    sync.Mutex
	flushTimer  *time.Timer
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewLogsHandler creates a new logs handler for the component.
func NewLogsHandler(component *Component) *LogsHandler {
	ctx, cancel := context.WithCancel(context.Background())
	
	h := &LogsHandler{
		component:  component,
		logsBatch:  make([]loki.Entry, 0, 100), // Pre-allocate for 100 entries
		flushTimer: time.NewTimer(5 * time.Second),
		ctx:        ctx,
		cancel:     cancel,
	}
	
	h.flushTimer.Stop() // Don't start the timer yet
	return h
}

// Start begins processing log entries in a background goroutine.
func (h *LogsHandler) Start(logsReceiver loki.LogsReceiver) {
	go h.handleLogEntries(logsReceiver)
}

// Stop shuts down the logs handler gracefully.
func (h *LogsHandler) Stop() {
	h.cancel()
	h.flushLogBatch()
	if h.flushTimer != nil {
		h.flushTimer.Stop()
	}
}

// handleLogEntries processes incoming log entries and batches them for efficient writing.
func (h *LogsHandler) handleLogEntries(logsReceiver loki.LogsReceiver) {
	const maxBatchSize = 50 // Max entries per batch
	
	for {
		select {
		case entry := <-logsReceiver.Chan():
			h.addLogToBatch(entry)
			
			h.batchMut.Lock()
			batchSize := len(h.logsBatch)
			h.batchMut.Unlock()
			
			// Flush if batch is full
			if batchSize >= maxBatchSize {
				h.flushLogBatch()
			}
			
		case <-h.flushTimer.C:
			// Periodic flush
			h.flushLogBatch()
			
		case <-h.ctx.Done():
			// Handler is shutting down
			h.flushLogBatch()
			return
		}
	}
}

// addLogToBatch adds a log entry to the current batch.
func (h *LogsHandler) addLogToBatch(entry loki.Entry) {
	h.batchMut.Lock()
	defer h.batchMut.Unlock()
	
	h.logsBatch = append(h.logsBatch, entry)
	
	// Start flush timer if this is the first entry in the batch
	if len(h.logsBatch) == 1 {
		h.flushTimer.Reset(5 * time.Second)
	}
}

// flushLogBatch writes all batched log entries to disk and clears the batch.
func (h *LogsHandler) flushLogBatch() {
	h.batchMut.Lock()
	defer h.batchMut.Unlock()
	
	if len(h.logsBatch) == 0 {
		return
	}
	
	h.component.mut.RLock()
	lokiLogsFolder := h.component.lokiLogsFolder
	h.component.mut.RUnlock()
	
	// Write all entries in the batch
	filePath := filepath.Join(lokiLogsFolder, "logs.json")
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		_ = level.Error(h.component.logger).Log("msg", "failed to open logs file", "err", err)
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			_ = level.Error(h.component.logger).Log("msg", "failed to close logs file", "err", closeErr)
		}
	}()
	
	for _, entry := range h.logsBatch {
		// Convert loki.Entry to our JSON-serializable format
		logEntry := LogEntry{
			Timestamp: entry.Timestamp,
			Line:      entry.Line,
			Labels:    make(map[string]string),
		}

		// Convert model.LabelSet to map[string]string
		for k, v := range entry.Labels {
			logEntry.Labels[string(k)] = string(v)
		}

		// Convert structured metadata
		for _, label := range entry.StructuredMetadata {
			logEntry.StructuredMetadata = append(logEntry.StructuredMetadata, LabelPair{
				Name:  label.Name,
				Value: label.Value,
			})
		}

		// Convert parsed labels
		for _, label := range entry.Parsed {
			logEntry.Parsed = append(logEntry.Parsed, LabelPair{
				Name:  label.Name,
				Value: label.Value,
			})
		}

		jsonData, err := json.Marshal(logEntry)
		if err != nil {
			_ = level.Error(h.component.logger).Log("msg", "failed to marshal log entry to JSON", "err", err)
			continue
		}

		if _, err := file.WriteString(string(jsonData) + "\n"); err != nil {
			_ = level.Error(h.component.logger).Log("msg", "failed to write log entry to file", "err", err)
			break
		}
	}
	
	// Clear the batch
	h.logsBatch = h.logsBatch[:0]
	
	// Stop the flush timer
	h.flushTimer.Stop()
}