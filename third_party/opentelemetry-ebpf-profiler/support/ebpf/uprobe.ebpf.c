#include "bpfdefs.h"
#include "tracemgmt.h"
#include "types.h"

// uprobe__generic serves as entry point for uprobe based profiling.
SEC("uprobe/generic")
int uprobe__generic(void *ctx)
{
  u64 pid_tgid = bpf_get_current_pid_tgid();
  u32 pid      = pid_tgid >> 32;
  u32 tid      = pid_tgid & 0xFFFFFFFF;

  if (pid == 0 || tid == 0) {
    return 0;
  }

  u64 ts = bpf_ktime_get_ns();

  return collect_trace(ctx, TRACE_UPROBE, pid, tid, ts, 0);
}
