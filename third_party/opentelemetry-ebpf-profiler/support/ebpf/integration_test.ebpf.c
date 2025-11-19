// This file contains the code and map definitions that are used in integration tests only.

#include "bpfdefs.h"
#include "extmaps.h"
#include "frametypes.h"
#include "tracemgmt.h"
#include "types.h"

static EBPF_INLINE void send_sample_traces(void *ctx, u64 pid, s32 kstack)
{
  // Use the per CPU record for trace storage: it's too big for stack.
  PerCPURecord *record = get_pristine_per_cpu_record();
  if (!record) {
    return; // unreachable
  }

  // Single native frame, no kernel trace.
  Trace *trace = &record->trace;

  // Use COMM as a marker for our test traces. COMM[3] serves as test case ID.
  trace->comm[0] = 0xAA;
  trace->comm[1] = 0xBB;
  trace->comm[2] = 0xCC;

  trace->origin = TRACE_SAMPLING;

  trace->comm[3]         = 1;
  trace->pid             = pid;
  trace->tid             = pid;
  trace->kernel_stack_id = -1;
  trace->stack_len       = 1;
  trace->frames[0]       = (Frame){
          .kind         = FRAME_MARKER_NATIVE,
          .file_id      = 1337,
          .addr_or_line = 21,
  };
  send_trace(ctx, trace);

  // Single native frame, with kernel trace.
  trace->comm[3]         = 2;
  trace->kernel_stack_id = kstack;
  send_trace(ctx, trace);

  // Single Python frame.
  trace->comm[3]         = 3;
  trace->kernel_stack_id = -1;
  trace->stack_len       = 3;
  trace->frames[0]       = (Frame){
          .kind         = FRAME_MARKER_NATIVE,
          .file_id      = 1337,
          .addr_or_line = 42,
  };
  trace->frames[1] = (Frame){
    .kind         = FRAME_MARKER_NATIVE,
    .file_id      = 1338,
    .addr_or_line = 21,
  };
  trace->frames[2] = (Frame){
    .kind         = FRAME_MARKER_PYTHON,
    .file_id      = 1339,
    .addr_or_line = 22,
  };
  send_trace(ctx, trace);

  // Maximum length native trace.
  trace->comm[3]         = 4;
  trace->stack_len       = MAX_FRAME_UNWINDS;
  trace->kernel_stack_id = kstack;
  UNROLL for (u64 i = 0; i < MAX_FRAME_UNWINDS; ++i)
  {
    // NOTE: this init schema eats up a lot of instructions. If we need more
    // space later, we can instead just init `.kind` and a few fields in the
    // start, middle, and end of the trace.
    trace->frames[i] = (Frame){
      .kind         = FRAME_MARKER_NATIVE,
      .file_id      = ~i,
      .addr_or_line = i,
    };
  }
  send_trace(ctx, trace);
}

// tracepoint_integration__sched_switch fetches the current kernel stack ID from
// kernel_stackmap and communicates it to userspace via kernel_stack_id map.
SEC("tracepoint/integration/sched_switch")
int tracepoint_integration__sched_switch(void *ctx)
{
  u64 id  = bpf_get_current_pid_tgid();
  u64 pid = id >> 32;

  s32 kernel_stack_id = bpf_get_stackid(ctx, &kernel_stackmap, BPF_F_REUSE_STACKID);
  printt("pid %lld with kernel_stack_id %d", pid, kernel_stack_id);

  send_sample_traces(ctx, pid, kernel_stack_id);

  return 0;
}
