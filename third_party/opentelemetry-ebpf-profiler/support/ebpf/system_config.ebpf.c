// This file contains the code and map definitions for system configuration
// and analysis related functions.

#include "bpfdefs.h"
#include "extmaps.h"
#include "types.h"

#ifndef TESTING_COREDUMP

// system_analysis is the bpf map the HA and this module uses to communicate
struct system_analysis_t {
  __uint(type, BPF_MAP_TYPE_ARRAY);
  __type(key, u32);
  __type(value, struct SystemAnalysis);
  __uint(max_entries, 1);
} system_analysis SEC(".maps");

// read_kernel_memory reads data from given kernel address. This is
// invoked once on entry to bpf() syscall on the given pid context.
SEC("tracepoint/syscalls/sys_enter_bpf")
int read_kernel_memory(UNUSED void *ctx)
{
  u32 key0 = 0;

  struct SystemAnalysis *sys = bpf_map_lookup_elem(&system_analysis, &key0);
  if (!sys) {
    // Not reachable. The one array element always exists.
    return 0;
  }

  if (sys->pid != (bpf_get_current_pid_tgid() >> 32)) {
    // Execute the hook only in the context of requesting task.
    return 0;
  }

  // Mark request handled
  sys->pid = 0;

  // Handle the read request
  if (bpf_probe_read_kernel(sys->code, sizeof(sys->code), (void *)sys->address)) {
    DEBUG_PRINT("Failed to read code from 0x%lx", (unsigned long)sys->address);
    return -1;
  }

  return 0;
}

// read_task_struct reads data the current struct task_struct along with
// the struct pt_regs pointer to the entry stack's usermode cpu state.
// Requires kernel 4.19 or newer due to attaching to a raw tracepoint.
SEC("raw_tracepoint/sys_enter")
int read_task_struct(struct bpf_raw_tracepoint_args *ctx)
{
  struct pt_regs *regs = (struct pt_regs *)ctx->args[0];
  u32 key0             = 0;

  struct SystemAnalysis *sys = bpf_map_lookup_elem(&system_analysis, &key0);
  if (!sys) {
    // Not reachable. The one array element always exists.
    return 0;
  }

  if (sys->pid != (bpf_get_current_pid_tgid() >> 32)) {
    // Execute the hook only in the context of requesting task.
    return 0;
  }

  // Mark request handled
  sys->pid = 0;

  // Request to read current task. Adjust read address, and return
  // also the address of struct pt_regs in the entry stack.
  u64 addr = bpf_get_current_task() + sys->address;

  // As this is a raw tracepoint for syscall entry, the struct pt_regs *
  // is guaranteed to be the user mode cpu state on the entry stack.
  // Return this to the caller.
  sys->address = (u64)regs;

  // Execute the read request.
  if (bpf_probe_read_kernel(sys->code, sizeof(sys->code), (void *)addr)) {
    DEBUG_PRINT("Failed to read task_struct from 0x%lx", (unsigned long)addr);
    return -1;
  }

  return 0;
}

#endif
