// This file contains the code and map definitions for the V8 tracer
//
// Core unwinding of frames is simple, as all the generated code uses frame pointers,
// and all the interesting data is directly accessible via FP. The only additional
// task needed in EBPF code is to collect a Code* or SharedFunctionInfo* and potentially
// the current bytecode offset when in interpreted mode. Rest of the processing can
// be done from host agent.
//
// See the host agent interpreterv8.go for more references.

#include "v8_tracer.h"
#include "bpfdefs.h"
#include "tracemgmt.h"
#include "types.h"

#define v8Ver(x, y, z) (((x) << 24) + ((y) << 16) + (z))

// The number of V8 frames to unwind per frame-unwinding eBPF program.
#define V8_FRAMES_PER_PROGRAM 8

// The maximum V8 frame length used in heuristic to validate FP
#define V8_MAX_FRAME_LENGTH 8192

#if defined(__aarch64__)
  // On aarch64, a JS EntryFrame's layout differs from that of any other frame,
  // and stores 20 registers, the fp being the top-most.
  // See: https://chromium.googlesource.com/v8/v8/+/main/src/execution/arm64/frame-constants-arm64.h
  #define V8_ENTRYFRAME_CALLEE_SAVED_REGS_BEFORE_FP_LR_PAIR 18
#else
  #define V8_ENTRYFRAME_CALLEE_SAVED_REGS_BEFORE_FP_LR_PAIR 0
#endif

// Map from V8 process IDs to a structure containing addresses of variables
// we require in order to build the stack trace
struct v8_procs_t {
  __uint(type, BPF_MAP_TYPE_HASH);
  __type(key, pid_t);
  __type(value, V8ProcInfo);
  __uint(max_entries, 1024);
} v8_procs SEC(".maps");

// Record a V8 frame
static EBPF_INLINE ErrorCode push_v8(
  Trace *trace, unsigned long pointer_and_type, unsigned long delta_or_marker, bool return_address)
{
  DEBUG_PRINT(
    "Pushing v8 frame delta_or_marker=%lx, pointer_and_type=%lx",
    delta_or_marker,
    pointer_and_type);
  return _push_with_return_address(
    trace, pointer_and_type, delta_or_marker, FRAME_MARKER_V8, return_address);
}

// Verify a V8 tagged pointer
static EBPF_INLINE uintptr_t v8_verify_pointer(uintptr_t maybe_pointer)
{
  if ((maybe_pointer & V8_HeapObjectTagMask) != V8_HeapObjectTag) {
    return 0;
  }
  return maybe_pointer & ~V8_HeapObjectTagMask;
}

// Read and verify a V8 tagged pointer from given memory location.
static EBPF_INLINE uintptr_t v8_read_object_ptr(uintptr_t addr)
{
  uintptr_t maybe_pointer;
  if (bpf_probe_read_user(&maybe_pointer, sizeof(maybe_pointer), (void *)addr)) {
    return 0;
  }
  return v8_verify_pointer(maybe_pointer);
}

// Verify and parse a V8 SMI  ("SMall Integer") value.
// On 64-bit systems: SMI is the upper 32-bits of a 64-bit word, and the lowest bit is the tag.
// Returns the SMI value, or def_value in case of errors.
static EBPF_INLINE uintptr_t v8_parse_smi(uintptr_t maybe_smi, uintptr_t def_value)
{
  if ((maybe_smi & V8_SmiTagMask) != V8_SmiTag) {
    return def_value;
  }
  return maybe_smi >> V8_SmiValueShift;
}

// Read the type tag of a Heap Object at given memory location.
// Returns zero on error (valid object type IDs are non-zero).
static EBPF_INLINE u16 v8_read_object_type(V8ProcInfo *vi, uintptr_t addr)
{
  if (!addr) {
    return 0;
  }
  uintptr_t map = v8_read_object_ptr(addr + vi->off_HeapObject_map);
  u16 type;
  if (!map || bpf_probe_read_user(&type, sizeof(type), (void *)(map + vi->off_Map_instancetype))) {
    return 0;
  }
  return type;
}

// Unwind one V8 frame
static EBPF_INLINE ErrorCode unwind_one_v8_frame(PerCPURecord *record, V8ProcInfo *vi, bool top)
{
  UnwindState *state = &record->state;
  Trace *trace       = &record->trace;
  unsigned long sp = state->sp, fp = state->fp, pc = state->pc;
  V8UnwindScratchSpace *scratch = &record->v8UnwindScratch;

  // All V8 frames have frame pointer. Check that the FP looks valid.
  DEBUG_PRINT("v8: pc: %lx, sp: %lx, fp: %lx", pc, sp, fp);
  if (fp < sp || fp >= sp + V8_MAX_FRAME_LENGTH) {
    DEBUG_PRINT("v8: frame pointer too far off %lx / %lx", fp, sp);
    increment_metric(metricID_UnwindV8ErrBadFP);
    return ERR_V8_BAD_FP;
  }

  // Read FP pointer data
  if (bpf_probe_read_user(scratch->fp_ctx, V8_FP_CONTEXT_SIZE, (void *)(fp - V8_FP_CONTEXT_SIZE))) {
    DEBUG_PRINT("v8:  -> failed to read frame pointer context");
    increment_metric(metricID_UnwindV8ErrBadFP);
    return ERR_V8_BAD_FP;
  }

  // Make the verifier happy to access fpctx using the HA provided fp_* variables
  if (
    vi->fp_marker > V8_FP_CONTEXT_SIZE - sizeof(unsigned long) ||
    vi->fp_function > V8_FP_CONTEXT_SIZE - sizeof(unsigned long) ||
    vi->fp_bytecode_offset > V8_FP_CONTEXT_SIZE - sizeof(unsigned long)) {
    return ERR_UNREACHABLE;
  }
  unsigned long fp_marker          = *(unsigned long *)(scratch->fp_ctx + vi->fp_marker);
  unsigned long fp_function        = *(unsigned long *)(scratch->fp_ctx + vi->fp_function);
  unsigned long fp_bytecode_offset = *(unsigned long *)(scratch->fp_ctx + vi->fp_bytecode_offset);

  // Data that will be sent to HA is in these variables.
  uintptr_t pointer_and_type = 0, delta_or_marker = 0;

  // Before V8 5.8.261 the frame marker was a SMI. Now it has the tag, but it's not shifted fully.
  // The special coding was done to reduce the frame marker push <immed64> to <immed32>.
  if ((fp_marker & V8_SmiTagMask) == V8_SmiTag) {
    // Shift with the tag length only (shift on normal SMI is different).
    pointer_and_type = V8_FILE_TYPE_MARKER;
    delta_or_marker  = fp_marker >> V8_SmiTagShift;
    DEBUG_PRINT("v8:  -> stub frame, tag %ld", delta_or_marker);
    goto frame_done;
  }

  // Extract the JSFunction being executed
  uintptr_t jsfunc = v8_verify_pointer(fp_function);
  u16 jsfunc_tag   = v8_read_object_type(vi, jsfunc);
  if (jsfunc_tag < vi->type_JSFunction_first || jsfunc_tag > vi->type_JSFunction_last) {
    DEBUG_PRINT(
      "v8:  -> not a JSFunction: %x <= %x <= %x",
      vi->type_JSFunction_first,
      jsfunc_tag,
      vi->type_JSFunction_last);
    increment_metric(metricID_UnwindV8ErrBadJSFunc);
    return ERR_V8_BAD_JS_FUNC;
  }

  // Read the SFI to identify the function.
  uintptr_t sfi = v8_read_object_ptr(jsfunc + vi->off_JSFunction_shared);
  if (v8_read_object_type(vi, sfi) != vi->type_SharedFunctionInfo) {
    DEBUG_PRINT("v8:  -> no SharedFunctionInfo");
    increment_metric(metricID_UnwindV8ErrBadJSFunc);
    return ERR_V8_BAD_JS_FUNC;
  }

  // First determine if we are in interpreter mode. The simplest way to check
  // is if fp_bytecode_offset holds a SMI (the bytecode delta). The delta is
  // relative to the object pointer (not the actual bytecode data), so it is
  // always positive. In native mode, the same slot contains a Feedback Vector
  // tagged pointer.
  delta_or_marker = v8_parse_smi(fp_bytecode_offset, 0);
  if (delta_or_marker != 0) {
    DEBUG_PRINT("v8:  -> bytecode_delta %lx", delta_or_marker);
    pointer_and_type = V8_FILE_TYPE_BYTECODE | sfi;
    goto frame_done;
  }

  // Executing native code. At this point we can at least report the SFI if
  // other things fail.
  pointer_and_type = V8_FILE_TYPE_NATIVE_SFI | sfi;

  // Try to determine the Code object from JSFunction.
  uintptr_t code = v8_read_object_ptr(jsfunc + vi->off_JSFunction_code);
  u16 code_type  = v8_read_object_type(vi, code);
  if (code_type != vi->type_Code) {
    // If the object type tag does not match, it might be some new functionality
    // in the VM. Report the JSFunction for function name, but report no line
    // number information. This allows to get a complete trace even if this one
    // frame will have some missing information.
    DEBUG_PRINT("v8: jsfunc = %lx, code = %lx, code_type = %x", jsfunc, code, code_type);
    increment_metric(metricID_UnwindV8ErrBadCode);
    goto frame_done;
  }

  // Read the Code blob type and size
  if (bpf_probe_read_user(scratch->code, sizeof(scratch->code), (void *)code)) {
    increment_metric(metricID_UnwindV8ErrBadCode);
    goto frame_done;
  }
  // Make the verifier happy to access fpctx using the HA provided fp_* variables
  if (
    vi->off_Code_instruction_size > sizeof(scratch->code) - sizeof(u32) ||
    vi->off_Code_flags > sizeof(scratch->code) - sizeof(u32)) {
    return ERR_UNREACHABLE;
  }

  uintptr_t code_start;
  if (vi->version >= v8Ver(11, 1, 204)) {
    // Starting V8 11.1.204 the instruction/code start is a pointer field instead
    // of offset where the code starts.
    code_start = *(uintptr_t *)(scratch->code + vi->off_Code_instruction_start);
  } else {
    code_start = code + vi->off_Code_instruction_start;
  }
  u32 code_size  = *(u32 *)(scratch->code + vi->off_Code_instruction_size);
  u32 code_flags = *(u32 *)(scratch->code + vi->off_Code_flags);
  u8 code_kind   = (code_flags & vi->codekind_mask) >> vi->codekind_shift;

  uintptr_t code_end = code_start + code_size;
  DEBUG_PRINT("v8: func = %lx / sfi = %lx / code = %lx", jsfunc, sfi, code);
  DEBUG_PRINT("v8:  -> instructions: %lx..%lx (%d)", code_start, code_end, code_size);

  if (!(pc >= code_start && pc < code_end)) {
    // PC is not inside the Code object's code area. This can happen due to:
    // - on top frame when we are executing prologue/epilogue of called function,
    //   in this case we can try to recover original PC from the stack
    // - the JSFunction's Code object was changed due to On-Stack-Replacement or
    //   or other deoptimization reasons. This case is currently not handled.

    if (top && trace->stack_len == 0) {
      unsigned long stk[3];
      if (bpf_probe_read_user(stk, sizeof(stk), (void *)(sp - sizeof(stk)))) {
        DEBUG_PRINT("v8:  --> bad stack pointer");
        increment_metric(metricID_UnwindV8ErrBadFP);
        return ERR_V8_BAD_FP;
      }

      int i;
      UNROLL for (i = sizeof(stk) / sizeof(stk[0]) - 1; i >= 0; i--)
      {
        if (stk[i] >= code_start && stk[i] < code_end) {
          break;
        }
      }
      if (i < 0) {
        // Not able to recover PC.
        // TODO: investigate why this seems to happen occasionally
        DEBUG_PRINT("v8:  --> outside code blob: stack top %lx %lx %lx", stk[2], stk[1], stk[0]);
        goto frame_done;
      }

      // Recover the PC for the function which is in FP.
      pc = stk[i];
      DEBUG_PRINT("v8:  --> pc recovered from stack: %lx", pc);
    } else {
      DEBUG_PRINT("v8:  --> outside code blob (not topmost frame)");
      goto frame_done;
    }
  }

  // Code matches RIP, report it.
  if (code_kind == vi->codekind_baseline) {
    // Baseline Code does not have backpointer to SFI, so give the JSFunc.
    pointer_and_type = V8_FILE_TYPE_NATIVE_JSFUNC | jsfunc;
  } else {
    pointer_and_type = V8_FILE_TYPE_NATIVE_CODE | code;
  }

  // Use cookie that differentiates different types of Code objects
  u32 cookie      = (code_size << 4) | code_kind;
  delta_or_marker = (pc - code_start) | ((uintptr_t)cookie << V8_LINE_COOKIE_SHIFT);

frame_done:;
  ErrorCode error = push_v8(trace, pointer_and_type, delta_or_marker, state->return_address);
  if (error) {
    return error;
  }

  // Unwind with frame pointer
  if (!unwinder_unwind_frame_pointer(state)) {
    DEBUG_PRINT("v8:  --> bad frame pointer");
    increment_metric(metricID_UnwindV8ErrBadFP);
    return ERR_V8_BAD_FP;
  }

  // The JS Entry Frame's layout differs from other frames because some callee
  // saved registers might be pushed onto the stack before the [fp, lr] pair.
  // This frame is represented by markers 0 (inner) and 1 (outermost).
  // See: https://chromium.googlesource.com/v8/v8/+/main/src/execution/frames.h#167
  if (pointer_and_type == V8_FILE_TYPE_MARKER && delta_or_marker == 1)
    state->sp += V8_ENTRYFRAME_CALLEE_SAVED_REGS_BEFORE_FP_LR_PAIR * sizeof(size_t);

  DEBUG_PRINT(
    "v8: pc: %lx, sp: %lx, fp: %lx",
    (unsigned long)state->pc,
    (unsigned long)state->sp,
    (unsigned long)state->fp);

  increment_metric(metricID_UnwindV8Frames);
  return ERR_OK;
}

// unwind_v8 is the entry point for tracing when invoked from the native tracer
// or interpreter dispatcher. It does not reset the trace object and will append the
// V8 stack frames to the trace object for the current CPU.
static EBPF_INLINE int unwind_v8(struct pt_regs *ctx)
{
  PerCPURecord *record = get_per_cpu_record();
  if (!record) {
    return -1;
  }

  Trace *trace = &record->trace;
  u32 pid      = trace->pid;
  DEBUG_PRINT("==== unwind_v8 %d ====", trace->stack_len);

  int unwinder    = PROG_UNWIND_STOP;
  ErrorCode error = ERR_OK;
  V8ProcInfo *vi  = bpf_map_lookup_elem(&v8_procs, &pid);
  if (!vi) {
    DEBUG_PRINT("v8: no V8ProcInfo for this pid");
    error = ERR_V8_NO_PROC_INFO;
    increment_metric(metricID_UnwindV8ErrNoProcInfo);
    goto exit;
  }

  increment_metric(metricID_UnwindV8Attempts);

  UNROLL for (int i = 0; i < V8_FRAMES_PER_PROGRAM; i++)
  {
    unwinder = PROG_UNWIND_STOP;

    error = unwind_one_v8_frame(record, vi, i == 0);
    if (error) {
      break;
    }

    error = get_next_unwinder_after_native_frame(record, &unwinder);
    if (error || unwinder != PROG_UNWIND_V8) {
      break;
    }
  }

exit:
  record->state.unwind_error = error;
  tail_call(ctx, unwinder);
  DEBUG_PRINT("v8: tail call for next frame unwinder (%d) failed", unwinder);
  return -1;
}
MULTI_USE_FUNC(unwind_v8)
