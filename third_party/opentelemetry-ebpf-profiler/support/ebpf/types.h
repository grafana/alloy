// Provides type definitions shared by the eBPF and Go components

#ifndef OPTI_TYPES_H
#define OPTI_TYPES_H

#include "errors.h"
#include "kernel.h"

// ID values used as index to maps/metrics array.
// If you add enums below please update the following places too:
//  - The actual metric knob in:
//    metrics/metrics.json
//  - The mapping of this enum to the metric in Go:
//    support/types_def.go
enum {
  // number of calls to interpreter unwinding in get_next_interpreter()
  metricID_UnwindCallInterpreter = 0,

  // number of failures due to PC == 0 in unwind_next_frame()
  metricID_UnwindErrZeroPC,

  // number of times MAX_STACK_LEN has been exceeded
  metricID_UnwindErrStackLengthExceeded,

  // number of failures to read the TSD address
  metricID_UnwindErrBadTSDAddr,

  // number of failures to read the TSD base in get_tls_base()
  metricID_UnwindErrBadTPBaseAddr,

  // number of attempted unwinds
  metricID_UnwindNativeAttempts,

  // number of unwound frames
  metricID_UnwindNativeFrames,

  // number of native unwinds successfully ending with a stop delta
  metricID_UnwindNativeStackDeltaStop,

  // number of failures to look up ranges for text section in get_stack_delta()
  metricID_UnwindNativeErrLookupTextSection,

  // number of failed range searches within 20 steps in get_stack_delta()
  metricID_UnwindNativeErrLookupIterations,

  // number of failures to get StackUnwindInfo from stack delta map in get_stack_delta()
  metricID_UnwindNativeErrLookupRange,

  // number of kernel addresses passed to get_text_section()
  metricID_UnwindNativeErrKernelAddress,

  // number of failures to find the text section in get_text_section()
  metricID_UnwindNativeErrWrongTextSection,

  // number of invalid stack deltas in the native unwinder
  metricID_UnwindNativeErrStackDeltaInvalid,

  // number of failures to read PC from stack
  metricID_UnwindNativeErrPCRead,

  // number of attempted perl unwinds
  metricID_UnwindPerlAttempts,

  // number of perl frames unwound
  metricID_UnwindPerlFrames,

  // number of failures to read perl TSD info
  metricID_UnwindPerlTSD,

  // number of failures to read perl stack info
  metricID_UnwindPerlReadStackInfo,

  // number of failures to read perl context stack entry
  metricID_UnwindPerlReadContextStackEntry,

  // number of failures to resolve perl EGV
  metricID_UnwindPerlResolveEGV,

  // number of attempted python unwinds
  metricID_UnwindPythonAttempts,

  // number of unwound python frames
  metricID_UnwindPythonFrames,

  // number of failures to read from pyinfo->pyThreadStateCurrentAddr
  metricID_UnwindPythonErrBadPyThreadStateCurrentAddr,

  // number of PyThreadState being 0x0
  metricID_UnwindPythonErrZeroThreadState,

  // number of failures to read PyThreadState.frame in unwind_python()
  metricID_UnwindPythonErrBadThreadStateFrameAddr,

  // number of failures to read PyFrameObject->f_back in walk_python_stack()
  metricID_UnwindPythonErrBadFrameObjectBackAddr,

  // number of failures to read PyFrameObject->f_code in process_python_frame()
  metricID_UnwindPythonErrBadFrameCodeObjectAddr,

  // number of NULL code objects found in process_python_frame()
  metricID_UnwindPythonZeroFrameCodeObject,

  // number of failures to get the last instruction address in process_python_frame()
  metricID_UnwindPythonErrBadFrameLastInstructionAddr,

  // number of failures to get code object's argcount in process_python_frame()
  metricID_UnwindPythonErrBadCodeObjectArgCountAddr,

  // number of failures to get code object's kwonlyargcount in process_python_frame()
  metricID_UnwindPythonErrBadCodeObjectKWOnlyArgCountAddr,

  // number of failures to get code object's flags in process_python_frame()
  metricID_UnwindPythonErrBadCodeObjectFlagsAddr,

  // number of failures to get code object's first line number in process_python_frame()
  metricID_UnwindPythonErrBadCodeObjectFirstLineNumberAddr,

  // number of attempted PHP unwinds
  metricID_UnwindPHPAttempts,

  // number of unwound PHP frames
  metricID_UnwindPHPFrames,

  // number of failures to read PHP current execute data pointer
  metricID_UnwindPHPErrBadCurrentExecuteData,

  // number of failures to read PHP execute data contents
  metricID_UnwindPHPErrBadZendExecuteData,

  // number of failures to read PHP zend function contents
  metricID_UnwindPHPErrBadZendFunction,

  // number of failures to read PHP zend opline contents
  metricID_UnwindPHPErrBadZendOpline,

  // number of times unwind_stop is called without a trace
  metricID_ErrEmptyStack,

  // number of attempted Hotspot unwinds
  metricID_UnwindHotspotAttempts,

  // number of unwound Hotspot frames
  metricID_UnwindHotspotFrames,

  // number of failures to get codeblob address (no heap or bad segmap)
  metricID_UnwindHotspotErrNoCodeblob,

  // number of failures to get codeblob data or match it to current unwind state
  metricID_UnwindHotspotErrInvalidCodeblob,

  // number of failures to unwind interpreter due to invalid FP
  metricID_UnwindHotspotErrInterpreterFP,

  // number of failures to unwind because return address was not found with heuristic
  metricID_UnwindHotspotErrInvalidRA,

  // number of times the unwind instructions requested LR unwinding mid-trace
  metricID_UnwindHotspotErrLrUnwindingMidTrace,

  // number of times we encountered frame sizes larger than the supported maximum
  metricID_UnwindHotspotUnsupportedFrameSize,

  // number of times that PC hold a value smaller than 0x1000
  metricID_UnwindNativeSmallPC,

  // number of times that a lookup of a inner map for stack deltas failed
  metricID_UnwindNativeErrLookupStackDeltaInnerMap,

  // number of times that a lookup of the outer map for stack deltas failed
  metricID_UnwindNativeErrLookupStackDeltaOuterMap,

  // number of times the bpf helper failed to get the current comm of the task
  metricID_ErrBPFCurrentComm,

  // number of attempted Ruby unwinds
  metricID_UnwindRubyAttempts,

  // number of unwound Ruby frames
  metricID_UnwindRubyFrames,

  // number of attempted V8 unwinds
  metricID_UnwindV8Attempts,

  // number of unwound V8 frames
  metricID_UnwindV8Frames,

  // number of failures to read V8 frame pointer data
  metricID_UnwindV8ErrBadFP,

  // number of failures to read V8 JSFunction object
  metricID_UnwindV8ErrBadJSFunc,

  // number of failures to read V8 Code object
  metricID_UnwindV8ErrBadCode,

  // number of times frame unwinding failed because of LR == 0
  metricID_UnwindNativeLr0,

  // number of times we failed to update maps/reported_pids
  metricID_ReportedPIDsErr,

  // number of times we failed to update maps/pid_events
  metricID_PIDEventsErr,

  // number of "process new" PIDs written to maps/pid_events
  metricID_NumProcNew,

  // number of "process exit" PIDs written to maps/pid_events
  metricID_NumProcExit,

  // number of "unknown PC" PIDs written to maps/pid_events
  metricID_NumUnknownPC,

  // number of GENERIC_PID event sent to user space (perf_event)
  metricID_NumGenericPID,

  // number of failures to read _PyCFrame.current_frame in unwind_python()
  metricID_UnwindPythonErrBadCFrameFrameAddr,

  // number of times stack unwinding was stopped to not exceed the limit of tail calls
  metricID_MaxTailCalls,

  // number of times we didn't find an entry for this process in the Python process info array
  metricID_UnwindPythonErrNoProcInfo,

  // number of failures to read autoTLSkey
  metricID_UnwindPythonErrBadAutoTlsKeyAddr,

  // number of failures to read the thread state pointer from TLD
  metricID_UnwindPythonErrReadThreadStateAddr,

  // number of failures to determine the base address for thread-specific data
  metricID_UnwindPythonErrReadTsdBase,

  // number of times no entry for a process existed in the Ruby process info array
  metricID_UnwindRubyErrNoProcInfo,

  // number of failures to read the stack pointer from the Ruby context
  metricID_UnwindRubyErrReadStackPtr,

  // number of failures to read the size of the VM stack from the Ruby context
  metricID_UnwindRubyErrReadStackSize,

  // number of failures to read the control frame pointer from the Ruby context
  metricID_UnwindRubyErrReadCfp,

  // number of failures to read the expression path from the Ruby frame
  metricID_UnwindRubyErrReadEp,

  // number of failures to read the instruction sequence body
  metricID_UnwindRubyErrReadIseqBody,

  // number of failures to read the instruction sequence encoded size
  metricID_UnwindRubyErrReadIseqEncoded,

  // number of failures to read the instruction sequence size
  metricID_UnwindRubyErrReadIseqSize,

  // number of times the unwind instructions requested LR unwinding mid-trace
  metricID_UnwindNativeErrLrUnwindingMidTrace,

  // number of failures to read the kernel-mode registers
  metricID_UnwindNativeErrReadKernelModeRegs,

  // number of failures to read the IRQ stack link
  metricID_UnwindNativeErrChaseIrqStackLink,

  // number of times no entry for a process exists in the V8 process info array
  metricID_UnwindV8ErrNoProcInfo,

  // number of times an unwind_info_array index was invalid
  metricID_UnwindNativeErrBadUnwindInfoIndex,

  // number of failures to get TSD base for APM correlation
  metricID_UnwindApmIntErrReadTsdBase,

  // number of failures read the APM correlation pointer
  metricID_UnwindApmIntErrReadCorrBufPtr,

  // number of failures read the APM correlation buffer
  metricID_UnwindApmIntErrReadCorrBuf,

  // number of successful reads of APM correlation info
  metricID_UnwindApmIntReadSuccesses,

  // number of attempted Dotnet unwinds
  metricID_UnwindDotnetAttempts,

  // number of unwound Dotnet frames
  metricID_UnwindDotnetFrames,

  // number of times no entry for a process exists in the Dotnet process info array
  metricID_UnwindDotnetErrNoProcInfo,

  // number of failures to read Dotnet frame pointer data
  metricID_UnwindDotnetErrBadFP,

  // number of failures to read Dotnet CodeHeader object
  metricID_UnwindDotnetErrCodeHeader,

  // number of failures to unwind code object due to its large size
  metricID_UnwindDotnetErrCodeTooLarge,

  // number of attempts to read Go custom labels
  metricID_UnwindGoLabelsAttempts,

  // number of failures to read Go custom labels
  metricID_UnwindGoLabelsFailures,

  //
  // Metric IDs above are for counters (cumulative values)
  //

  metricID_BeginCumulative,

  //
  // Metric IDs below are for gauges (instantaneous values)
  //

  // used as size for maps/metrics (BPF_MAP_TYPE_PERCPU_ARRAY)
  metricID_Max
};

// TracePrograms provide the offset for each eBPF trace program in the
// map that holds them.
// The values of this enum must fit in a single byte.
typedef enum TracePrograms {
  PROG_UNWIND_STOP,
  PROG_UNWIND_NATIVE,
  PROG_UNWIND_HOTSPOT,
  PROG_UNWIND_PERL,
  PROG_UNWIND_PYTHON,
  PROG_UNWIND_PHP,
  PROG_UNWIND_RUBY,
  PROG_UNWIND_V8,
  PROG_UNWIND_DOTNET,
  PROG_GO_LABELS,
  NUM_TRACER_PROGS,
} TracePrograms;

// TraceOrigin describes the source of the trace. This enables
// origin specific handling of traces in user space.
typedef enum TraceOrigin {
  TRACE_UNKNOWN,
  TRACE_SAMPLING,
  TRACE_OFF_CPU,
  TRACE_UPROBE,
} TraceOrigin;

// MAX_FRAME_UNWINDS defines the maximum number of frames per
// Trace we can unwind and respect the limit of eBPF instructions,
// limit of tail calls and limit of stack size per eBPF program.
#define MAX_FRAME_UNWINDS 256

// MAX_NON_ERROR_FRAME_UNWINDS defines the maximum number of frames
// to be pushed by unwinders while still leaving space for an error frame.
// This is used to make sure that there is always space for an error
// frame reporting that we ran out of stack space.
#define MAX_NON_ERROR_FRAME_UNWINDS (MAX_FRAME_UNWINDS - 1)

// Maximum number of unique stack deltas needed on a system. This is based on
// normal desktop /usr/bin/* and /usr/lib/*.so having about 9700 unique deltas.
// Can be increased up to 2^15, see also STACK_DELTA_COMMAND_FLAG.
#define UNWIND_INFO_MAX_ENTRIES 16384

// Type to represent a globally-unique file id to be used as key for a BPF hash map
typedef u64 FileID;

// Individual frame in a stack-trace.
typedef struct Frame {
  // IDs that uniquely identify a file combination
  FileID file_id;
  // For PHP this is the line numbers, corresponding to the files in `stack`.
  // For Python, each value provides information to allow for the recovery of
  // the line number associated with its corresponding offset in `stack`.
  // The lower 32 bits provide the co_firstlineno value and the upper 32 bits
  // provide the f_lasti value. Other interpreter handlers use the field in
  // a similarly domain-specific fashion.
  u64 addr_or_line;
  // Indicates the type of the frame (Python, PHP, native etc.).
  u8 kind;
  // Indicates that the address is a return address.
  u8 return_address;
  // Explicit padding bytes that the compiler would have inserted anyway.
  // Here to make it clear to readers that there are spare bytes that could
  // be put to work without extra cost in case an interpreter needs it.
  u8 pad[6];
} Frame;

_Static_assert(sizeof(Frame) == 3 * 8, "frame padding not working as expected");

// TSDInfo contains data needed to extract Thread Specific Data (TSD) values
typedef struct TSDInfo {
  s16 offset;
  u8 multiplier;
  u8 indirect;
} TSDInfo;

// DotnetProcInfo is a container for the data needed to build stack trace for a dotnet process.
typedef struct DotnetProcInfo {
  u32 version;
} DotnetProcInfo;

// PerlProcInfo is a container for the data needed to build a stack trace for a Perl process.
typedef struct PerlProcInfo {
  u64 stateAddr;
  u32 version;
  TSDInfo tsdInfo;
  // Introspection data
  u16 interpreter_curcop, interpreter_curstackinfo;
  u8 stateInTSD, si_cxstack, si_next, si_cxix, si_type;
  u8 context_type, context_blk_oldcop, context_blk_sub_retop, context_blk_sub_cv, context_sizeof;
  u8 sv_flags, sv_any, svu_gp, xcv_flags, xcv_gv, gp_egv;
} PerlProcInfo;

// PyProcInfo is a container for the data needed to build a stack trace for a Python process.
typedef struct PyProcInfo {
  // The address of the autoTLSkey variable
  u64 autoTLSKeyAddr;
  u64 noneStructAddr;
  u16 version;
  TSDInfo tsdInfo;
  // The Python object member offsets
  u8 PyThreadState_frame;
  u8 PyCFrame_current_frame;
  u8 PyFrameObject_f_back, PyFrameObject_f_code, PyFrameObject_f_lasti;
  u8 PyFrameObject_entry_member, PyFrameObject_entry_val;
  u8 PyCodeObject_co_argcount, PyCodeObject_co_kwonlyargcount;
  u8 PyCodeObject_co_flags, PyCodeObject_co_firstlineno;
  u8 PyCodeObject_sizeof;
  u8 continue_with_next_unwinder;
} PyProcInfo;

// PHPProcInfo is a container for the data needed to build a stack trace for a PHP process.
typedef struct PHPProcInfo {
  u64 current_execute_data;
  // Return address for JIT code (in Hybrid mode)
  u64 jit_return_address;
  // Offsets for structures we need to access in ebpf
  u8 zend_execute_data_function, zend_execute_data_opline, zend_execute_data_prev_execute_data;
  u8 zend_execute_data_this_type_info, zend_function_type, zend_op_lineno;
} PHPProcInfo;

// HotspotProcInfo is a container for the data needed to build a stack trace
// for a Java Hotspot VM process.
typedef struct HotspotProcInfo {
  // The global JIT heap mapping. All JIT code is between these two address.
  u64 codecache_start, codecache_end;

  // Offsets of large structures, sizeof it is near or over 256 bytes.
  u16 nmethod_deopt_offset, nmethod_compileid, nmethod_orig_pc_offset;

  // Offsets and other data fitting in a uchar
  u8 codeblob_name;
  u8 codeblob_codestart, codeblob_codeend;
  u8 codeblob_framecomplete, codeblob_framesize;
  u8 heapblock_size, method_constmethod, cmethod_size;
  u8 jvm_version, segment_shift, nmethod_uses_offsets;
} HotspotProcInfo;

// RubyProcInfo is a container for the data needed to build a stack trace for a Ruby process.
typedef struct RubyProcInfo {
  // version of the Ruby interpreter.
  u32 version;

  // current_ctx_ptr holds the address of the symbol ruby_current_execution_context_ptr.
  u64 current_ctx_ptr;

  // Offsets and sizes of Ruby internal structs

  // rb_execution_context_struct offsets:
  u8 vm_stack, vm_stack_size, cfp;

  // rb_control_frame_struct offsets:
  u8 pc, iseq, ep, size_of_control_frame_struct;

  // rb_iseq_struct offsets:
  u8 body;

  // rb_iseq_constant_body:
  u8 iseq_type, iseq_encoded, iseq_size;

  // size_of_value holds the size of the macro VALUE as defined in
  // https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm_core.h#L1136
  u8 size_of_value;

  // rb_ractor_struct offset:
  u16 running_ec;

} RubyProcInfo;

// V8ProcInfo is a container for the data needed to build a stack trace for a V8 process.
typedef struct V8ProcInfo {
  u32 version;
  // Introspection data
  u16 type_JSFunction_first, type_JSFunction_last, type_Code, type_SharedFunctionInfo;
  u8 off_HeapObject_map, off_Map_instancetype, off_JSFunction_code, off_JSFunction_shared;
  u8 off_Code_instruction_start, off_Code_instruction_size, off_Code_flags;
  u8 fp_marker, fp_function, fp_bytecode_offset;
  u8 codekind_shift, codekind_mask, codekind_baseline;
} V8ProcInfo;

// COMM_LEN defines the maximum length we will receive for the comm of a task.
#define COMM_LEN 16

// 128-bit APM trace ID.
typedef union ApmTraceID {
  u8 raw[16];
  struct {
    u64 lo;
    u64 hi;
  } as_int;
} ApmTraceID;

_Static_assert(sizeof(ApmTraceID) == 16, "unexpected trace ID size");

// 64-bit APM transaction / span ID.
typedef union ApmSpanID {
  u8 raw[8];
  u64 as_int;
} ApmSpanID;

_Static_assert(sizeof(ApmSpanID) == 8, "unexpected trace ID size");

// Defines the format of the APM correlation TLS buffer.
//
// Specification:
// https://github.com/elastic/apm/blob/bd5fa9c1/specs/agents/universal-profiling-integration.md#thread-local-storage-layout
typedef struct __attribute__((packed)) ApmCorrelationBuf {
  u16 layout_minor_ver;
  u8 valid;
  u8 trace_present;
  u8 trace_flags;
  ApmTraceID trace_id;
  ApmSpanID span_id;
  ApmSpanID transaction_id;
} ApmCorrelationBuf;

#define CUSTOM_LABEL_MAX_KEY_LEN COMM_LEN
// Big enough to hold UUIDs, etc.
#define CUSTOM_LABEL_MAX_VAL_LEN 48

typedef struct CustomLabel {
  u8 key[CUSTOM_LABEL_MAX_KEY_LEN];
  u8 val[CUSTOM_LABEL_MAX_VAL_LEN];
} CustomLabel;

#define MAX_CUSTOM_LABELS 10

typedef struct CustomLabelsArray {
  unsigned len;
  CustomLabel labels[MAX_CUSTOM_LABELS];
} CustomLabelsArray;

// Container for a stack trace
typedef struct Trace {
  // The process ID
  // NOTE: Confusingly, this is what Linux calls "tgid"
  u32 pid;
  // The thread ID
  // NOTE: Confusingly, this is what Linux calls "pid".
  u32 tid;
  // Monotonic kernel time in nanosecond precision.
  u64 ktime;
  // The current COMM of the thread of this Trace.
  u8 comm[COMM_LEN];
  // APM transaction ID or all-zero if not present.
  ApmSpanID apm_transaction_id;
  // APM trace ID or all-zero if not present.
  ApmTraceID apm_trace_id;
  // Custom Labels
  CustomLabelsArray custom_labels;
  // The kernel stack ID.
  s32 kernel_stack_id;
  // The number of frames in the stack.
  u32 stack_len;

  // origin indicates the source of the trace.
  TraceOrigin origin;

  // offtime stores the nanoseconds that the trace was off-cpu for.
  u64 offtime;

  // The frames of the stack trace.
  Frame frames[MAX_FRAME_UNWINDS];

  // NOTE: both send_trace in BPF and loadBpfTrace in UM code require `frames`
  // to be the last item in the struct. Do not add new members here without also
  // adjusting the UM code.
} Trace;

// Container for unwinding state
typedef struct UnwindState {
  // Current register value for Program Counter
  u64 pc;
  // Current register value for Stack Pointer
  u64 sp;
  // Current register value for Frame Pointer
  u64 fp;

#if defined(__x86_64__)
  // Current register values for named registers
  u64 rax, r9, r11, r13, r15;
#elif defined(__aarch64__)
  // Current register values for named registers
  u64 lr, r22, r28;
#endif

  // The executable ID/hash associated with PC
  u64 text_section_id;
  // PC converted into the offset relative to the executables text section
  u64 text_section_offset;
  // The current mapping load bias
  u64 text_section_bias;

  // Unwind error condition to process and report in unwind_stop()
  s32 error_metric;
  // If unwinding was aborted due to an error, this contains the reason why.
  ErrorCode unwind_error;

  // Set if the PC is a return address. That is, it points to the next instruction
  // after a CALL instruction, and requires to be adjusted during symbolization.
  //
  // Consider calling unwinder_mark_nonleaf_frame rather than setting this directly.
  bool return_address;

#if defined(__aarch64__)
  // On aarch64, whether to forbid LR-based unwinding.
  // LR unwinding is only allowed for leaf user-mode frames. Frames making a syscall
  // are also considered leaf frames for this purpose, because LR is preserved across
  // syscalls.
  //
  // Consider calling unwinder_mark_nonleaf_frame rather than setting this directly.
  bool lr_invalid;
#endif
} UnwindState;

// Container for unwinding state needed by the Perl unwinder. Keeping track of
// current stackinfo, first seen COP, and the info about current context stack.
typedef struct PerlUnwindState {
  // Pointer to the next stackinfo to unwind
  const void *stackinfo;
  // First Control OP seen for the frame filename/linenumber info for next function frame
  const void *cop;
  // Current context state, pointer to the base and current entries
  const void *cxbase, *cxcur;
} PerlUnwindState;

// Container for unwinding state needed by the Python unwinder. At the moment
// the only thing we need to pass between invocations of the unwinding programs
// is the pointer to the next PyFrameObject to unwind.
typedef struct PythonUnwindState {
  // Pointer to the next PyFrameObject to unwind
  void *py_frame;
} PythonUnwindState;

// Container for unwinding state needed by the PHP unwinder. At the moment
// the only thing we need to pass between invocations of the unwinding programs
// is the pointer to the next zend_execute_data to unwind.
typedef struct PHPUnwindState {
  // Pointer to the next zend_execute_data to unwind
  const void *zend_execute_data;
} PHPUnwindState;

// Container for unwinding state needed by the Ruby unwinder.
typedef struct RubyUnwindState {
  // Pointer to the next control frame struct in the Ruby VM stack we want to unwind.
  void *stack_ptr;
  // Pointer to the last control frame struct in the Ruby VM stack we want to handle.
  void *last_stack_frame;
} RubyUnwindState;

// Container for additional scratch space needed by the HotSpot unwinder.
typedef struct DotnetUnwindScratchSpace {
  // Buffer to read nibble map to locate code start. One map entry allows seeking backwards
  // 32*8 = 256 bytes of code. This defines the maximum size for a JITted function we
  // can recognize: 256 bytes/element * 128 elements = 32kB function size.
  // Multiplied by two for extra space to read to this array a fixed amount of bytes
  // to a dynamic offset.
  union {
    u32 map[2 * 128];
    u64 map64[128];
  };
} DotnetUnwindScratchSpace;

// Container for additional scratch space needed by the HotSpot unwinder.
typedef struct HotspotUnwindScratchSpace {
  // Read buffer for storing the codeblob. It's not needed across calls, but the buffer is too
  // large to be allocated on stack. With my debug build of JDK17, the largest possible variant of
  // codeblob that we care about (nmethod) is 376 bytes in size. 512 bytes should thus be plenty.
  u8 codeblob[512];
} HotspotUnwindScratchSpace;

// The number of bytes read from frame pointer for V8 context
#define V8_FP_CONTEXT_SIZE 64

// Container for additional scratch space needed by the V8 unwinder.
typedef struct V8UnwindScratchSpace {
  // Read buffer for storing the V8 FP stored context. Needs to be in non-stack
  // area to allow variable indexing.
  u8 fp_ctx[V8_FP_CONTEXT_SIZE];
  // Read buffer for V8 Code object. Currently we need about 60 bytes to get
  // code instruction_size and flags.
  u8 code[96];
} V8UnwindScratchSpace;

// Container for additional scratch space needed by the Python unwinder.
typedef struct PythonUnwindScratchSpace {
  // Read buffer for storing the PyInterpreterFrame (PyFrameObject).
  // Python 3.11 is about 80 bytes, but Python 3.7 has larger requirement.
  u8 frame[128];
  // Read buffer for storing the PyCodeObject. Currently we need 148 bytes of the header. But
  // the structure is 192 bytes in Python 3.11.
  u8 code[192];
} PythonUnwindScratchSpace;

// https://github.com/golang/go/blob/6885bad7dd/src/cmd/compile/internal/types/size.go#L28
struct GoString {
  char *str;
  u64 len;
};

// https://github.com/golang/go/blob/6885bad7dd/src/cmd/compile/internal/types/size.go#L20
struct GoSlice {
  void *array;
  u64 len;
  s64 cap;
};

// https://github.com/golang/go/blob/6885bad7dd/src/runtime/map.go#L109
typedef struct GoMapBucket {
  char tophash[8];
  struct GoString keys[8];
  struct GoString values[8];
  void *overflow;
} GoMapBucket;

typedef struct CustomLabelsState {
  void *go_m_ptr;
} CustomLabelsState;

// Per-CPU info for the stack being built. This contains the stack as well as
// meta-data on the number of eBPF tail-calls used so far to construct it.
typedef struct PerCPURecord {
  // The output record, including the stack being built.
  Trace trace;
  // The current unwind state.
  UnwindState state;
  // The current Perl unwinder state
  PerlUnwindState perlUnwindState;
  // The current Python unwinder state.
  PythonUnwindState pythonUnwindState;
  // The current PHP unwinder state.
  PHPUnwindState phpUnwindState;
  // The current Ruby unwinder state.
  RubyUnwindState rubyUnwindState;
  // State for Go and Native custom labels
  CustomLabelsState customLabelsState;
  union {
    // Scratch space for the Dotnet unwinder.
    DotnetUnwindScratchSpace dotnetUnwindScratch;
    // Scratch space for the HotSpot unwinder.
    HotspotUnwindScratchSpace hotspotUnwindScratch;
    // Scratch space for the V8 unwinder
    V8UnwindScratchSpace v8UnwindScratch;
    // Scratch space for the Python unwinder
    PythonUnwindScratchSpace pythonUnwindScratch;
    // Go labels scratch
    GoMapBucket goMapBucket;
    // Scratch for Go 1.24 labels
    struct GoString labels[MAX_CUSTOM_LABELS * 2];
  };
  // Mask to indicate which unwinders are complete
  u32 unwindersDone;

  // tailCalls tracks the number of calls to bpf_tail_call().
  u8 tailCalls;

  // ratelimitAction determines the PID event rate limiting mode
  u8 ratelimitAction;
} PerCPURecord;

// UnwindInfo contains the unwind information needed to unwind one frame
// from a specific address.
typedef struct UnwindInfo {
  u8 opcode;      // main opcode to unwind CFA
  u8 fpOpcode;    // opcode to unwind FP
  u8 mergeOpcode; // opcode for generating next stack delta, see below
  s32 param;      // parameter for the CFA expression
  s32 fpParam;    // parameter for the FP expression
} UnwindInfo;

// The 8-bit mergeOpcode consists of two separate fields:
//  1 bit   the adjustment to 'param' is negative (-8), if not set positive (+8)
//  7 bits  the difference to next 'addrLow'
#define MERGEOPCODE_NEGATIVE 0x80

// An array entry that we will bsearch into that keeps address and stack unwind
// info, per executable.
typedef struct StackDelta {
  u16 addrLow;    // the low 16-bits of the ELF virtual address to which this stack delta applies
  u16 unwindInfo; // index of UnwindInfo, or UNWIND_COMMAND_* if STACK_DELTA_COMMAND_FLAG is set
} StackDelta;

// unwindInfo flag indicating that the value is UNWIND_COMMAND_* value and not an index to
// the unwind info array. When UnwindInfo.opcode is UNWIND_OPCODE_COMMAND the 'param' gives
// the UNWIND_COMMAND_* which describes the exact handling for this stack delta (all
// CFA/PC/FP recovery, or stop condition), and the eBPF code needs special code to handle it.
// This basically serves as a minor optimization to not take a slot from unwind info array,
// nor require a table lookup for these special cased stack deltas.
#define STACK_DELTA_COMMAND_FLAG 0x8000

// StackDeltaPageKey is the look up key for stack delta page map.
typedef struct StackDeltaPageKey {
  u64 fileID;
  u64 page;
} StackDeltaPageKey;

// StackDeltaPageInfo contains information of stack delta page so the correct map
// and range of StackDelta entries can be found.
typedef struct StackDeltaPageInfo {
  u32 firstDelta;
  u16 numDeltas;
  u16 mapID;
} StackDeltaPageInfo;

// Keep stack deltas in 64kB pages to limit search space and to fit the low address
// bits into the addrLow field of struct StackDelta.
#define STACK_DELTA_PAGE_BITS 16

// The binary mask for STACK_DELTA_PAGE_BITS, which can be used to and/nand an address
// for its page number and offset within that page.
#define STACK_DELTA_PAGE_MASK ((1 << STACK_DELTA_PAGE_BITS) - 1)

// In order to determine whether a given PC falls into the main interpreter loop
// of an interpreter, we need to store some data: The lower boundary of the loop,
// the upper boundary of the loop, and the relevant index to call in the prog
// array.
typedef struct OffsetRange {
  u64 lower_offset1;
  u64 upper_offset1;
  // Fields {lower,upper}_offset2 may be used to specify an optional second range
  // of an interpreter function. This may be useful if the interpreter function
  // consists of two non-contiguous memory ranges, which may happen due to Hot/Cold
  // split compiler optimization
  u64 lower_offset2;
  u64 upper_offset2;
  u16 program_index; // The interpreter-specific program index to call.
} OffsetRange;

// SystemAnalysis is the structure in system_analysis map
typedef struct SystemAnalysis {
  u64 address;
  u32 pid;
  u8 code[128];
} SystemAnalysis;

// Event is the header for all events sent through the report_events
// perf event output channel (event_send_trigger).
typedef struct Event {
  u32 event_type; // EVENT_TYPE_xxx selector of event
} Event;

// Event types that notifications are sent for through event_send_trigger.
#define EVENT_TYPE_GENERIC_PID 1

// PIDPage represents the key of the eBPF map pid_page_to_mapping_info.
typedef struct PIDPage {
  u32 prefixLen; // Number of bits for pid and page that defines the
                 // longest prefix.

  __be32 pid;  // Unique ID of the process.
  __be64 page; // Address to a certain part of memory within PID.
} PIDPage;

// BIT_WIDTH_PID defines the number of bits used in the value pid of the PIDPage struct.
#define BIT_WIDTH_PID  32
// BIT_WIDTH_PAGE defines the number of bits used in the value page of the PIDPage struct.
#define BIT_WIDTH_PAGE 64

// Constants for accessing bitfields within HotSpot text_section_offset/file_id.
#define HS_TSID_IS_STUB_BIT       63
#define HS_TSID_HAS_FRAME_BIT     62
#define HS_TSID_STACK_DELTA_BIT   56
#define HS_TSID_STACK_DELTA_MASK  ((1UL << 6) - 1)
#define HS_TSID_STACK_DELTA_SCALE 8
#define HS_TSID_SEG_MAP_BIT       0
#define HS_TSID_SEG_MAP_MASK      ((1UL << 56) - 1)

// PIDPageMappingInfo represents the value of the eBPF map pid_page_to_mapping_info.
typedef struct PIDPageMappingInfo {
  u64 file_id; // Unique identifier for the executable file

  // Load bias (7 bytes) + unwinding program to use (1 byte, shifted 7 bytes to the left), encoded
  // in a u64. We can do so because the load bias is for userspace addresses, for which the most
  // significant byte is always 0 on relevant architectures. This encoding may have to be changed if
  // bias can be negative.
  u64 bias_and_unwind_program;
} PIDPageMappingInfo;

// UNKNOWN_FILE indicates for unknown files.
#define UNKNOWN_FILE      0x0
// FUNC_TYPE_UNKNOWN indicates an unknown interpreted function.
#define FUNC_TYPE_UNKNOWN 0xfffffffffffffffe

// Smallest stack delta bucket that holds up to 2^8 entries
#define STACK_DELTA_BUCKET_SMALLEST 8
// Largest stack delta bucket that holds up to 2^23 entries
#define STACK_DELTA_BUCKET_LARGEST  23

// Avoid including all of arch/arm64/include/uapi/asm/ptrace.h by copying the
// actually used values.
#define PSR_MODE32_BIT 0x00000010
#define PSR_MODE_MASK  0x0000000f
#define PSR_MODE_EL0t  0x00000000

typedef struct ApmIntProcInfo {
  u64 tls_offset;
} ApmIntProcInfo;

typedef struct GoLabelsOffsets {
  u32 m_offset;
  u32 curg;
  u32 labels;
  u32 hmap_count;
  u32 hmap_log2_bucket_count;
  u32 hmap_buckets;
  s32 tls_offset;
} GoLabelsOffsets;

#endif // OPTI_TYPES_H
