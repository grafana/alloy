//go:build ignore

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package support // import "go.opentelemetry.io/ebpf-profiler/support"

import (
	"go.opentelemetry.io/ebpf-profiler/metrics"
)

/*
#include "./ebpf/types.h"
#include "./ebpf/frametypes.h"
#include "./ebpf/stackdeltatypes.h"
#include "./ebpf/v8_tracer.h"
*/
import "C"

const (
	FrameMarkerUnknown  = C.FRAME_MARKER_UNKNOWN
	FrameMarkerErrorBit = C.FRAME_MARKER_ERROR_BIT
	FrameMarkerPython   = C.FRAME_MARKER_PYTHON
	FrameMarkerNative   = C.FRAME_MARKER_NATIVE
	FrameMarkerPHP      = C.FRAME_MARKER_PHP
	FrameMarkerPHPJIT   = C.FRAME_MARKER_PHP_JIT
	FrameMarkerKernel   = C.FRAME_MARKER_KERNEL
	FrameMarkerHotSpot  = C.FRAME_MARKER_HOTSPOT
	FrameMarkerRuby     = C.FRAME_MARKER_RUBY
	FrameMarkerPerl     = C.FRAME_MARKER_PERL
	FrameMarkerV8       = C.FRAME_MARKER_V8
	FrameMarkerDotnet   = C.FRAME_MARKER_DOTNET
	FrameMarkerGo       = C.FRAME_MARKER_GO
	FrameMarkerAbort    = C.FRAME_MARKER_ABORT
)

const (
	ProgUnwindStop    = C.PROG_UNWIND_STOP
	ProgUnwindNative  = C.PROG_UNWIND_NATIVE
	ProgUnwindHotspot = C.PROG_UNWIND_HOTSPOT
	ProgUnwindPython  = C.PROG_UNWIND_PYTHON
	ProgUnwindPHP     = C.PROG_UNWIND_PHP
	ProgUnwindRuby    = C.PROG_UNWIND_RUBY
	ProgUnwindPerl    = C.PROG_UNWIND_PERL
	ProgUnwindV8      = C.PROG_UNWIND_V8
	ProgUnwindDotnet  = C.PROG_UNWIND_DOTNET
	ProgGoLabels      = C.PROG_GO_LABELS
)

const (
	DeltaCommandFlag = C.STACK_DELTA_COMMAND_FLAG

	MergeOpcodeNegative = C.MERGEOPCODE_NEGATIVE
)

const (
	EventTypeGenericPID = C.EVENT_TYPE_GENERIC_PID
)

const MaxFrameUnwinds = C.MAX_FRAME_UNWINDS

const UnwindInfoMaxEntries = C.UNWIND_INFO_MAX_ENTRIES

const (
	MetricIDBeginCumulative = C.metricID_BeginCumulative
)

const (
	BitWidthPID  = C.BIT_WIDTH_PID
	BitWidthPage = C.BIT_WIDTH_PAGE
)

const (
	// StackDeltaBucket[Smallest|Largest] define the boundaries of the bucket sizes of the various
	// nested stack delta maps.
	StackDeltaBucketSmallest = C.STACK_DELTA_BUCKET_SMALLEST
	StackDeltaBucketLargest  = C.STACK_DELTA_BUCKET_LARGEST

	// StackDeltaPage[Bits|Mask] determine the paging size of stack delta map information
	StackDeltaPageBits = C.STACK_DELTA_PAGE_BITS
	StackDeltaPageMask = C.STACK_DELTA_PAGE_MASK
)

const (
	HSTSIDIsStubBit       = C.HS_TSID_IS_STUB_BIT
	HSTSIDHasFrameBit     = C.HS_TSID_HAS_FRAME_BIT
	HSTSIDStackDeltaBit   = C.HS_TSID_STACK_DELTA_BIT
	HSTSIDStackDeltaMask  = C.HS_TSID_STACK_DELTA_MASK
	HSTSIDStackDeltaScale = C.HS_TSID_STACK_DELTA_SCALE
	HSTSIDSegMapBit       = C.HS_TSID_SEG_MAP_BIT
	HSTSIDSegMapMask      = C.HS_TSID_SEG_MAP_MASK
)

const (
	// PerfMaxStackDepth is the bpf map data array length for BPF_MAP_TYPE_STACK_TRACE traces
	PerfMaxStackDepth = C.PERF_MAX_STACK_DEPTH
)

const (
	TraceOriginUnknown  = C.TRACE_UNKNOWN
	TraceOriginSampling = C.TRACE_SAMPLING
	TraceOriginOffCPU   = C.TRACE_OFF_CPU
	TraceOriginUProbe   = C.TRACE_UPROBE
)

type ApmSpanID C.ApmSpanID
type ApmTraceID C.ApmTraceID
type CustomLabel C.CustomLabel
type CustomLabelsArray C.CustomLabelsArray
type Event C.Event
type Frame C.Frame
type OffsetRange C.OffsetRange
type PIDPage C.PIDPage
type PIDPageMappingInfo C.PIDPageMappingInfo
type StackDelta C.StackDelta
type StackDeltaPageInfo C.StackDeltaPageInfo
type StackDeltaPageKey C.StackDeltaPageKey
type SystemAnalysis C.SystemAnalysis
type TSDInfo C.TSDInfo
type Trace C.Trace
type UnwindInfo C.UnwindInfo

type ApmIntProcInfo C.ApmIntProcInfo
type DotnetProcInfo C.DotnetProcInfo
type GoLabelsOffsets C.GoLabelsOffsets
type HotspotProcInfo C.HotspotProcInfo
type PHPProcInfo C.PHPProcInfo
type PerlProcInfo C.PerlProcInfo
type PyProcInfo C.PyProcInfo
type RubyProcInfo C.RubyProcInfo
type V8ProcInfo C.V8ProcInfo

const (
	Sizeof_Frame      = C.sizeof_Frame
	Sizeof_StackDelta = C.sizeof_StackDelta
	Sizeof_Trace      = C.sizeof_Trace

	sizeof_ApmIntProcInfo = C.sizeof_ApmIntProcInfo
	sizeof_DotnetProcInfo = C.sizeof_DotnetProcInfo
	sizeof_PHPProcInfo    = C.sizeof_PHPProcInfo
	sizeof_RubyProcInfo   = C.sizeof_RubyProcInfo
)

const (
	// UnwindOpcodes from the C header file
	UnwindOpcodeCommand   uint8 = C.UNWIND_OPCODE_COMMAND
	UnwindOpcodeBaseCFA   uint8 = C.UNWIND_OPCODE_BASE_CFA
	UnwindOpcodeBaseSP    uint8 = C.UNWIND_OPCODE_BASE_SP
	UnwindOpcodeBaseFP    uint8 = C.UNWIND_OPCODE_BASE_FP
	UnwindOpcodeBaseLR    uint8 = C.UNWIND_OPCODE_BASE_LR
	UnwindOpcodeBaseReg   uint8 = C.UNWIND_OPCODE_BASE_REG
	UnwindOpcodeFlagDeref uint8 = C.UNWIND_OPCODEF_DEREF

	// UnwindCommands from the C header file
	UnwindCommandInvalid      int32 = C.UNWIND_COMMAND_INVALID
	UnwindCommandStop         int32 = C.UNWIND_COMMAND_STOP
	UnwindCommandPLT          int32 = C.UNWIND_COMMAND_PLT
	UnwindCommandSignal       int32 = C.UNWIND_COMMAND_SIGNAL
	UnwindCommandFramePointer int32 = C.UNWIND_COMMAND_FRAME_POINTER

	// UnwindDeref handling from the C header file
	UnwindDerefMask       int32 = C.UNWIND_DEREF_MASK
	UnwindDerefMultiplier int32 = C.UNWIND_DEREF_MULTIPLIER
)

const (
	// Hotspot specific
	FrameHotspotStub        = C.FRAME_HOTSPOT_STUB
	FrameHotspotVtable      = C.FRAME_HOTSPOT_VTABLE
	FrameHotspotInterpreter = C.FRAME_HOTSPOT_INTERPRETER
	FrameHotspotNative      = C.FRAME_HOTSPOT_NATIVE

	// V8 specific
	V8SmiTag            = C.V8_SmiTag
	V8SmiTagMask        = C.V8_SmiTagMask
	V8SmiTagShift       = C.V8_SmiTagShift
	V8SmiValueShift     = C.V8_SmiValueShift
	V8HeapObjectTag     = C.V8_HeapObjectTag
	V8HeapObjectTagMask = C.V8_HeapObjectTagMask

	V8FpContextSize = C.V8_FP_CONTEXT_SIZE

	V8FileTypeMarker       = C.V8_FILE_TYPE_MARKER
	V8FileTypeByteCode     = C.V8_FILE_TYPE_BYTECODE
	V8FileTypeNativeSFI    = C.V8_FILE_TYPE_NATIVE_SFI
	V8FileTypeNativeCode   = C.V8_FILE_TYPE_NATIVE_CODE
	V8FileTypeNativeJSFunc = C.V8_FILE_TYPE_NATIVE_JSFUNC
	V8FileTypeMask         = C.V8_FILE_TYPE_MASK

	V8LineCookieShift = C.V8_LINE_COOKIE_SHIFT
	V8LineCookieMask  = C.V8_LINE_COOKIE_MASK
	V8LineDeltaMask   = C.V8_LINE_DELTA_MASK
)

var MetricsTranslation = []metrics.MetricID{
	C.metricID_UnwindCallInterpreter:                      metrics.IDUnwindCallInterpreter,
	C.metricID_UnwindErrZeroPC:                            metrics.IDUnwindErrZeroPC,
	C.metricID_UnwindErrStackLengthExceeded:               metrics.IDUnwindErrStackLengthExceeded,
	C.metricID_UnwindErrBadTSDAddr:                        metrics.IDUnwindErrBadTLSAddr,
	C.metricID_UnwindErrBadTPBaseAddr:                     metrics.IDUnwindErrBadTPBaseAddr,
	C.metricID_UnwindNativeAttempts:                       metrics.IDUnwindNativeAttempts,
	C.metricID_UnwindNativeFrames:                         metrics.IDUnwindNativeFrames,
	C.metricID_UnwindNativeStackDeltaStop:                 metrics.IDUnwindNativeStackDeltaStop,
	C.metricID_UnwindNativeErrLookupTextSection:           metrics.IDUnwindNativeErrLookupTextSection,
	C.metricID_UnwindNativeErrLookupIterations:            metrics.IDUnwindNativeErrLookupIterations,
	C.metricID_UnwindNativeErrLookupRange:                 metrics.IDUnwindNativeErrLookupRange,
	C.metricID_UnwindNativeErrKernelAddress:               metrics.IDUnwindNativeErrKernelAddress,
	C.metricID_UnwindNativeErrWrongTextSection:            metrics.IDUnwindNativeErrWrongTextSection,
	C.metricID_UnwindNativeErrPCRead:                      metrics.IDUnwindNativeErrPCRead,
	C.metricID_UnwindPythonAttempts:                       metrics.IDUnwindPythonAttempts,
	C.metricID_UnwindPythonFrames:                         metrics.IDUnwindPythonFrames,
	C.metricID_UnwindPythonErrBadPyThreadStateCurrentAddr: metrics.IDUnwindPythonErrBadPyThreadStateCurrentAddr,
	C.metricID_UnwindPythonErrZeroThreadState:             metrics.IDUnwindPythonErrZeroThreadState,
	C.metricID_UnwindPythonErrBadThreadStateFrameAddr:     metrics.IDUnwindPythonErrBadThreadStateFrameAddr,
	C.metricID_UnwindPythonZeroFrameCodeObject:            metrics.IDUnwindPythonZeroFrameCodeObject,
	C.metricID_UnwindPythonErrBadCodeObjectArgCountAddr:   metrics.IDUnwindPythonErrBadCodeObjectArgCountAddr,
	C.metricID_UnwindNativeErrStackDeltaInvalid:           metrics.IDUnwindNativeErrStackDeltaInvalid,
	C.metricID_ErrEmptyStack:                              metrics.IDErrEmptyStack,
	C.metricID_UnwindHotspotAttempts:                      metrics.IDUnwindHotspotAttempts,
	C.metricID_UnwindHotspotFrames:                        metrics.IDUnwindHotspotFrames,
	C.metricID_UnwindHotspotErrNoCodeblob:                 metrics.IDUnwindHotspotErrNoCodeblob,
	C.metricID_UnwindHotspotErrInvalidCodeblob:            metrics.IDUnwindHotspotErrInvalidCodeblob,
	C.metricID_UnwindHotspotErrInterpreterFP:              metrics.IDUnwindHotspotErrInterpreterFP,
	C.metricID_UnwindHotspotErrLrUnwindingMidTrace:        metrics.IDUnwindHotspotErrLrUnwindingMidTrace,
	C.metricID_UnwindHotspotUnsupportedFrameSize:          metrics.IDHotspotUnsupportedFrameSize,
	C.metricID_UnwindNativeSmallPC:                        metrics.IDUnwindNativeSmallPC,
	C.metricID_UnwindNativeErrLookupStackDeltaInnerMap:    metrics.IDUnwindNativeErrLookupStackDeltaInnerMap,
	C.metricID_UnwindNativeErrLookupStackDeltaOuterMap:    metrics.IDUnwindNativeErrLookupStackDeltaOuterMap,
	C.metricID_ErrBPFCurrentComm:                          metrics.IDErrBPFCurrentComm,
	C.metricID_UnwindPHPAttempts:                          metrics.IDUnwindPHPAttempts,
	C.metricID_UnwindPHPFrames:                            metrics.IDUnwindPHPFrames,
	C.metricID_UnwindPHPErrBadCurrentExecuteData:          metrics.IDUnwindPHPErrBadCurrentExecuteData,
	C.metricID_UnwindPHPErrBadZendExecuteData:             metrics.IDUnwindPHPErrBadZendExecuteData,
	C.metricID_UnwindPHPErrBadZendFunction:                metrics.IDUnwindPHPErrBadZendFunction,
	C.metricID_UnwindPHPErrBadZendOpline:                  metrics.IDUnwindPHPErrBadZendOpline,
	C.metricID_UnwindRubyAttempts:                         metrics.IDUnwindRubyAttempts,
	C.metricID_UnwindRubyFrames:                           metrics.IDUnwindRubyFrames,
	C.metricID_UnwindPerlAttempts:                         metrics.IDUnwindPerlAttempts,
	C.metricID_UnwindPerlFrames:                           metrics.IDUnwindPerlFrames,
	C.metricID_UnwindPerlTSD:                              metrics.IDUnwindPerlTLS,
	C.metricID_UnwindPerlReadStackInfo:                    metrics.IDUnwindPerlReadStackInfo,
	C.metricID_UnwindPerlReadContextStackEntry:            metrics.IDUnwindPerlReadContextStackEntry,
	C.metricID_UnwindPerlResolveEGV:                       metrics.IDUnwindPerlResolveEGV,
	C.metricID_UnwindHotspotErrInvalidRA:                  metrics.IDUnwindHotspotErrInvalidRA,
	C.metricID_UnwindV8Attempts:                           metrics.IDUnwindV8Attempts,
	C.metricID_UnwindV8Frames:                             metrics.IDUnwindV8Frames,
	C.metricID_UnwindV8ErrBadFP:                           metrics.IDUnwindV8ErrBadFP,
	C.metricID_UnwindV8ErrBadJSFunc:                       metrics.IDUnwindV8ErrBadJSFunc,
	C.metricID_UnwindV8ErrBadCode:                         metrics.IDUnwindV8ErrBadCode,
	C.metricID_ReportedPIDsErr:                            metrics.IDReportedPIDsErr,
	C.metricID_PIDEventsErr:                               metrics.IDPIDEventsErr,
	C.metricID_UnwindNativeLr0:                            metrics.IDUnwindNativeLr0,
	C.metricID_NumProcNew:                                 metrics.IDNumProcNew,
	C.metricID_NumProcExit:                                metrics.IDNumProcExit,
	C.metricID_NumUnknownPC:                               metrics.IDNumUnknownPC,
	C.metricID_NumGenericPID:                              metrics.IDNumGenericPID,
	C.metricID_UnwindPythonErrBadCFrameFrameAddr:          metrics.IDUnwindPythonErrBadCFrameFrameAddr,
	C.metricID_MaxTailCalls:                               metrics.IDMaxTailCalls,
	C.metricID_UnwindPythonErrNoProcInfo:                  metrics.IDUnwindPythonErrNoProcInfo,
	C.metricID_UnwindPythonErrBadAutoTlsKeyAddr:           metrics.IDUnwindPythonErrBadAutoTlsKeyAddr,
	C.metricID_UnwindPythonErrReadThreadStateAddr:         metrics.IDUnwindPythonErrReadThreadStateAddr,
	C.metricID_UnwindPythonErrReadTsdBase:                 metrics.IDUnwindPythonErrReadTsdBase,
	C.metricID_UnwindRubyErrNoProcInfo:                    metrics.IDUnwindRubyErrNoProcInfo,
	C.metricID_UnwindRubyErrReadStackPtr:                  metrics.IDUnwindRubyErrReadStackPtr,
	C.metricID_UnwindRubyErrReadStackSize:                 metrics.IDUnwindRubyErrReadStackSize,
	C.metricID_UnwindRubyErrReadCfp:                       metrics.IDUnwindRubyErrReadCfp,
	C.metricID_UnwindRubyErrReadEp:                        metrics.IDUnwindRubyErrReadEp,
	C.metricID_UnwindRubyErrReadIseqBody:                  metrics.IDUnwindRubyErrReadIseqBody,
	C.metricID_UnwindRubyErrReadIseqEncoded:               metrics.IDUnwindRubyErrReadIseqEncoded,
	C.metricID_UnwindRubyErrReadIseqSize:                  metrics.IDUnwindRubyErrReadIseqSize,
	C.metricID_UnwindNativeErrLrUnwindingMidTrace:         metrics.IDUnwindNativeErrLrUnwindingMidTrace,
	C.metricID_UnwindNativeErrReadKernelModeRegs:          metrics.IDUnwindNativeErrReadKernelModeRegs,
	C.metricID_UnwindNativeErrChaseIrqStackLink:           metrics.IDUnwindNativeErrChaseIrqStackLink,
	C.metricID_UnwindV8ErrNoProcInfo:                      metrics.IDUnwindV8ErrNoProcInfo,
	C.metricID_UnwindNativeErrBadUnwindInfoIndex:          metrics.IDUnwindNativeErrBadUnwindInfoIndex,
	C.metricID_UnwindApmIntErrReadTsdBase:                 metrics.IDUnwindApmIntErrReadTsdBase,
	C.metricID_UnwindApmIntErrReadCorrBufPtr:              metrics.IDUnwindApmIntErrReadCorrBufPtr,
	C.metricID_UnwindApmIntErrReadCorrBuf:                 metrics.IDUnwindApmIntErrReadCorrBuf,
	C.metricID_UnwindApmIntReadSuccesses:                  metrics.IDUnwindApmIntReadSuccesses,
	C.metricID_UnwindDotnetAttempts:                       metrics.IDUnwindDotnetAttempts,
	C.metricID_UnwindDotnetFrames:                         metrics.IDUnwindDotnetFrames,
	C.metricID_UnwindDotnetErrNoProcInfo:                  metrics.IDUnwindDotnetErrNoProcInfo,
	C.metricID_UnwindDotnetErrBadFP:                       metrics.IDUnwindDotnetErrBadFP,
	C.metricID_UnwindDotnetErrCodeHeader:                  metrics.IDUnwindDotnetErrCodeHeader,
	C.metricID_UnwindDotnetErrCodeTooLarge:                metrics.IDUnwindDotnetErrCodeTooLarge,
}
