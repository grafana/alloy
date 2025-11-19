// This file contains the code and map definitions for the Ruby tracer

#include "bpfdefs.h"
#include "tracemgmt.h"
#include "types.h"

// Map from Ruby process IDs to a structure containing addresses of variables
// we require in order to build the stack trace
struct ruby_procs_t {
  __uint(type, BPF_MAP_TYPE_HASH);
  __type(key, pid_t);
  __type(value, RubyProcInfo);
  __uint(max_entries, 1024);
} ruby_procs SEC(".maps");

// The number of Ruby frames to unwind per frame-unwinding eBPF program. If
// we start running out of instructions in the walk_ruby_stack program, one
// option is to adjust this number downwards.
#define FRAMES_PER_WALK_RUBY_STACK 27

// Ruby VM frame flags are internal indicators for the VM interpreter to
// treat frames in a dedicated way.
// https://github.com/ruby/ruby/blob/5741ae379b2037ad5968b6994309e1d25cda6e1a/vm_core.h#L1208
#define RUBY_FRAME_FLAG_BMETHOD 0x0040
#define RUBY_FRAME_FLAG_LAMBDA  0x0100

// Record a Ruby frame
static EBPF_INLINE ErrorCode push_ruby(Trace *trace, u64 file, u64 line)
{
  return _push(trace, file, line, FRAME_MARKER_RUBY);
}

// walk_ruby_stack processes a Ruby VM stack, extracts information from the individual frames and
// pushes this information to user space for symbolization of these frames.
//
// Ruby unwinder workflow:
// From the current execution context struct [0] we can get pointers to the current Ruby VM stack
// as well as to the current call frame pointer (cfp).
// On the Ruby VM stack we have for each cfp one struct [1]. These cfp structs then point to
// instruction sequence (iseq) structs [2] that store the information about file and function name
// that we forward to user space for the symbolization process of the frame.
//
//
// [0] rb_execution_context_struct
// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm_core.h#L843
//
// [1] rb_control_frame_struct
// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm_core.h#L760
//
// [2] rb_iseq_struct
// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm_core.h#L456
static EBPF_INLINE ErrorCode walk_ruby_stack(
  PerCPURecord *record,
  const RubyProcInfo *rubyinfo,
  const void *current_ctx_addr,
  int *next_unwinder)
{
  if (!current_ctx_addr) {
    *next_unwinder = get_next_unwinder_after_interpreter();
    return ERR_OK;
  }

  Trace *trace = &record->trace;

  *next_unwinder = PROG_UNWIND_STOP;

  // stack_ptr points to the frame of the Ruby VM call stack that will be unwound next
  void *stack_ptr        = record->rubyUnwindState.stack_ptr;
  // last_stack_frame points to the last frame on the Ruby VM stack we want to process
  void *last_stack_frame = record->rubyUnwindState.last_stack_frame;

  if (!stack_ptr || !last_stack_frame) {
    // stack_ptr_current points to the current frame in the Ruby VM call stack
    void *stack_ptr_current;
    // stack_size does not reflect the number of frames on the Ruby VM stack
    // but contains the current stack size in words.
    // stack_size = size in word (size in bytes / sizeof(VALUE))
    // https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm_core.h#L846
    size_t stack_size;

    if (bpf_probe_read_user(
          &stack_ptr_current,
          sizeof(stack_ptr_current),
          (void *)(current_ctx_addr + rubyinfo->vm_stack))) {
      DEBUG_PRINT("ruby: failed to read current stack pointer");
      increment_metric(metricID_UnwindRubyErrReadStackPtr);
      return ERR_RUBY_READ_STACK_PTR;
    }

    if (bpf_probe_read_user(
          &stack_size, sizeof(stack_size), (void *)(current_ctx_addr + rubyinfo->vm_stack_size))) {
      DEBUG_PRINT("ruby: failed to get stack size");
      increment_metric(metricID_UnwindRubyErrReadStackSize);
      return ERR_RUBY_READ_STACK_SIZE;
    }

    // Calculate the base of the stack so we can calculate the number of frames from it.
    // Ruby places two dummy frames on the Ruby VM stack in which we are not interested.
    // https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm_backtrace.c#L477-L485
    last_stack_frame = stack_ptr_current + (rubyinfo->size_of_value * stack_size) -
                       (2 * rubyinfo->size_of_control_frame_struct);

    if (bpf_probe_read_user(
          &stack_ptr, sizeof(stack_ptr), (void *)(current_ctx_addr + rubyinfo->cfp))) {
      DEBUG_PRINT("ruby: failed to get cfp");
      increment_metric(metricID_UnwindRubyErrReadCfp);
      return ERR_RUBY_READ_CFP;
    }
  }

  // iseq_addr holds the address to a rb_iseq_struct struct
  void *iseq_addr;
  // iseq_body points to a rb_iseq_constant_body struct
  void *iseq_body;
  // pc stores the Ruby VM program counter information
  u64 pc;
  // iseq_encoded holds the instruction address and operands of a particular instruction sequence
  // The format of this element is documented in:
  // https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm_core.h#L328-L348
  u64 iseq_encoded;
  // iseq_size holds the size in bytes of a particular instruction sequence
  u32 iseq_size;
  s64 n;

  UNROLL for (u32 i = 0; i < FRAMES_PER_WALK_RUBY_STACK; ++i)
  {
    pc        = 0;
    iseq_addr = NULL;

    bpf_probe_read_user(&iseq_addr, sizeof(iseq_addr), (void *)(stack_ptr + rubyinfo->iseq));
    bpf_probe_read_user(&pc, sizeof(pc), (void *)(stack_ptr + rubyinfo->pc));
    // If iseq or pc is 0, then this frame represents a registered hook.
    // https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm.c#L1960
    if (pc == 0 || iseq_addr == NULL) {
      // Ruby frames without a PC or iseq are special frames and do not hold information
      // we can use further on. So we either skip them or ask the native unwinder to continue.

      if (rubyinfo->version < 0x20600) {
        // With Ruby version 2.6 the scope of our entry symbol ruby_current_execution_context_ptr
        // got extended. We need this extension to jump back unwinding Ruby VM frames if we
        // continue at this point with unwinding native frames.
        // As this is not available for Ruby versions < 2.6 we just skip this indicator frame and
        // continue unwinding Ruby VM frames. Due to this issue, the ordering of Ruby and native
        // frames might not be correct for Ruby versions < 2.6.
        goto skip;
      }

      u64 ep = 0;
      if (bpf_probe_read_user(&ep, sizeof(ep), (void *)(stack_ptr + rubyinfo->ep))) {
        DEBUG_PRINT("ruby: failed to get ep");
        increment_metric(metricID_UnwindRubyErrReadEp);
        return ERR_RUBY_READ_EP;
      }

      if (
        (ep & (RUBY_FRAME_FLAG_LAMBDA | RUBY_FRAME_FLAG_BMETHOD)) ==
        (RUBY_FRAME_FLAG_LAMBDA | RUBY_FRAME_FLAG_BMETHOD)) {
        // When identifying Ruby lambda blocks at this point, we do not want to return to the
        // native unwinder. So we just skip this Ruby VM frame.
        goto skip;
      }

      stack_ptr += rubyinfo->size_of_control_frame_struct;
      *next_unwinder = PROG_UNWIND_NATIVE;
      goto save_state;
    }

    if (bpf_probe_read_user(&iseq_body, sizeof(iseq_body), (void *)(iseq_addr + rubyinfo->body))) {
      DEBUG_PRINT("ruby: failed to get iseq body");
      increment_metric(metricID_UnwindRubyErrReadIseqBody);
      return ERR_RUBY_READ_ISEQ_BODY;
    }

    if (bpf_probe_read_user(
          &iseq_encoded, sizeof(iseq_encoded), (void *)(iseq_body + rubyinfo->iseq_encoded))) {
      DEBUG_PRINT("ruby: failed to get iseq encoded");
      increment_metric(metricID_UnwindRubyErrReadIseqEncoded);
      return ERR_RUBY_READ_ISEQ_ENCODED;
    }

    if (bpf_probe_read_user(
          &iseq_size, sizeof(iseq_size), (void *)(iseq_body + rubyinfo->iseq_size))) {
      DEBUG_PRINT("ruby: failed to get iseq size");
      increment_metric(metricID_UnwindRubyErrReadIseqSize);
      return ERR_RUBY_READ_ISEQ_SIZE;
    }

    // To get the line number iseq_encoded is subtracted from pc. This result also represents the
    // size of the current instruction sequence. If the calculated size of the instruction sequence
    // is greater than the value in iseq_encoded we don't report this pc to user space.
    //
    // https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm_backtrace.c#L47-L48
    n = (pc - iseq_encoded) / rubyinfo->size_of_value;
    if (n > iseq_size || n < 0) {
      DEBUG_PRINT("ruby: skipping invalid instruction sequence");
      goto skip;
    }

    // For symbolization of the frame we forward the information about the instruction sequence
    // and program counter to user space.
    // From this we can then extract information like file or function name and line number.
    ErrorCode error = push_ruby(trace, (u64)iseq_body, pc);
    if (error) {
      DEBUG_PRINT("ruby: failed to push frame");
      return error;
    }
    increment_metric(metricID_UnwindRubyFrames);

  skip:
    if (last_stack_frame <= stack_ptr) {
      // We have processed all frames in the Ruby VM and can stop here.
      *next_unwinder = PROG_UNWIND_NATIVE;
      return ERR_OK;
    }
    stack_ptr += rubyinfo->size_of_control_frame_struct;
  }
  *next_unwinder = PROG_UNWIND_RUBY;

save_state:
  // Store the current progress in the Ruby unwind state so we can continue walking the stack
  // after the tail call.
  record->rubyUnwindState.stack_ptr        = stack_ptr;
  record->rubyUnwindState.last_stack_frame = last_stack_frame;

  return ERR_OK;
}

// unwind_ruby is the tail call destination for PROG_UNWIND_RUBY.
static EBPF_INLINE int unwind_ruby(struct pt_regs *ctx)
{
  PerCPURecord *record = get_per_cpu_record();
  if (!record)
    return -1;

  int unwinder           = get_next_unwinder_after_interpreter();
  ErrorCode error        = ERR_OK;
  u32 pid                = record->trace.pid;
  RubyProcInfo *rubyinfo = bpf_map_lookup_elem(&ruby_procs, &pid);
  if (!rubyinfo) {
    DEBUG_PRINT("No Ruby introspection data");
    error = ERR_RUBY_NO_PROC_INFO;
    increment_metric(metricID_UnwindRubyErrNoProcInfo);
    goto exit;
  }

  increment_metric(metricID_UnwindRubyAttempts);

  // Pointer for an address to a rb_execution_context_struct struct.
  void *current_ctx_addr = NULL;

  if (rubyinfo->version >= 0x30000) {
    // With Ruby 3.x and its internal change of the execution model, we can no longer
    // access rb_execution_context_struct directly. Therefore we have to first lookup
    // ruby_single_main_ractor and get access to the current execution context via
    // the offset to running_ec.

    void *single_main_ractor = NULL;
    if (bpf_probe_read_user(
          &single_main_ractor, sizeof(single_main_ractor), (void *)rubyinfo->current_ctx_ptr)) {
      goto exit;
    }

    if (bpf_probe_read_user(
          &current_ctx_addr,
          sizeof(current_ctx_addr),
          (void *)(single_main_ractor + rubyinfo->running_ec))) {
      goto exit;
    }
  } else {
    if (bpf_probe_read_user(
          &current_ctx_addr, sizeof(current_ctx_addr), (void *)rubyinfo->current_ctx_ptr)) {
      goto exit;
    }
  }

  if (!current_ctx_addr) {
    goto exit;
  }

  error = walk_ruby_stack(record, rubyinfo, current_ctx_addr, &unwinder);

exit:
  record->state.unwind_error = error;
  tail_call(ctx, unwinder);
  return -1;
}
MULTI_USE_FUNC(unwind_ruby)
