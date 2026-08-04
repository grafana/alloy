package stages

import (
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"reflect"
	"slices"
)

// Config Errors
const (
	ErrEmptySplitJSONStageSource = "empty split_json stage source"
)

// SplitJSONConfig represents a split_json stage configuration.
type SplitJSONConfig struct {
	Source *string `alloy:"source,attr,optional"`
}

// splitJSONStage splits a top-level JSON array into one entry per element.
type splitJSONStage struct {
	cfg    SplitJSONConfig
	logger *slog.Logger
}

// newSplitJSONStage creates a new split_json pipeline stage from a config.
func newSplitJSONStage(logger *slog.Logger, cfg SplitJSONConfig) (Stage, error) {
	if cfg.Source != nil && *cfg.Source == "" {
		return nil, errors.New(ErrEmptySplitJSONStageSource)
	}

	return &splitJSONStage{
		cfg:    cfg,
		logger: logger.With("stage", "split_json"),
	}, nil
}

// Run implements Stage. Children are cloned and sent one at a time rather
// than materialized as a batch, so channel backpressure applies per child.
func (s *splitJSONStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range in {
			elems, ok := s.split(e)
			if !ok {
				out <- e
				continue
			}
			for _, raw := range elems {
				child := e               // struct copy keeps the timestamp and the private created field
				child.Line = string(raw) // element bytes verbatim, no re-encode
				child.Labels = e.Labels.Clone()
				child.Extracted = maps.Clone(e.Extracted)
				child.StructuredMetadata = slices.Clone(e.StructuredMetadata)
				child.Parsed = slices.Clone(e.Parsed)
				out <- child
			}
		}
	}()
	return out
}

// split returns the raw elements of the top-level JSON array carried by the
// entry, or false when the entry must pass through unchanged instead.
func (s *splitJSONStage) split(e Entry) ([]json.RawMessage, bool) {
	input := e.Line
	if s.cfg.Source != nil {
		value, ok := e.Extracted[*s.cfg.Source]
		if !ok {
			if debugEnabled(s.logger) {
				s.logger.Debug("source does not exist in the set of extracted values", "source", *s.cfg.Source)
			}
			return nil, false
		}

		str, err := getString(value)
		if err != nil {
			if debugEnabled(s.logger) {
				s.logger.Debug("failed to convert source value to string", "source", *s.cfg.Source, "err", err, "type", reflect.TypeOf(value))
			}
			return nil, false
		}
		input = str
	}

	// Skip only the four JSON whitespace bytes (RFC 8259) — not
	// strings.TrimSpace, which would also strip non-JSON Unicode whitespace —
	// and pass through anything that is not a top-level array.
	i := 0
	for i < len(input) && (input[i] == ' ' || input[i] == '\t' || input[i] == '\r' || input[i] == '\n') {
		i++
	}
	if i == len(input) || input[i] != '[' {
		return nil, false
	}

	// stdlib RawMessage keeps every element byte-for-byte, null included;
	// jsoniter would decode a null element to an empty line. A failed
	// unmarshal yields no children at all.
	var elems []json.RawMessage
	if err := json.Unmarshal([]byte(input), &elems); err != nil {
		if debugEnabled(s.logger) {
			s.logger.Debug("failed to unmarshal top-level JSON array", "err", err)
		}
		return nil, false
	}
	return elems, true
}

// Cleanup implements Stage.
func (*splitJSONStage) Cleanup() {
	// no-op
}
