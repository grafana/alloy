package usagestats

import (
	"sort"
	"strings"
	"sync"
)

// Metric keys used in the "metrics" payload of a usage report.
const (
	// enabledComponentsMetric lists the enabled Alloy component names (Default Engine).
	enabledComponentsMetric = "enabled-components"
	// otelComponentsMetric holds the configured OTel Collector component types,
	// grouped by kind (OTel Engine).
	otelComponentsMetric = "otel-components"
	// alloyEngineComponentsMetric lists the Alloy component names run by the
	// alloyengine extension ("an alloy within an alloy").
	alloyEngineComponentsMetric = "alloyengine-components"
)

var otelComponentSections = []string{
	"receivers",
	"processors",
	"exporters",
	"connectors",
	"extensions",
}

// ExtractOtelComponents returns the OTel Collector component types grouped by kind.
// For example: {"receivers":  {"otlp", "prometheus"}, "processors": {"batch"}}
func ExtractOtelComponents(conf map[string]any) map[string][]string {
	out := map[string][]string{}
	for _, section := range otelComponentSections {
		components, ok := conf[section].(map[string]any)
		if !ok {
			continue
		}
		types := map[string]struct{}{}
		for id := range components {
			t := componentType(id)
			if t == "" {
				continue
			}
			types[t] = struct{}{}
		}
		if len(types) == 0 {
			continue
		}
		list := make([]string, 0, len(types))
		for t := range types {
			list = append(list, t)
		}
		sort.Strings(list)
		out[section] = list
	}
	return out
}

// componentType returns the type portion of an OTel component ID. IDs use the
// form "type[/name]"; the type is everything before the first "/".
func componentType(id string) string {
	if idx := strings.IndexByte(id, '/'); idx >= 0 {
		id = id[:idx]
	}
	return strings.TrimSpace(id)
}

// Tracker holds the component information gathered during a run, and produces the
// "metrics" payload for the usage report. Both engines feed the same tracker
//
// Each source registers a getter that is read lazily at report time, so the
// reported lists always reflect the current state. In a given process only one
// top-level engine runs, so Metrics only ever emits the keys relevant to that
// engine.
type Tracker struct {
	mut                       sync.Mutex
	enabledComponentsFunc     func() []string
	otelComponentsFunc        func() map[string][]string
	alloyEngineComponentsFunc func() []string
}

// GlobalTracker is the process-wide tracker read by the usage reporter. It is a
// singleton because the component information is populated from several
// independent places in the same process (the Default Engine runtime, the OTel
// confmap converter, and the alloyengine extension).
var GlobalTracker = &Tracker{}

// SetEnabledComponentsFunc registers a getter for the Default Engine's enabled
// component names.
func (t *Tracker) SetEnabledComponentsFunc(fn func() []string) {
	t.mut.Lock()
	defer t.mut.Unlock()
	t.enabledComponentsFunc = fn
}

// SetOTelComponentsFunc registers a getter for the configured OTel Collector
// component types, grouped by kind. It is called by the confmap converter on
// every config resolve.
func (t *Tracker) SetOTelComponentsFunc(fn func() map[string][]string) {
	t.mut.Lock()
	defer t.mut.Unlock()
	t.otelComponentsFunc = fn
}

// SetAlloyEngineComponentsFunc registers a getter for the Alloy component names
// run by the alloyengine extension.
func (t *Tracker) SetAlloyEngineComponentsFunc(fn func() []string) {
	t.mut.Lock()
	defer t.mut.Unlock()
	t.alloyEngineComponentsFunc = fn
}

// Metrics returns the "metrics" payload for the usage report. Only the keys whose
// sources have been registered are included.
func (t *Tracker) Metrics() map[string]any {
	t.mut.Lock()
	enabledComponentsFunc := t.enabledComponentsFunc
	otelComponentsFunc := t.otelComponentsFunc
	alloyEngineComponentsFunc := t.alloyEngineComponentsFunc
	t.mut.Unlock()

	metrics := map[string]any{}
	if enabledComponentsFunc != nil {
		metrics[enabledComponentsMetric] = enabledComponentsFunc()
	}
	if otelComponentsFunc != nil {
		metrics[otelComponentsMetric] = otelComponentsFunc()
	}
	if alloyEngineComponentsFunc != nil {
		metrics[alloyEngineComponentsMetric] = alloyEngineComponentsFunc()
	}
	return metrics
}
