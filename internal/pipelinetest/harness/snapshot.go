package harness

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"time"
)

// snapshotLimit caps how many items of each signal a Snapshot keeps. A real
// pipeline can capture thousands of samples, and a failure report only needs
// enough of them to show what arrived.
const snapshotLimit = 100

// Snapshot is a copy of what the sink captured, in a form fit for reports and
// JSON output. Each signal keeps at most snapshotLimit items, and the totals
// count everything captured.
type Snapshot struct {
	Logs    []LogEntry `json:"logs"`
	Metrics []Sample   `json:"metrics"`
	Spans   []SpanInfo `json:"spans"`
	// Targets maps a component export, such as discovery.relabel.pods.output,
	// to the targets it held. Only exports a targets assertion reads are
	// included.
	Targets map[string][]map[string]string `json:"targets,omitempty"`

	Totals SnapshotTotals `json:"totals"`
}

// SnapshotTotals counts everything the sink captured.
type SnapshotTotals struct {
	Logs    int `json:"logs"`
	Metrics int `json:"metrics"`
	Spans   int `json:"spans"`
	// OTLP metric data points and log records reach the sink, but cannot be
	// asserted on yet.
	OTLPDataPoints int `json:"otlp_data_points,omitempty"`
	OTLPLogRecords int `json:"otlp_log_records,omitempty"`
}

// LogEntry is one captured Loki entry.
type LogEntry struct {
	// Sink is the exit component whose sink captured the entry. It is empty
	// for the default sink.
	Sink               string            `json:"sink,omitempty"`
	Labels             map[string]string `json:"labels"`
	Line               string            `json:"line"`
	Timestamp          time.Time         `json:"timestamp"`
	StructuredMetadata map[string]string `json:"structured_metadata,omitempty"`
}

// Sample is one captured Prometheus sample.
type Sample struct {
	Sink   string            `json:"sink,omitempty"`
	Labels map[string]string `json:"labels"`
	Value  *float64          `json:"value,omitempty"`
	// ValueText holds values JSON cannot encode: NaN, such as a staleness
	// marker, and the infinities.
	ValueText string    `json:"value_text,omitempty"`
	Histogram bool      `json:"histogram,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type,omitempty"`
	Help      string    `json:"help,omitempty"`
	Unit      string    `json:"unit,omitempty"`
}

// SpanInfo is one captured span.
type SpanInfo struct {
	Sink               string            `json:"sink,omitempty"`
	Name               string            `json:"name"`
	Kind               string            `json:"kind"`
	Status             string            `json:"status"`
	Scope              string            `json:"scope,omitempty"`
	TraceID            string            `json:"trace_id,omitempty"`
	SpanID             string            `json:"span_id,omitempty"`
	ParentSpanID       string            `json:"parent_span_id,omitempty"`
	Attributes         map[string]string `json:"attributes,omitempty"`
	ResourceAttributes map[string]string `json:"resource_attributes,omitempty"`
}

// Text renders the snapshot for a terminal, one line per item, with each
// signal capped at limit items.
func (s Snapshot) Text(limit int) string {
	var sections []string
	if s.Totals.Logs > 0 {
		sections = append(sections, renderCapped("logs", s.Totals.Logs, len(s.Logs), limit, func(i int) string {
			e := s.Logs[i]
			return fmt.Sprintf("%s%s %q", sinkPrefix(e.Sink), renderStringMap(e.Labels), e.Line)
		}))
	}
	if s.Totals.Metrics > 0 {
		sections = append(sections, renderCapped("metrics", s.Totals.Metrics, len(s.Metrics), limit, func(i int) string {
			m := s.Metrics[i]
			series := sinkPrefix(m.Sink) + renderStringMap(m.Labels)
			switch {
			case m.Histogram:
				return series + " histogram"
			case m.Value != nil:
				return series + " " + fmt.Sprint(*m.Value)
			case m.ValueText != "":
				return series + " " + m.ValueText
			default:
				return series
			}
		}))
	}
	if s.Totals.Spans > 0 {
		sections = append(sections, renderCapped("spans", s.Totals.Spans, len(s.Spans), limit, func(i int) string {
			sp := s.Spans[i]
			line := fmt.Sprintf("%s%q kind=%s status=%s", sinkPrefix(sp.Sink), sp.Name, sp.Kind, sp.Status)
			if len(sp.Attributes) > 0 {
				line += " attributes=" + renderStringMap(sp.Attributes)
			}
			return line
		}))
	}
	if s.Totals.OTLPDataPoints > 0 || s.Totals.OTLPLogRecords > 0 {
		sections = append(sections, fmt.Sprintf("otlp metric data points: %d, otlp log records: %d (not assertable yet)",
			s.Totals.OTLPDataPoints, s.Totals.OTLPLogRecords))
	}
	for _, ref := range slices.Sorted(maps.Keys(s.Targets)) {
		targets := s.Targets[ref]
		sections = append(sections, renderCapped("targets of "+ref, len(targets), len(targets), limit, func(i int) string {
			return renderTarget(targets[i])
		}))
	}
	if len(sections) == 0 {
		return "nothing reached the sink"
	}
	return strings.Join(sections, "\n")
}

// renderTarget shows a target's labels without the __meta_ labels, which
// discovery adds in large numbers and relabel rules consume.
func renderTarget(t map[string]string) string {
	kept := make(map[string]string, len(t))
	hidden := 0
	for k, v := range t {
		if strings.HasPrefix(k, "__meta_") {
			hidden++
			continue
		}
		kept[k] = v
	}
	out := renderStringMap(kept)
	if hidden > 0 {
		out += fmt.Sprintf(" (+%d __meta_ labels)", hidden)
	}
	return out
}

func renderCapped(name string, total, kept, limit int, line func(i int) string) string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "%s (%d):", name, total)
	n := min(kept, limit)
	for i := range n {
		b.WriteString("\n  ")
		b.WriteString(line(i))
	}
	if total > n {
		_, _ = fmt.Fprintf(&b, "\n  … %d more", total-n)
	}
	return b.String()
}

func nameAt(names []string, i int) string {
	if i < len(names) {
		return names[i]
	}
	return ""
}

// sinkPrefix shows which sink captured an item, when it is not the default.
func sinkPrefix(name string) string {
	if name == "" {
		return ""
	}
	return "[" + name + "] "
}

func exportSnapshot(s snapshot) Snapshot {
	out := Snapshot{
		Logs:    make([]LogEntry, 0, min(len(s.loki), snapshotLimit)),
		Metrics: make([]Sample, 0, min(len(s.prometheus), snapshotLimit)),
		Spans:   make([]SpanInfo, 0, min(len(s.spans), snapshotLimit)),
		Totals: SnapshotTotals{
			Logs:           len(s.loki),
			Metrics:        len(s.prometheus),
			Spans:          len(s.spans),
			OTLPDataPoints: s.otlpDataPoints,
			OTLPLogRecords: s.otlpLogRecords,
		},
	}

	for i, e := range s.loki[:min(len(s.loki), snapshotLimit)] {
		labels := make(map[string]string, len(e.Labels))
		for k, v := range e.Labels {
			labels[string(k)] = string(v)
		}
		var sm map[string]string
		if len(e.StructuredMetadata) > 0 {
			sm = make(map[string]string, len(e.StructuredMetadata))
			for _, l := range e.StructuredMetadata {
				sm[l.Name] = l.Value
			}
		}
		out.Logs = append(out.Logs, LogEntry{
			Sink:               nameAt(s.lokiSinks, i),
			Labels:             labels,
			Line:               e.Line,
			Timestamp:          e.Timestamp.UTC(),
			StructuredMetadata: sm,
		})
	}

	for i, p := range s.prometheus[:min(len(s.prometheus), snapshotLimit)] {
		sample := Sample{
			Sink:      nameAt(s.promSinks, i),
			Labels:    p.Labels.Map(),
			Histogram: p.Histogram != nil,
			Timestamp: p.Timestamp,
		}
		switch {
		case p.Histogram != nil:
		case math.IsNaN(p.Value) || math.IsInf(p.Value, 0):
			sample.ValueText = fmt.Sprint(p.Value)
		default:
			v := p.Value
			sample.Value = &v
		}
		if p.Metadata != nil {
			sample.Type = string(p.Metadata.Type)
			sample.Help = p.Metadata.Help
			sample.Unit = p.Metadata.Unit
		}
		out.Metrics = append(out.Metrics, sample)
	}

	for ref, ts := range s.targets {
		if ts.err != nil {
			continue
		}
		if out.Targets == nil {
			out.Targets = map[string][]map[string]string{}
		}
		out.Targets[ref] = ts.targets[:min(len(ts.targets), snapshotLimit)]
	}

	for i, sp := range s.spans[:min(len(s.spans), snapshotLimit)] {
		out.Spans = append(out.Spans, SpanInfo{
			Sink:               nameAt(s.spanSinks, i),
			Name:               sp.Name,
			Kind:               sp.Kind,
			Status:             sp.StatusCode,
			Scope:              sp.Scope,
			TraceID:            sp.TraceID,
			SpanID:             sp.SpanID,
			ParentSpanID:       sp.ParentSpanID,
			Attributes:         sp.Attributes,
			ResourceAttributes: sp.ResourceAttributes,
		})
	}

	return out
}
