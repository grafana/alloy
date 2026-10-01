//go:build ORT

package secretfilter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/hoophq/alcatraz"
	"github.com/hoophq/alcatraz/analyzer"
	"github.com/hoophq/alcatraz/ner"
	"github.com/hoophq/alcatraz/recognizers"
)

// alcatrazDetector combines alcatraz's pattern recognizers with a NER model
// running on ONNX Runtime.
type alcatrazDetector struct {
	engine  *alcatraz.Engine
	ner     *ner.Engine
	options alcatraz.Options
}

func newPIIDetector(args PIIArguments) (piiDetector, error) {
	ignore, err := unmappedLabels(args.ModelPath, args.LabelMapping)
	if err != nil {
		return nil, err
	}

	cfg := ner.DefaultConfig()
	cfg.ModelPath = args.ModelPath
	cfg.Offline = true // never download at runtime
	cfg.Backend = ner.BackendORT
	cfg.ORTLibraryPath = args.ONNXLibraryPath
	cfg.LabelMapping = args.LabelMapping
	cfg.LabelsToIgnore = ignore
	cfg.Segmentation = ner.SegmentLines
	// The largest bucket is alcatraz's window size (capped at the model's own
	// limit). Long windows let long-context models (ettin: ~8k tokens) read a
	// log line in one piece, and avoid an alcatraz v0.21.0 panic in
	// ner.tokenWindows with BPE tokenizers (last token span ends one byte past
	// the text).
	cfg.SequenceBuckets = append(cfg.SequenceBuckets, 512, 1024, 2048, 4096, 8192)

	nlp, err := ner.New(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("load PII NER model: %w", err)
	}

	reg := analyzer.NewRegistry("en")
	recognizers.LoadDefaults(reg, "en") // pattern recognizers
	reg.Add("en", nlp.Recognizer("en")) // + NER
	engine := analyzer.NewEngine(reg, []string{"en"})
	engine.SetNlpEngine(nlp) // run the model once per Analyze call

	minScore := args.MinScore
	return &alcatrazDetector{engine: engine, ner: nlp, options: alcatraz.Options{Threshold: &minScore}}, nil
}

func (d *alcatrazDetector) detect(line string) []piiSpan {
	var spans []piiSpan
	for _, r := range d.engine.Analyze(line, d.options) {
		spans = append(spans, piiSpan{Entity: r.EntityType, Start: r.Start, End: r.End})
	}
	return spans
}

func (d *alcatrazDetector) close() error {
	return d.ner.Close()
}

// unmappedLabels returns the labels in the model's config.json (without
// B-/I-/E-/S- prefixes) that mapping doesn't cover, so alcatraz drops them.
func unmappedLabels(modelDir string, mapping map[string]string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(modelDir, "config.json"))
	if err != nil {
		return nil, fmt.Errorf("read PII NER model labels: %w", err)
	}
	var cfg struct {
		ID2Label map[string]string `json:"id2label"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse PII NER model config: %w", err)
	}

	var ignore []string
	for _, label := range cfg.ID2Label {
		if len(label) > 2 && label[1] == '-' { // B-first_name -> first_name
			label = label[2:]
		}
		if _, mapped := mapping[label]; !mapped && label != "O" && !slices.Contains(ignore, label) {
			ignore = append(ignore, label)
		}
	}
	return ignore, nil
}
