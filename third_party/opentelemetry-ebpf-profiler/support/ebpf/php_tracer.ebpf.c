// This file contains the code and map definitions for the PHP tracer

#include "bpfdefs.h"
#include "tracemgmt.h"
#include "types.h"

// The number of PHP frames to unwind per frame-unwinding eBPF program. If
// we start running out of instructions in the walk_php_stack program, one
// option is to adjust this number downwards.
#define FRAMES_PER_WALK_PHP_STACK 19

// The type_info flag for executor data to indicate top-of-stack frames
// as defined in php/Zend/zend_compile.h.
#define ZEND_CALL_TOP (1 << 17)

// zend_function.type values we need from php/Zend/zend_compile.h
#define ZEND_USER_FUNCTION 2
#define ZEND_EVAL_CODE     4

// Map from PHP process IDs to the address of the `executor_globals` for that process
struct php_procs_t {
  __uint(type, BPF_MAP_TYPE_HASH);
  __type(key, pid_t);
  __type(value, PHPProcInfo);
  __uint(max_entries, 1024);
} php_procs SEC(".maps");

// Record a PHP frame
static EBPF_INLINE ErrorCode push_php(Trace *trace, u64 file, u64 line, bool is_jitted)
{
  int frame_type = is_jitted ? FRAME_MARKER_PHP_JIT : FRAME_MARKER_PHP;
  return _push(trace, file, line, frame_type);
}

// Record a PHP call for which no function object is available
static EBPF_INLINE ErrorCode push_unknown_php(Trace *trace)
{
  return _push(trace, UNKNOWN_FILE, FUNC_TYPE_UNKNOWN, FRAME_MARKER_PHP);
}

static EBPF_INLINE int process_php_frame(
  PerCPURecord *record,
  PHPProcInfo *phpinfo,
  bool is_jitted,
  const void *execute_data,
  u32 *type_info)
{
  Trace *trace = &record->trace;

  // Get current_execute_data->func
  void *zend_function;
  if (bpf_probe_read_user(
        &zend_function, sizeof(void *), execute_data + phpinfo->zend_execute_data_function)) {
    DEBUG_PRINT(
      "Failed to read current_execute_data->func (0x%lx)",
      (unsigned long)(execute_data + phpinfo->zend_execute_data_function));
    return metricID_UnwindPHPErrBadZendExecuteData;
  }

  // It is possible there is no function object.
  if (!zend_function) {
    if (push_unknown_php(trace) != ERR_OK) {
      DEBUG_PRINT("failed to push unknown php frame");
      return -1;
    }
    return metricID_UnwindPHPFrames;
  }

  // Get zend_function->type
  u8 func_type;
  if (bpf_probe_read_user(
        &func_type, sizeof(func_type), zend_function + phpinfo->zend_function_type)) {
    DEBUG_PRINT("Failed to read execute_data->func->type (0x%lx)", (unsigned long)zend_function);
    return metricID_UnwindPHPErrBadZendFunction;
  }

  u32 lineno = 0;
  if (func_type == ZEND_USER_FUNCTION || func_type == ZEND_EVAL_CODE) {
    // Get execute_data->opline
    void *zend_op;
    if (bpf_probe_read_user(
          &zend_op, sizeof(void *), execute_data + phpinfo->zend_execute_data_opline)) {
      DEBUG_PRINT(
        "Failed to read execute_data->opline (0x%lx)",
        (unsigned long)(execute_data + phpinfo->zend_execute_data_opline));
      return metricID_UnwindPHPErrBadZendExecuteData;
    }

    // Get opline->lineno
    if (bpf_probe_read_user(&lineno, sizeof(u32), zend_op + phpinfo->zend_op_lineno)) {
      DEBUG_PRINT(
        "Failed to read executor_globals->opline->lineno (0x%lx)",
        (unsigned long)(zend_op + phpinfo->zend_op_lineno));
      return metricID_UnwindPHPErrBadZendOpline;
    }

    // Get execute_data->This.type_info. This reads into the `type_info` argument
    // so we can reuse it in walk_php_stack
    if (bpf_probe_read_user(
          type_info, sizeof(u32), execute_data + phpinfo->zend_execute_data_this_type_info)) {
      DEBUG_PRINT(
        "Failed to read execute_data->This.type_info (0x%lx)", (unsigned long)execute_data);
      return metricID_UnwindPHPErrBadZendExecuteData;
    }
  }

  // To give more information to the HA we also pass up the type info. This is safe
  // because lineno is 32-bits too.
  u64 lineno_and_type_info = ((u64)*type_info) << 32 | lineno;

  DEBUG_PRINT("Pushing PHP 0x%lx %u", (unsigned long)zend_function, lineno);
  if (push_php(trace, (u64)zend_function, lineno_and_type_info, is_jitted) != ERR_OK) {
    DEBUG_PRINT("failed to push php frame");
    return -1;
  }

  return metricID_UnwindPHPFrames;
}

static EBPF_INLINE int walk_php_stack(PerCPURecord *record, PHPProcInfo *phpinfo, bool is_jitted)
{
  const void *execute_data = record->phpUnwindState.zend_execute_data;
  bool mixed_traces        = get_next_unwinder_after_interpreter() != PROG_UNWIND_STOP;

  // If PHP data is not available, all frames have been processed, then
  // continue with native unwinding.
  if (!execute_data) {
    return get_next_unwinder_after_interpreter();
  }

  int unwinder  = PROG_UNWIND_PHP;
  u32 type_info = 0;
  UNROLL for (u32 i = 0; i < FRAMES_PER_WALK_PHP_STACK; ++i)
  {
    int metric = process_php_frame(record, phpinfo, is_jitted, execute_data, &type_info);
    if (metric >= 0) {
      increment_metric(metric);
    }
    if (metric != metricID_UnwindPHPFrames) {
      goto err;
    }

    // Get current_execute_data->prev_execute_data
    if (bpf_probe_read_user(
          &execute_data,
          sizeof(void *),
          execute_data + phpinfo->zend_execute_data_prev_execute_data)) {
      DEBUG_PRINT(
        "Failed to read current_execute_data->prev_execute_data (0x%lx)",
        (unsigned long)execute_data);
      increment_metric(metricID_UnwindPHPErrBadZendExecuteData);
      goto err;
    }

    // Check end-of-stack and end of current interpreter loop stack conditions
    if (!execute_data || (mixed_traces && (type_info & ZEND_CALL_TOP))) {
      DEBUG_PRINT("Top-of-stack, with next execute_data=0x%lx", (unsigned long)execute_data);
      // JIT'd PHP code needs special support for recovering the return address on both amd64
      // and arm.
      // Essentially we have two cases here:
      // 1) The PC corresponds to something in the interpreter loop. We have stack
      //    deltas for this, so we don't need to do anything.
      // 2) The PC corresponds to something in the JIT region. We don't have stack
      //    deltas for this, so we need to use the previously recovered address.
      //    This previously recovered return address corresponds to an address inside
      //    "execute_ex" (the PHP interpreter loop). In particular, the asm looks like this:
      //    jmp [r15]
      //    mov rax, imm <==== This is the return address we previously recovered
      //    This approach only works because the address we're using here is inside the
      //    interpreter loop and on the same native stack frame: otherwise we'd need to
      //    get the next unwinder instead.
      // This is only necessary when it's the last function because walking the PHP
      // stack is enough for the other functions.
      if (is_jitted) {
        record->state.pc             = phpinfo->jit_return_address;
        record->state.return_address = false;
        if (resolve_unwind_mapping(record, &unwinder) != ERR_OK) {
          unwinder = PROG_UNWIND_STOP;
        }
      } else {
        unwinder = get_next_unwinder_after_interpreter();
      }
      break;
    }
  }

  if (!execute_data) {
  err:
    unwinder_mark_done(record, PROG_UNWIND_PHP);
  }
  record->phpUnwindState.zend_execute_data = execute_data;
  return unwinder;
}

// unwind_php is the tail call destination for PROG_UNWIND_PHP.
static EBPF_INLINE int unwind_php(struct pt_regs *ctx)
{
  PerCPURecord *record = get_per_cpu_record();
  if (!record)
    return -1;

  int unwinder         = get_next_unwinder_after_interpreter();
  u32 pid              = record->trace.pid;
  PHPProcInfo *phpinfo = bpf_map_lookup_elem(&php_procs, &pid);
  if (!phpinfo) {
    DEBUG_PRINT("No PHP introspection data");
    goto exit;
  }

  // The section id and bias are zeroes if matched via JIT page mapping.
  // Otherwise its the native code interpreter range match and these are
  // set to the native code's values.
  bool is_jitted = record->state.text_section_id == 0 && record->state.text_section_bias == 0;

  increment_metric(metricID_UnwindPHPAttempts);

  if (!record->phpUnwindState.zend_execute_data) {
    // Get executor_globals.current_execute_data
    if (bpf_probe_read_user(
          &record->phpUnwindState.zend_execute_data,
          sizeof(void *),
          (void *)phpinfo->current_execute_data)) {
      DEBUG_PRINT(
        "Failed to read executor_globals.current_execute data (0x%lx)",
        (unsigned long)phpinfo->current_execute_data);
      increment_metric(metricID_UnwindPHPErrBadCurrentExecuteData);
      goto exit;
    }
  }

#if defined(__aarch64__)
  // On ARM we need to adjust the stack pointer if we entered from JIT code
  // This is only a problem on ARM where the SP/FP are used for unwinding.
  // This is necessary because:
  // a) The PHP VM jumps into code by default. This is equivalent to having an inner frame.
  // b) The PHP VM allocates some space for alignment purposes and saving registers.
  // c) The amount and alignment of this space can change in hard-to-detect ways.
  // Given that there's no guarantess that anything pushed to the stack is useful we
  // simply ignore it. There may be a return address in some modes, but this is hard to detect
  // consistently.
  if (is_jitted) {
    record->state.sp = record->state.fp;
  }
#endif

  DEBUG_PRINT(
    "Building PHP stack (execute_data = 0x%lx)",
    (unsigned long)record->phpUnwindState.zend_execute_data);

  // Unwind one call stack or unrolled length, and continue
  unwinder = walk_php_stack(record, phpinfo, is_jitted);

exit:
  tail_call(ctx, unwinder);
  return -1;
}
MULTI_USE_FUNC(unwind_php)
