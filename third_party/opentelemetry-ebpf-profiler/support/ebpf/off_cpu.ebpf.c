#include "bpfdefs.h"
#include "tracemgmt.h"
#include "types.h"

// kprobe_progs maps from a program ID to a kprobe eBPF program
struct kprobe_progs_t {
  __uint(type, BPF_MAP_TYPE_PROG_ARRAY);
  __type(key, u32);
  __type(value, u32);
  __uint(max_entries, NUM_TRACER_PROGS);
} kprobe_progs SEC(".maps");

// sched_times keeps track of sched_switch call times.
struct sched_times_t {
  __uint(type, BPF_MAP_TYPE_LRU_PERCPU_HASH);
  __type(key, u64);         // pid_tgid
  __type(value, u64);       // time in ns
  __uint(max_entries, 256); // value is adjusted at load time in loadAllMaps.
} sched_times SEC(".maps");

// off_cpu_threshold is set during load time.
BPF_RODATA_VAR(u32, off_cpu_threshold, 0)

// tracepoint__sched_switch serves as entry point for off cpu profiling.
SEC("tracepoint/sched/sched_switch")
int tracepoint__sched_switch(UNUSED void *ctx)
{
  u64 pid_tgid = bpf_get_current_pid_tgid();
  u32 pid      = pid_tgid >> 32;
  u32 tid      = pid_tgid & 0xFFFFFFFF;

  if (pid == 0 || tid == 0) {
    return 0;
  }

  if (bpf_get_prandom_u32() > off_cpu_threshold) {
    return 0;
  }

  u64 ts = bpf_ktime_get_ns();

  if (bpf_map_update_elem(&sched_times, &pid_tgid, &ts, BPF_ANY) < 0) {
    DEBUG_PRINT("Failed to record sched_switch event entry");
    return 0;
  }

  return 0;
}

// kprobe__dummy is never loaded or called. It just makes sure kprobe_progs is
// referenced and make the compiler and linker happy.
SEC("kprobe/dummy")
int kprobe__dummy(struct pt_regs *ctx)
{
  bpf_tail_call(ctx, &kprobe_progs, 0);
  return 0;
}

// kp__finish_task_switch is triggered right after the scheduler updated
// the CPU registers.
SEC("kprobe/finish_task_switch")
int finish_task_switch(struct pt_regs *ctx)
{
  // Get the PID and TGID register.
  u64 pid_tgid = bpf_get_current_pid_tgid();
  u32 pid      = pid_tgid >> 32;
  u32 tid      = pid_tgid & 0xFFFFFFFF;

  if (pid == 0 || tid == 0) {
    return 0;
  }

  u64 ts = bpf_ktime_get_ns();

  u64 *start_ts = bpf_map_lookup_elem(&sched_times, &pid_tgid);
  if (!start_ts || *start_ts == 0) {
    // There is no information from the sched/sched_switch entry hook.
    return 0;
  }

  // Remove entry from the map so the stack for the same pid_tgid does not get unwound and
  // reported accidentally without the start timestamp updated in tracepoint/sched/sched_switch.
  bpf_map_delete_elem(&sched_times, &pid_tgid);

  u64 diff = ts - *start_ts;
  DEBUG_PRINT("==== finish_task_switch ====");

  return collect_trace(ctx, TRACE_OFF_CPU, pid, tid, ts, diff);
}
