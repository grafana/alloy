// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libpf // import "go.opentelemetry.io/ebpf-profiler/libpf"

import (
	"go.opentelemetry.io/ebpf-profiler/support"
)

// FrameType defines the type of frame. This usually corresponds to the interpreter type that
// emitted it, but can additionally contain meta-information like error frames.
//
// A frame type can represent one of the following things:
//
//   - A successfully unwound frame. This is represented simply as the `InterpreterType` ID.
//   - A partial (non-critical failure), indicated by ORing the `InterpreterType` ID with
//     the error bit.
//   - A fatal failure that caused further unwinding to be aborted. This is indicated using the
//     special value support.FrameMarkerAbort (0xFF). It thus also contains the error bit, but
//     does not fit into the `InterpreterType` enum.
type FrameType int

// Convenience shorthands to create various frame types.
//
// Code should not compare against the constants below directly, but instead use the provided
// methods to query the required information (IsError, Interpreter, ...) to improve forward
// compatibility and clarify intentions.
const (
	// unknownFrame indicates a frame of an unknown interpreter.
	// If this appears, it's likely a bug somewhere.
	unknownFrame FrameType = support.FrameMarkerUnknown
	// PHPFrame identifies PHP interpreter frames.
	PHPFrame FrameType = support.FrameMarkerPHP
	// PHPJITFrame identifies PHP JIT interpreter frames.
	PHPJITFrame FrameType = support.FrameMarkerPHPJIT
	// PythonFrame identifies the Python interpreter frames.
	PythonFrame FrameType = support.FrameMarkerPython
	// NativeFrame identifies native frames.
	NativeFrame FrameType = support.FrameMarkerNative
	// KernelFrame identifies kernel frames.
	KernelFrame FrameType = support.FrameMarkerKernel
	// HotSpotFrame identifies Java HotSpot VM frames.
	HotSpotFrame FrameType = support.FrameMarkerHotSpot
	// RubyFrame identifies the Ruby interpreter frames.
	RubyFrame FrameType = support.FrameMarkerRuby
	// PerlFrame identifies the Perl interpreter frames.
	PerlFrame FrameType = support.FrameMarkerPerl
	// V8Frame identifies the V8 interpreter frames.
	V8Frame FrameType = support.FrameMarkerV8
	// DotnetFrame identifies the Dotnet interpreter frames.
	DotnetFrame FrameType = support.FrameMarkerDotnet
	// GoFrame identifies Go frames.
	GoFrame FrameType = support.FrameMarkerGo
	// AbortFrame identifies frames that report that further unwinding was aborted due to an error.
	AbortFrame FrameType = support.FrameMarkerAbort
)

const (
	abortFrameName = "abort-marker"
)

func FrameTypeFromString(name string) FrameType {
	if name == abortFrameName {
		return AbortFrame
	}
	return InterpreterTypeFromString(name).Frame()
}

// Interpreter returns the interpreter that produced the frame.
func (ty FrameType) Interpreter() InterpreterType {
	switch ty {
	case support.FrameMarkerAbort, support.FrameMarkerUnknown:
		return UnknownInterp
	default:
		return InterpreterType(ty &^ support.FrameMarkerErrorBit)
	}
}

// IsInterpType checks whether the frame type belongs to the given interpreter.
func (ty FrameType) IsInterpType(ity InterpreterType) bool {
	return ity == ty.Interpreter()
}

// Error adds the error bit into the frame type.
func (ty FrameType) Error() FrameType {
	return ty | support.FrameMarkerErrorBit
}

// IsError checks whether the frame is an error frame.
func (ty FrameType) IsError() bool {
	return ty&support.FrameMarkerErrorBit != 0
}

// String implements the Stringer interface.
func (ty FrameType) String() string {
	switch ty {
	case support.FrameMarkerAbort:
		return abortFrameName
	default:
		interp := ty.Interpreter()
		return interp.String()
	}
}
