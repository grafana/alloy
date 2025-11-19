// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package types // import "go.opentelemetry.io/ebpf-profiler/tracer/types"

import (
	"fmt"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"
)

// tracerType values identify tracers, such as the native code tracer, or PHP tracer
type tracerType int

const (
	PerlTracer tracerType = iota
	PHPTracer
	PythonTracer
	HotspotTracer
	RubyTracer
	V8Tracer
	DotnetTracer
	GoTracer
	Labels

	// maxTracers indicates the max. number of different tracers
	maxTracers
)

var tracerTypeToName = map[tracerType]string{
	PerlTracer:    "perl",
	PHPTracer:     "php",
	PythonTracer:  "python",
	HotspotTracer: "hotspot",
	RubyTracer:    "ruby",
	V8Tracer:      "v8",
	DotnetTracer:  "dotnet",
	GoTracer:      "go",
	Labels:        "labels",
}

var tracerNameToType = make(map[string]tracerType, maxTracers)

func init() {
	for k, v := range tracerTypeToName {
		tracerNameToType[v] = k
	}
}

// tracerTypeFromName returns the tracer type for the given name.
func tracerTypeFromName(s string) (tracerType, bool) {
	tt, ok := tracerNameToType[s]
	return tt, ok
}

// String returns the tracer's name.
// It returns '<unknown>' in case the tracer is unknown.
func (t tracerType) String() string {
	if result, ok := tracerTypeToName[t]; ok {
		return result
	}

	return "<unknown>"
}

// IncludedTracers holds information about which tracers are enabled.
type IncludedTracers uint16

// String returns a comma-separated list of enabled tracers.
func (t *IncludedTracers) String() string {
	var names []string
	for tracer := range maxTracers {
		if t.Has(tracer) {
			names = append(names, tracer.String())
		}
	}
	return strings.Join(names, ",")
}

// Has returns true if the given tracer is enabled.
func (t *IncludedTracers) Has(tracer tracerType) bool {
	return *t&(1<<tracer) != 0
}

// Enable enables the given tracer.
func (t *IncludedTracers) Enable(tracer tracerType) {
	*t |= 1 << tracer
}

// Disable disables the given tracer.
func (t *IncludedTracers) Disable(tracer tracerType) {
	*t &= ^(1 << tracer)
}

// enableAll enables all known tracers.
func (t *IncludedTracers) enableAll() {
	for tracer := range maxTracers {
		t.Enable(tracer)
	}
}

// enableByName enables the given tracer by its name.
func (t *IncludedTracers) enableByName(name string) bool {
	tracer, ok := tracerTypeFromName(name)
	if ok {
		t.Enable(tracer)
	}
	return ok
}

// Parse parses a string that specifies one or more eBPF tracers to enable.
// Valid inputs are 'all', or any comma-delimited combination of names listed in tracerTypeToName.
// The return value holds the information whether a tracer has been set or not.
// E.g. to check if the Python tracer was requested: `if result.Has(tracertypes.PythonTracer)...`.
func Parse(tracers string) (IncludedTracers, error) {
	var result IncludedTracers

	// Parse and validate tracers string.
	for _, name := range strings.Split(tracers, ",") {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}

		if result.enableByName(name) {
			continue
		}

		switch name {
		case "all":
			result.enableAll()
			if runtime.GOARCH == "arm64" {
				result.Disable(DotnetTracer)
			}
		case "native":
			log.Warn("Enabling the `native` tracer explicitly is deprecated (it's always-on)")
		default:
			return result, fmt.Errorf("unknown tracer: %s", name)
		}
	}

	if runtime.GOARCH == "arm64" {
		if result.Has(DotnetTracer) {
			result.Disable(DotnetTracer)
			log.Warn("The dotnet tracer is currently not supported on ARM64")
		}
	}

	if tracersEnabled := result.String(); tracersEnabled != "" {
		log.Debugf("Tracer string: %v", tracers)
		log.Infof("Interpreter tracers: %v", tracersEnabled)
	}

	return result, nil
}

// AllTracers is a shortcut that returns an element with all
// tracers enabled.
func AllTracers() IncludedTracers {
	var result IncludedTracers
	result.enableAll()
	return result
}
