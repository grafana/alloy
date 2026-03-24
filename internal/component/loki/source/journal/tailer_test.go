//go:build linux && cgo && promtail_journal_enabled

package journal

// This code is copied from Promtail (https://github.com/grafana/loki/commit/954df433e98f659d006ced52b23151cb5eb2fdfa) with minor edits. The target package is used to
// configure and run the targets that can read journal entries and forward them
// to other loki components.

import (
	"io"
	"maps"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-systemd/v22/sdjournal"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/loki/promtail/scrapeconfig"
)

var randomGenerator *rand.Rand

func initRandom() {
	randomGenerator = rand.New(rand.NewSource(time.Now().UnixNano()))
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randName() string {
	b := make([]rune, 10)
	for i := range b {
		b[i] = letters[randomGenerator.Intn(len(letters))] //#nosec G404 -- Generating random test data, fine.
	}
	return string(b)
}

type mockJournalReader struct {
	config sdjournal.JournalReaderConfig
	t      *testing.T
}

func newMockJournalReader(c sdjournal.JournalReaderConfig) (journalReader, error) {
	return &mockJournalReader{config: c}, nil
}

func (r *mockJournalReader) Close() error {
	return nil
}

func (r *mockJournalReader) Follow(until <-chan time.Time, writer io.Writer) error {
	<-until
	return nil
}

func newMockJournalEntry(entry *sdjournal.JournalEntry) journalEntryFunc {
	return func(c sdjournal.JournalReaderConfig, cursor string) (*sdjournal.JournalEntry, error) {
		return entry, nil
	}
}

func (r *mockJournalReader) Write(fields map[string]string) {
	allFields := make(map[string]string, len(fields))
	maps.Copy(allFields, fields)

	ts := uint64(time.Now().UnixNano())

	_, err := r.config.Formatter(&sdjournal.JournalEntry{
		Fields:             allFields,
		MonotonicTimestamp: ts,
		RealtimeTimestamp:  ts,
	})
	assert.NoError(r.t, err)
}

func TestTailer(t *testing.T) {
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)

	initRandom()
	dirName := filepath.Join(os.TempDir(), randName())
	positionsFileName := dirName + "/positions.yml"

	// Set the sync period to a really long value, to guarantee the sync timer
	// never runs, this way we know everything saved was done through channel
	// notifications when target.stop() was called.
	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: positionsFileName,
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := loki.NewCollectingHandler()
	defer handler.Stop()

	relabelCfg := `
- source_labels: ['__journal_code_file']
  regex: 'journaltarget_test\.go'
  action: 'keep'
- source_labels: ['__journal_code_file']
  target_label: 'code_file'`

	relabels := parseRelabelRules(t, relabelCfg)
	registry := prometheus.NewRegistry()
	jt, err := newTailerWithReader(newMetrics(registry), logger, handler.Receiver(), ps, "test", relabels,
		&scrapeconfig.JournalTargetConfig{}, newMockJournalReader, newMockJournalEntry(nil))
	require.NoError(t, err)

	r := jt.r.(*mockJournalReader)
	r.t = t

	for range 10 {
		r.Write(map[string]string{
			"MESSAGE":   "ping",
			"CODE_FILE": "journaltarget_test.go",
		})
		assert.NoError(t, err)
	}
	require.NoError(t, jt.Stop())

	expectedMetrics := `# HELP loki_source_journal_target_lines_total Total number of successful journal lines read
	# TYPE loki_source_journal_target_lines_total counter
	loki_source_journal_target_lines_total 10
	`

	if err := testutil.GatherAndCompare(registry,
		strings.NewReader(expectedMetrics)); err != nil {
		t.Fatalf("mismatch metrics: %v", err)
	}
	assert.Len(t, handler.Received(), 10)
}

func TestTailerParsingErrors(t *testing.T) {
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)

	initRandom()
	dirName := filepath.Join(os.TempDir(), randName())
	positionsFileName := dirName + "/positions.yml"

	// Set the sync period to a really long value, to guarantee the sync timer
	// never runs, this way we know everything saved was done through channel
	// notifications when target.stop() was called.
	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: positionsFileName,
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := loki.NewCollectingHandler()
	defer handler.Stop()

	// We specify no relabel rules, so that we end up with an empty labelset
	var relabels []*relabel.Config

	registry := prometheus.NewRegistry()
	jt, err := newTailerWithReader(newMetrics(registry), logger, handler.Receiver(), ps, "test", relabels,
		&scrapeconfig.JournalTargetConfig{}, newMockJournalReader, newMockJournalEntry(nil))
	require.NoError(t, err)

	r := jt.r.(*mockJournalReader)
	r.t = t

	// No labels but correct message
	for range 10 {
		r.Write(map[string]string{
			"MESSAGE":   "ping",
			"CODE_FILE": "journaltarget_test.go",
		})
		assert.NoError(t, err)
	}

	// No labels and no message
	for range 10 {
		r.Write(map[string]string{
			"CODE_FILE": "journaltarget_test.go",
		})
		assert.NoError(t, err)
	}
	require.NoError(t, jt.Stop())

	expectedMetrics := `# HELP loki_source_journal_target_lines_total Total number of successful journal lines read
	# TYPE loki_source_journal_target_lines_total counter
	loki_source_journal_target_lines_total 0
	# HELP loki_source_journal_target_parsing_errors_total Total number of parsing errors while reading journal messages
	# TYPE loki_source_journal_target_parsing_errors_total counter
	loki_source_journal_target_parsing_errors_total{error="empty_labels"} 10
	loki_source_journal_target_parsing_errors_total{error="no_message"} 10
	`

	if err := testutil.GatherAndCompare(registry,
		strings.NewReader(expectedMetrics)); err != nil {
		t.Fatalf("mismatch metrics: %v", err)
	}

	assert.Len(t, handler.Received(), 0)
}

func TestTailer_JSON(t *testing.T) {
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)

	initRandom()
	dirName := filepath.Join(os.TempDir(), randName())
	positionsFileName := dirName + "/positions.yml"

	// Set the sync period to a really long value, to guarantee the sync timer
	// never runs, this way we know everything saved was done through channel
	// notifications when target.stop() was called.
	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: positionsFileName,
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := loki.NewCollectingHandler()
	defer handler.Stop()

	relabelCfg := `
- source_labels: ['__journal_code_file']
  regex: 'journaltarget_test\.go'
  action: 'keep'
- source_labels: ['__journal_code_file']
  target_label: 'code_file'`
	relabels := parseRelabelRules(t, relabelCfg)
	cfg := &scrapeconfig.JournalTargetConfig{JSON: true}

	jt, err := newTailerWithReader(newMetrics(prometheus.NewRegistry()), logger, handler.Receiver(), ps, "test", relabels,
		cfg, newMockJournalReader, newMockJournalEntry(nil))
	require.NoError(t, err)

	r := jt.r.(*mockJournalReader)
	r.t = t

	for range 10 {
		r.Write(map[string]string{
			"MESSAGE":     "ping",
			"CODE_FILE":   "journaltarget_test.go",
			"OTHER_FIELD": "foobar",
		})
		assert.NoError(t, err)

	}
	expectMsg := `{"CODE_FILE":"journaltarget_test.go","MESSAGE":"ping","OTHER_FIELD":"foobar"}`
	require.NoError(t, jt.Stop())

	entries := handler.Received()
	assert.Len(t, entries, 10)
	for i := range 10 {
		require.Equal(t, expectMsg, entries[i].Line)
	}
}

func TestTailer_Since(t *testing.T) {
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)

	initRandom()
	dirName := filepath.Join(os.TempDir(), randName())
	positionsFileName := dirName + "/positions.yml"

	// Set the sync period to a really long value, to guarantee the sync timer
	// never runs, this way we know everything saved was done through channel
	// notifications when target.stop() was called.
	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: positionsFileName,
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := loki.NewCollectingHandler()
	defer handler.Stop()

	cfg := scrapeconfig.JournalTargetConfig{
		MaxAge: "4h",
	}

	jt, err := newTailerWithReader(newMetrics(prometheus.NewRegistry()), logger, handler.Receiver(), ps, "test", nil,
		&cfg, newMockJournalReader, newMockJournalEntry(nil))
	require.NoError(t, err)

	r := jt.r.(*mockJournalReader)
	require.Equal(t, r.config.Since, -1*time.Hour*4)
}

func TestTailer_Cursor_TooOld(t *testing.T) {
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)

	initRandom()
	dirName := filepath.Join(os.TempDir(), randName())
	positionsFileName := dirName + "/positions.yml"

	// Set the sync period to a really long value, to guarantee the sync timer
	// never runs, this way we know everything saved was done through channel
	// notifications when target.stop() was called.
	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: positionsFileName,
	})
	if err != nil {
		t.Fatal(err)
	}
	ps.PutString("journal-test", "", "foobar")

	handler := loki.NewCollectingHandler()
	defer handler.Stop()

	cfg := scrapeconfig.JournalTargetConfig{}

	entryTs := time.Date(1980, time.July, 3, 12, 0, 0, 0, time.UTC)
	journalEntry := newMockJournalEntry(&sdjournal.JournalEntry{
		Cursor:            "foobar",
		Fields:            nil,
		RealtimeTimestamp: uint64(entryTs.UnixNano()),
	})

	jt, err := newTailerWithReader(newMetrics(prometheus.NewRegistry()), logger, handler.Receiver(), ps, "test", nil,
		&cfg, newMockJournalReader, journalEntry)
	require.NoError(t, err)

	r := jt.r.(*mockJournalReader)
	require.Equal(t, r.config.Since, -1*time.Hour*7)
}

func TestTailer_Cursor_NotTooOld(t *testing.T) {
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)

	initRandom()
	dirName := filepath.Join(os.TempDir(), randName())
	positionsFileName := dirName + "/positions.yml"

	// Set the sync period to a really long value, to guarantee the sync timer
	// never runs, this way we know everything saved was done through channel
	// notifications when target.stop() was called.
	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: positionsFileName,
	})
	if err != nil {
		t.Fatal(err)
	}
	ps.PutString(positions.CursorKey("test"), "", "foobar")

	handler := loki.NewCollectingHandler()
	defer handler.Stop()

	cfg := scrapeconfig.JournalTargetConfig{}

	entryTs := time.Now().Add(-time.Hour)
	journalEntry := newMockJournalEntry(&sdjournal.JournalEntry{
		Cursor:            "foobar",
		Fields:            nil,
		RealtimeTimestamp: uint64(entryTs.UnixNano() / int64(time.Microsecond)),
	})

	jt, err := newTailerWithReader(newMetrics(prometheus.NewRegistry()), logger, handler.Receiver(), ps, "test", nil,
		&cfg, newMockJournalReader, journalEntry)
	require.NoError(t, err)

	r := jt.r.(*mockJournalReader)
	require.Equal(t, r.config.Since, time.Duration(0))
	require.Equal(t, r.config.Cursor, "foobar")
}

func Test_MakeJournalFields(t *testing.T) {
	entryFields := map[string]string{
		"CODE_FILE":   "journaltarget_test.go",
		"OTHER_FIELD": "foobar",
		"PRIORITY":    "6",
	}
	receivedFields := makeJournalFields(entryFields)
	expectedFields := map[string]string{
		"__journal_code_file":        "journaltarget_test.go",
		"__journal_other_field":      "foobar",
		"__journal_priority":         "6",
		"__journal_priority_keyword": "info",
	}
	assert.Equal(t, expectedFields, receivedFields)
}

func TestTailer_Matches(t *testing.T) {
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)

	initRandom()
	dirName := filepath.Join(os.TempDir(), randName())
	positionsFileName := dirName + "/positions.yml"

	// Set the sync period to a really long value, to guarantee the sync timer
	// never runs, this way we know everything saved was done through channel
	// notifications when target.stop() was called.
	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: positionsFileName,
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := loki.NewCollectingHandler()

	cfg := scrapeconfig.JournalTargetConfig{
		Matches: "UNIT=foo.service PRIORITY=1",
	}

	jt, err := newTailerWithReader(newMetrics(prometheus.NewRegistry()), logger, handler.Receiver(), ps, "test", nil,
		&cfg, newMockJournalReader, newMockJournalEntry(nil))
	require.NoError(t, err)

	r := jt.r.(*mockJournalReader)
	matches := []sdjournal.Match{{Field: "UNIT", Value: "foo.service"}, {Field: "PRIORITY", Value: "1"}}
	require.Equal(t, r.config.Matches, matches)
	handler.Stop()
}

func parseRelabelRules(t *testing.T, s string) []*relabel.Config {
	var relabels []*relabel.Config
	err := yaml.Unmarshal([]byte(s), &relabels)
	require.NoError(t, err)
	for i := range relabels {
		relabels[i].NameValidationScheme = model.LegacyValidation
	}
	return relabels
}
