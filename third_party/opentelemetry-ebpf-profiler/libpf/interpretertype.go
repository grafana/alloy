// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libpf // import "go.opentelemetry.io/ebpf-profiler/libpf"

import "go.opentelemetry.io/ebpf-profiler/support"

// InterpreterType variables can hold one of the interpreter type values defined below.
type InterpreterType int

const (
	// UnknownInterp signifies that the interpreter is unknown.
	UnknownInterp InterpreterType = support.FrameMarkerUnknown
	// PHP identifies the PHP interpreter.
	PHP InterpreterType = support.FrameMarkerPHP
	// PHPJIT identifies PHP JIT processes.
	PHPJIT InterpreterType = support.FrameMarkerPHPJIT
	// Python identifies the Python interpreter.
	Python InterpreterType = support.FrameMarkerPython
	// Native identifies native code.
	Native InterpreterType = support.FrameMarkerNative
	// Kernel identifies kernel code.
	Kernel InterpreterType = support.FrameMarkerKernel
	// HotSpot identifies the Java HotSpot VM.
	HotSpot InterpreterType = support.FrameMarkerHotSpot
	// Ruby identifies the Ruby interpreter.
	Ruby InterpreterType = support.FrameMarkerRuby
	// Perl identifies the Perl interpreter.
	Perl InterpreterType = support.FrameMarkerPerl
	// V8 identifies the V8 interpreter.
	V8 InterpreterType = support.FrameMarkerV8
	// Dotnet identifies the Dotnet interpreter.
	Dotnet InterpreterType = support.FrameMarkerDotnet
	// Go identifies Go code.
	Go InterpreterType = support.FrameMarkerGo
)

// Pseudo-interpreters without a corresponding frame type.
const (
	// pseudoInterpreterStart marks the start of the pseudo interpreter ID space.
	pseudoInterpreterStart InterpreterType = 0x100

	// APMInt identifies the pseudo-interpreter for the APM integration.
	APMInt InterpreterType = 0x100

	// Go identifies the pseudo-interpreter for Go custom labels support.
	GoLabels InterpreterType = 0x101
)

// Frame converts the interpreter type into the corresponding frame type.
func (i InterpreterType) Frame() FrameType {
	if i >= pseudoInterpreterStart {
		return unknownFrame
	}

	return FrameType(i)
}

var interpreterTypeToString = map[InterpreterType]string{
	UnknownInterp: "unknown",
	PHP:           "php",
	// OTel SemConv does not differentiate between jitted code and interpreted code.
	PHPJIT:   "php",
	Python:   "cpython",
	Native:   "native",
	Kernel:   "kernel",
	HotSpot:  "jvm",
	Ruby:     "ruby",
	Perl:     "perl",
	V8:       "v8js",
	Dotnet:   "dotnet",
	APMInt:   "apm-integration",
	Go:       "go",
	GoLabels: "go-labels",
}

var stringToInterpreterType = make(map[string]InterpreterType, len(interpreterTypeToString))

func init() {
	for k, v := range interpreterTypeToString {
		if k == PHPJIT {
			continue
		}
		stringToInterpreterType[v] = k
	}
}

func InterpreterTypeFromString(name string) InterpreterType {
	if result, ok := stringToInterpreterType[name]; ok {
		return result
	}
	return UnknownInterp
}

// String converts the frame type int to the related string value to be displayed in the UI.
func (i InterpreterType) String() string {
	if result, ok := interpreterTypeToString[i]; ok {
		return result
	}
	//nolint:goconst
	return "<invalid>"
}
