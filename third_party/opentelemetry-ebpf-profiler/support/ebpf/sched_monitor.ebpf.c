// This file contains the code and map definitions for the tracepoint on the scheduler to
// report the stopping a process.

#include "bpfdefs.h"
#include "tracemgmt.h"

#include "types.h"

// See /sys/kernel/debug/tracing/events/sched/sched_process_free/format
// for struct layout. This is pre-6.16 format which uses a fixed-size
// (TASK_COMM_LEN) array for comm.
struct sched_process_free_ctx_pre616 {
  unsigned char skip[24];
  pid_t pid;
  int prio;
};

// This is the newer kernel version 6.16+ format.
// The change was introduced upstream with
// https://github.com/torvalds/linux/commit/155fd6c3e2f02efdc71a9b62888942efc217aff0
struct sched_process_free_ctx {
  unsigned char skip[12];
  pid_t pid;
  int prio;
};

static EBPF_INLINE int do_process_free(void *ctx, u32 pid)
{
  if (!bpf_map_lookup_elem(&reported_pids, &pid) && !pid_information_exists(pid)) {
    // Only report PIDs that we explicitly track. This avoids sending kernel worker PIDs
    // to userspace.
    goto exit;
  }

  if (report_pid(ctx, (u64)pid << 32 | pid, RATELIMIT_ACTION_RESET)) {
    increment_metric(metricID_NumProcExit);
  }
exit:
  return 0;
}

// tracepoint__sched_process_free is a tracepoint attached to the scheduler that frees processes.
// Every time a processes exits this hook is triggered.
SEC("tracepoint/sched/sched_process_free/v2")
int tracepoint__sched_process_free(struct sched_process_free_ctx *ctx)
{
  return do_process_free(ctx, ctx->pid);
}

SEC("tracepoint/sched/sched_process_free/v1")
int tracepoint__sched_process_free_pre616(struct sched_process_free_ctx_pre616 *ctx)
{
  return do_process_free(ctx, ctx->pid);
}
