package pipelinetest

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/pipelinetest/harness"
	"github.com/grafana/alloy/syntax/scanner"
	builder "github.com/grafana/alloy/syntax/token/builder"
)

// builtSource is the Alloy source a test runs, and the IDs of the
// components the harness reads from or sends to.
type builtSource struct {
	source string
	// sinks maps each exit ID to its sink's component ID.
	sinks map[string]string
	// inputs maps the name of a generated pipelinetest.source to its
	// component IDs. A source inside a module has one ID per module
	// instance. A name that is missing has the top-level ID.
	inputs inputComponents
}

// inputComponents maps the name of a generated pipelinetest.source to its
// component IDs.
type inputComponents map[string][]string

// ids returns the component IDs of the source of one input.
func (c inputComponents) ids(sig signal, i int) []string {
	if ids, ok := c[generatedInputComponentName(sig, i)]; ok {
		return ids
	}
	return []string{generatedInputComponentID(sig, i)}
}

// sources returns the running pipelinetest.source components of one input.
func (c inputComponents) sources(alloy *harness.Alloy, sig signal, i int) ([]*harness.Source, error) {
	var out []*harness.Source
	for _, id := range c.ids(sig, i) {
		s, err := harness.WaitComponent[*harness.Source](alloy, id)
		if err != nil {
			return nil, fmt.Errorf("%s input %d: %w", sig, i, err)
		}
		out = append(out, s)
	}
	return out, nil
}

// buildSource builds the Alloy source a test runs: the transformed config,
// then the sinks, mocks and input sources. Generated code goes after the
// config, so error line numbers still match the user's file. An input that
// replaces a component inside a module is the exception: its source goes
// inside the module, in place of the component.
func buildSource(schema TestSchema) (builtSource, error) {
	inputs := collectInputs(schema.Inputs)

	var replacements []sourceReplacement
	for _, in := range inputs {
		if in.replace != "" {
			replacements = append(replacements, sourceReplacement{id: in.replace, signal: in.signal, name: generatedInputComponentName(in.signal, in.index)})
		}
	}

	discovery, err := targetLiterals(schema.Inputs.Targets)
	if err != nil {
		return builtSource{}, err
	}

	overrides, err := overrideExprs(schema.Config.Override)
	if err != nil {
		return builtSource{}, err
	}

	transformed, err := transformConfig(configName(schema.Config), schema.Config.Source, transformSpec{
		sources:      replacements,
		sinks:        schema.Config.Sinks,
		remove:       schema.Config.Remove,
		discovery:    discovery,
		overrides:    overrides,
		writeRelabel: schema.Config.KeepWriteRelabel,
	})
	if err != nil {
		return builtSource{}, err
	}

	generated, err := buildSources(inputs, transformed, schema.Mocks)
	if err != nil {
		return builtSource{}, err
	}

	out := builtSource{sinks: map[string]string{}, inputs: inputComponents{}}
	for _, s := range transformed.sinks {
		out.sinks[s.exit] = s.componentID()
	}
	for _, r := range replacements {
		src := transformed.sources[r.id]
		for _, m := range src.modules {
			out.inputs[r.name] = append(out.inputs[r.name], m+"/pipelinetest.source."+r.name)
		}
	}

	out.source = rewritePipelineTestRefs(transformed.source) + "\n\n" + buildSinks(transformed.sinks) + "\n\n" + generated
	return out, nil
}

func configName(cfg ConfigSchema) string {
	if cfg.Path != "" {
		return cfg.Path
	}
	return "config"
}

func rewritePipelineTestRefs(source string) string {
	replacer := strings.NewReplacer(
		"pipelinetest.loki.url", "pipelinetest.sink.out.loki_push_url",
		"pipelinetest.loki.receiver", "pipelinetest.sink.out.loki_receiver",
		"pipelinetest.prometheus.url", "pipelinetest.sink.out.prometheus_remote_write_url",
		"pipelinetest.prometheus.receiver", "pipelinetest.sink.out.prometheus_receiver",
		"pipelinetest.otel.input", "pipelinetest.sink.out.otel_input",
	)
	return replacer.Replace(source)
}

// buildSinks writes the default sink, which inline configs send to, and one
// sink per replaced exit component, carrying its external_labels.
func buildSinks(sinks []generatedSink) string {
	var b strings.Builder
	b.WriteString(`pipelinetest.sink "out" {}`)
	for _, s := range sinks {
		fmt.Fprintf(&b, "\n\npipelinetest.sink %q {", s.label)
		if s.externalLabels != "" {
			fmt.Fprintf(&b, "\n\texternal_labels = %s\n", s.externalLabels)
		}
		for _, rule := range s.writeRelabel {
			fmt.Fprintf(&b, "\n\t%s\n", rule)
		}
		b.WriteString("}")
	}
	return b.String()
}

// testInput is one input of any signal that needs a generated source.
type testInput struct {
	signal     signal
	index      int
	components []string
	replace    string
	hasData    bool
}

func collectInputs(inputs InputSchema) []testInput {
	var out []testInput
	for i, in := range inputs.Loki {
		out = append(out, testInput{signal: signalLogs, index: i, components: in.Components, replace: in.Replace, hasData: len(in.Entries) > 0})
	}
	for i, in := range inputs.Prometheus {
		out = append(out, testInput{signal: signalMetrics, index: i, components: in.Components, replace: in.Replace, hasData: len(in.Samples) > 0})
	}
	for i, in := range inputs.Traces {
		out = append(out, testInput{signal: signalTraces, index: i, components: in.Components, replace: in.Replace, hasData: len(in.Spans) > 0})
	}
	return out
}

// buildSources generates the mocks and one pipelinetest.source per input.
func buildSources(inputs []testInput, cfg transformedConfig, mocks MockSchema) (string, error) {
	var out strings.Builder

	file := builder.NewFile()
	appendMocks(file.Body(), mocks)
	if len(file.Body().Nodes()) > 0 {
		out.Write(file.Bytes())
		out.WriteString("\n")
	}

	for _, in := range inputs {
		if len(in.components) > 0 && in.replace != "" {
			return "", fmt.Errorf("%s input %d: set components or replace, not both", in.signal, in.index)
		}

		var forwardTo string
		switch {
		case in.replace != "":
			if len(cfg.sources[in.replace].modules) > 0 {
				// The source is inside the module.
				continue
			}
			forwardTo = cfg.sources[in.replace].forwardTo
		case len(in.components) > 0:
			targets := make([]string, 0, len(in.components))
			for _, c := range in.components {
				if !isReference(c) {
					return "", fmt.Errorf("%s input %d: %q is not a component export such as loki.process.default.receiver", in.signal, in.index, c)
				}
				if replacement, ok := cfg.sinkRefs[c]; ok {
					c = replacement
				}
				targets = append(targets, rewritePipelineTestRefs(c))
			}
			forwardTo = "[" + strings.Join(targets, ", ") + "]"
		case in.hasData:
			return "", fmt.Errorf("%s input %d: set components or replace to say where its data goes", in.signal, in.index)
		default:
			continue
		}

		fmt.Fprintf(&out, "pipelinetest.source %q {\n\tforward_to {\n\t\t%s = %s\n\t}\n}\n\n",
			generatedInputComponentName(in.signal, in.index), in.signal, forwardTo)
	}

	return out.String(), nil
}

// isReference reports whether s is a dotted chain of identifiers, so it can be
// pasted into the generated config as is.
func isReference(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		if !scanner.IsValidIdentifier(p) {
			return false
		}
	}
	return true
}

func generatedInputComponentID(sig signal, i int) string {
	return "pipelinetest.source." + generatedInputComponentName(sig, i)
}

func generatedInputComponentName(sig signal, i int) string {
	return fmt.Sprintf("__pipelinetest_%s_input_%d", sig, i)
}
