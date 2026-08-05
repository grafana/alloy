package stages

import (
	"encoding/json"
	"log/slog"
	"maps"
	"reflect"
	"unsafe"
)

// SplitJSONConfig represents a split_json stage configuration.
type SplitJSONConfig struct {
	Source *Source `alloy:"source,attr,optional"`
}

// splitJSONStage splits a top-level JSON array into one entry per element.
type splitJSONStage struct {
	cfg    SplitJSONConfig
	logger *slog.Logger
}

// newSplitJSONStage creates a new split_json pipeline stage from a config.
// An Alloy-decoded config cannot carry an empty source (Source rejects it at
// decode time), so construction cannot fail.
func newSplitJSONStage(logger *slog.Logger, cfg SplitJSONConfig) Stage {
	return &splitJSONStage{
		cfg:    cfg,
		logger: logger.With("stage", "split_json"),
	}
}

// Run implements Stage. Non-final children are cloned and sent one at a time
// rather than materialized as a batch, so channel backpressure applies per
// child; the final child reuses the original entry's allocations, which the
// stage never touches again.
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
			for i, raw := range elems {
				child := e // struct copy keeps scalar fields, incl. the private created field
				if i < len(elems)-1 {
					child.Entry = e.Entry.Clone()
					child.Extracted = maps.Clone(e.Extracted)
				}
				// #nosec G103 nosemgrep: use-of-unsafe-block
				// Safety: json.RawMessage's UnmarshalJSON contract copies each
				// value into its own backing slice, which is never exposed or
				// mutated after this point, so the string's bytes stay immutable.
				child.Line = unsafe.String(unsafe.SliceData(raw), len(raw))
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
		source := string(*s.cfg.Source)
		value, ok := e.Extracted[source]
		if !ok {
			if debugEnabled(s.logger) {
				s.logger.Debug("source does not exist in the set of extracted values", "source", source)
			}
			return nil, false
		}

		str, err := getString(value)
		if err != nil {
			if debugEnabled(s.logger) {
				s.logger.Debug("failed to convert source value to string", "source", source, "err", err, "type", reflect.TypeOf(value))
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
