package secretfilter

import (
	"cmp"
	"errors"
	"reflect"
	"slices"
	"strings"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
)

// PIIArguments configures the optional PII redaction stage, which runs after
// the gitleaks secret redaction. It combines alcatraz's pattern recognizers
// (emails, cards, IBANs, ... with checksum validation) with a NER model for
// names, places, passwords and API keys. Requires an Alloy build with -tags ORT.
type PIIArguments struct {
	Enabled         bool              `alloy:"enabled,attr,optional"`           // Enables PII redaction. Disabled by default.
	ModelPath       string            `alloy:"model_path,attr,optional"`        // Local NER model directory (model.onnx, tokenizer.json, config.json), e.g. ettin-17m-nemotron-pii.
	ONNXLibraryPath string            `alloy:"onnx_library_path,attr,optional"` // libonnxruntime.so file or its directory. Empty uses the platform default (/usr/lib on Linux).
	LabelMapping    map[string]string `alloy:"label_mapping,attr,optional"`     // NER model label -> entity name. Labels not listed are ignored. Defaults to a mapping for the ettin nemotron-pii models.
	IgnoreEntities  []string          `alloy:"ignore_entities,attr,optional"`   // Entity types never redacted (pattern or NER).
	MinScore        float64           `alloy:"min_score,attr,optional"`         // Detections scoring below this are not redacted.
	RedactWith      string            `alloy:"redact_with,attr,optional"`       // Replacement template; $ENTITY and $ENTITY_HASH are replaced.
}

// DefaultPIIArguments are the defaults for the pii block.
var DefaultPIIArguments = PIIArguments{
	// Mapping for kalyan-ks/ettin-*-nemotron-pii: only what the patterns can't find.
	LabelMapping: map[string]string{
		"first_name":     "PERSON",
		"last_name":      "PERSON",
		"street_address": "LOCATION",
		"city":           "LOCATION",
		"state":          "LOCATION",
		"county":         "LOCATION",
		"country":        "LOCATION",
		"postcode":       "LOCATION",
		"password":       "PASSWORD",
		"api_key":        "API_KEY",
	},
	// Country-specific plate formats that match log tokens like "nio-8080" (thread names).
	IgnoreEntities: []string{"BR_PLACA", "IN_VEHICLE_REGISTRATION"},
	MinScore:       0.5,
	RedactWith:     "<$ENTITY>",
}

// SetToDefault implements syntax.Defaulter.
func (a *PIIArguments) SetToDefault() {
	*a = DefaultPIIArguments
}

func (a *PIIArguments) validate() error {
	if a.Enabled && a.ModelPath == "" {
		return errors.New("secretfilter: pii.model_path is required when pii.enabled is true")
	}
	return nil
}

// piiSpan is one detected piece of PII: byte offsets [Start, End) into the line.
type piiSpan struct {
	Entity     string
	Start, End int
}

// piiDetector finds PII in a log line. Implemented in pii_ort.go (-tags ORT).
type piiDetector interface {
	detect(line string) []piiSpan
	close() error
}

// presenceGate decides cheaply whether a line may contain PII at all, so only
// those lines pay for the PII detector.
//
// TODO: placeholder for a very fast presence gate (being worked on separately).
// Plug an implementation into piiStage.gate in newPIIStage.
type presenceGate interface {
	mayContainPII(line string) bool
}

// piiStage redacts PII in log entries.
type piiStage struct {
	args     PIIArguments // the settings it was built with, to detect changes
	gate     presenceGate // nil: every line goes to the detector
	detector piiDetector

	redactedTotal *prometheus.CounterVec
}

// buildPIIStage returns the PII stage for args. It reuses current when the
// settings are unchanged (loading the model is expensive) and returns nil when
// PII redaction is disabled.
func buildPIIStage(current *piiStage, args PIIArguments, reg prometheus.Registerer) (*piiStage, error) {
	if !args.Enabled {
		return nil, nil
	}
	if current != nil && reflect.DeepEqual(current.args, args) {
		return current, nil
	}

	detector, err := newPIIDetector(args)
	if err != nil {
		return nil, err
	}

	redactedTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: "loki_secretfilter",
		Name:      "pii_redacted_total",
		Help:      "Number of PII values redacted, partitioned by entity type.",
	}, []string{"entity"})
	if reg != nil {
		redactedTotal = util.MustRegisterOrGet(reg, redactedTotal).(*prometheus.CounterVec)
	}

	return &piiStage{
		args:          args,
		gate:          nil, // TODO: presence gate goes here.
		detector:      detector,
		redactedTotal: redactedTotal,
	}, nil
}

// redactEntry replaces every PII value in the entry's line with the redact_with template.
func (s *piiStage) redactEntry(entry loki.Entry) loki.Entry {
	if s.gate != nil && !s.gate.mayContainPII(entry.Line) {
		return entry
	}

	spans := s.detector.detect(entry.Line)
	if len(spans) == 0 {
		return entry
	}

	// Replace from left to right; on overlap keep the earlier (then longer) span.
	slices.SortFunc(spans, func(a, b piiSpan) int {
		return cmp.Or(cmp.Compare(a.Start, b.Start), cmp.Compare(b.End, a.End))
	})
	line := entry.Line
	var b strings.Builder
	last := 0
	for _, sp := range spans {
		// BPE tokenizers (ettin) include the preceding space in a span; keep it in the line.
		for sp.Start < sp.End && sp.Start < len(line) && line[sp.Start] == ' ' {
			sp.Start++
		}
		if sp.Start < last || sp.Start >= sp.End || sp.End > len(line) || slices.Contains(s.args.IgnoreEntities, sp.Entity) {
			continue
		}
		b.WriteString(line[last:sp.Start])
		replacement := strings.ReplaceAll(s.args.RedactWith, "$ENTITY_HASH", hashSecret(line[sp.Start:sp.End]))
		b.WriteString(strings.ReplaceAll(replacement, "$ENTITY", sp.Entity))
		last = sp.End
		s.redactedTotal.WithLabelValues(sp.Entity).Inc()
	}
	b.WriteString(line[last:])

	entry.Line = b.String()
	return entry
}

func (s *piiStage) close() error {
	return s.detector.close()
}
