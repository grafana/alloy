// This file contains the code and map definitions that are shared between
// the tracers, as well as a dispatcher program that can be attached to a
// perf event and will call the appropriate tracer for a given process

#include "bpfdefs.h"
#include "kernel.h"
#include "tracemgmt.h"
#include "tsd.h"
#include "types.h"

// Begin shared maps

// Per-CPU record of the stack being built and meta-data on the building process
struct per_cpu_records_t {
  __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
  __type(key, int);
  __type(value, PerCPURecord);
  __uint(max_entries, 1);
} per_cpu_records SEC(".maps");

// metrics maps metric ID to a value
struct metrics_t {
  __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
  __type(key, u32);
  __type(value, u64);
  __uint(max_entries, metricID_Max);
} metrics SEC(".maps");

// perf_progs maps from a program ID to a perf eBPF program
struct perf_progs_t {
  __uint(type, BPF_MAP_TYPE_PROG_ARRAY);
  __type(key, u32);
  __type(value, u32);
  __uint(max_entries, NUM_TRACER_PROGS);
} perf_progs SEC(".maps");

// report_events notifies user space about events (GENERIC_PID and TRACES_FOR_SYMBOLIZATION).
//
// As a key the CPU number is used and the value represents a perf event file descriptor.
// Information transmitted is the event type only. We use 0 as the number of max entries
// for this map as at load time it will be replaced by the number of possible CPUs. At
// the same time this will then also define the number of perf event rings that are
// used for this map.
struct report_events_t {
  __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
  __type(key, int);
  __type(value, u32);
  __uint(max_entries, 0);
} report_events SEC(".maps");

// reported_pids is a map that holds PIDs recently reported to user space.
//
// We use this map to avoid sending multiple notifications for the same PID to user space.
// As key, we use the PID and value is a rate limit token (see pid_event_ratelimit()).
// When sizing this map, we are thinking about the maximum number of unique PIDs that could
// be stored, without immediately being removed, that we would like to support. PIDs are
// either left to expire from the LRU or updated based on the rate limit token. Note that
// timeout checks are done lazily on access, so this map may contain multiple expired PIDs.
struct reported_pids_t {
  __uint(type, BPF_MAP_TYPE_LRU_HASH);
  __type(key, u32);
  __type(value, u64);
  __uint(max_entries, 65536);
} reported_pids SEC(".maps");

// pid_events is a map that holds PIDs that should be processed in user space.
//
// User space code will periodically iterate through the map and process each entry.
// Additionally, each time eBPF code writes a value into the map, user space is notified
// through event_send_trigger (which uses maps/report_events). As key we use the PID/TID
// of the process/thread and as value always true. When sizing this map, we are thinking
// about the maximum number of unique PIDs that could generate events we're interested in
// (process new, thread group exit, unknown PC) within a map monitor/processing interval,
// that we would like to support.
struct pid_events_t {
  __uint(type, BPF_MAP_TYPE_HASH);
  __type(key, u64);
  __type(value, bool);
  __uint(max_entries, 65536);
} pid_events SEC(".maps");

// The native unwinder needs to be able to determine how each mapping should be unwound.
//
// This map contains data to help the native unwinder translate from a virtual address in a given
// process. It contains information of the unwinder program to use, how to convert the virtual
// address to relative address, and what executable file is in question.
struct pid_page_to_mapping_info_t {
  __uint(type, BPF_MAP_TYPE_LPM_TRIE);
  __type(key, PIDPage);
  __type(value, PIDPageMappingInfo);
  __uint(max_entries, 524288); // 2^19
  __uint(map_flags, BPF_F_NO_PREALLOC);
} pid_page_to_mapping_info SEC(".maps");

// inhibit_events map is used to inhibit sending events to user space.
//
// Only one event needs to be sent as it's a manual trigger to start processing
// traces / PIDs early. HA (Go) will reset this entry once it has reacted to the
// trigger, so next event is sent when needed.
// NOTE: Update .max_entries if additional event types are added. The value should
// equal the number of different event types using this mechanism.
struct inhibit_events_t {
  __uint(type, BPF_MAP_TYPE_HASH);
  __type(key, u32);
  __type(value, bool);
  __uint(max_entries, 2);
} inhibit_events SEC(".maps");

// Perf event ring buffer for sending completed traces to user-mode.
//
// The map is periodically polled and read from in `tracer`.
struct trace_events_t {
  __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
  __type(key, int);
  __type(value, u32);
  __uint(max_entries, 0);
} trace_events SEC(".maps");

// End shared maps

struct apm_int_procs_t {
  __uint(type, BPF_MAP_TYPE_HASH);
  __type(key, pid_t);
  __type(value, ApmIntProcInfo);
  __uint(max_entries, 128);
} apm_int_procs SEC(".maps");

struct go_labels_procs_t {
  __uint(type, BPF_MAP_TYPE_HASH);
  __type(key, pid_t);
  __type(value, GoLabelsOffsets);
  __uint(max_entries, 128);
} go_labels_procs SEC(".maps");

// drop_error_only_traces is set during load time.
BPF_RODATA_VAR(bool, drop_error_only_traces, false)

static EBPF_INLINE void *get_m_ptr(struct GoLabelsOffsets *offs, UNUSED UnwindState *state)
{
  u64 g_addr     = 0;
  void *tls_base = NULL;
  if (tsd_get_base(&tls_base) < 0) {
    DEBUG_PRINT("cl: failed to get tsd base; can't read m_ptr");
    return NULL;
  }
  DEBUG_PRINT(
    "cl: read tsd_base at 0x%lx, g offset: %d", (unsigned long)tls_base, offs->tls_offset);

  if (offs->tls_offset == 0) {
#if defined(__aarch64__)
    // On aarch64 for !iscgo programs the g is only stored in r28 register.
    g_addr = state->r28;
#elif defined(__x86_64__)
    DEBUG_PRINT("cl: TLS offset for g pointer missing for amd64");
    return NULL;
#endif
  }

  if (g_addr == 0) {
    if (bpf_probe_read_user(&g_addr, sizeof(void *), (void *)((s64)tls_base + offs->tls_offset))) {
      DEBUG_PRINT("cl: failed to read g_addr, tls_base(%lx)", (unsigned long)tls_base);
      return NULL;
    }
  }

  DEBUG_PRINT("cl: reading m_ptr_addr at 0x%lx + 0x%x", (unsigned long)g_addr, offs->m_offset);
  void *m_ptr_addr;
  if (bpf_probe_read_user(&m_ptr_addr, sizeof(void *), (void *)(g_addr + offs->m_offset))) {
    DEBUG_PRINT("cl: failed m_ptr_addr");
    return NULL;
  }
  DEBUG_PRINT("cl: m_ptr_addr 0x%lx", (unsigned long)m_ptr_addr);
  return m_ptr_addr;
}

static EBPF_INLINE void maybe_add_go_custom_labels(struct pt_regs *ctx, PerCPURecord *record)
{
  u32 pid                  = record->trace.pid;
  GoLabelsOffsets *offsets = bpf_map_lookup_elem(&go_labels_procs, &pid);
  if (!offsets) {
    DEBUG_PRINT("cl: no offsets, %d not recognized as a go binary", pid);
    return;
  }

  void *m_ptr_addr = get_m_ptr(offsets, &record->state);
  if (!m_ptr_addr) {
    return;
  }
  record->customLabelsState.go_m_ptr = m_ptr_addr;

  DEBUG_PRINT("cl: trace is within a process with Go custom labels enabled");
  increment_metric(metricID_UnwindGoLabelsAttempts);
  // The Go label extraction code is too big to fit in the UNWIND_STOP program, so
  // it is tail_call'd.
  tail_call(ctx, PROG_GO_LABELS);
}

static EBPF_INLINE void maybe_add_apm_info(Trace *trace)
{
  u32 pid              = trace->pid; // verifier needs this to be on stack on 4.15 kernel
  ApmIntProcInfo *proc = bpf_map_lookup_elem(&apm_int_procs, &pid);
  if (!proc) {
    return;
  }

  DEBUG_PRINT("Trace is within a process with APM integration enabled");

  u64 tsd_base;
  if (tsd_get_base((void **)&tsd_base) != 0) {
    increment_metric(metricID_UnwindApmIntErrReadTsdBase);
    DEBUG_PRINT("Failed to get TSD base for APM integration");
    return;
  }

  DEBUG_PRINT("APM corr ptr should be at 0x%llx", tsd_base + proc->tls_offset);

  void *apm_corr_buf_ptr;
  if (bpf_probe_read_user(
        &apm_corr_buf_ptr, sizeof(apm_corr_buf_ptr), (void *)(tsd_base + proc->tls_offset))) {
    increment_metric(metricID_UnwindApmIntErrReadCorrBufPtr);
    DEBUG_PRINT("Failed to read APM correlation buffer pointer");
    return;
  }

  ApmCorrelationBuf corr_buf;
  if (bpf_probe_read_user(&corr_buf, sizeof(corr_buf), apm_corr_buf_ptr)) {
    increment_metric(metricID_UnwindApmIntErrReadCorrBuf);
    DEBUG_PRINT("Failed to read APM correlation buffer");
    return;
  }

  if (corr_buf.trace_present && corr_buf.valid) {
    trace->apm_trace_id.as_int.hi    = corr_buf.trace_id.as_int.hi;
    trace->apm_trace_id.as_int.lo    = corr_buf.trace_id.as_int.lo;
    trace->apm_transaction_id.as_int = corr_buf.transaction_id.as_int;
  }

  increment_metric(metricID_UnwindApmIntReadSuccesses);

  // WARN: we print this as little endian
  DEBUG_PRINT(
    "APM transaction ID: %016llX, flags: 0x%02X",
    trace->apm_transaction_id.as_int,
    corr_buf.trace_flags);
}

// unwind_stop is the tail call destination for PROG_UNWIND_STOP.
static EBPF_INLINE int unwind_stop(struct pt_regs *ctx)
{
  PerCPURecord *record = get_per_cpu_record();
  if (!record)
    return -1;
  Trace *trace       = &record->trace;
  UnwindState *state = &record->state;

  maybe_add_apm_info(trace);

  // If the stack is otherwise empty, push an error for that: we should
  // never encounter empty stacks for successful unwinding.
  if (trace->stack_len == 0 && trace->kernel_stack_id < 0) {
    DEBUG_PRINT("unwind_stop called but the stack is empty");
    increment_metric(metricID_ErrEmptyStack);
    if (!state->unwind_error) {
      state->unwind_error = ERR_EMPTY_STACK;
    }
  }

  // If unwinding was aborted due to a critical error, push an error frame.
  if (state->unwind_error) {
    DEBUG_PRINT("Aborting further unwinding due to error code %d", state->unwind_error);
    push_error(&record->trace, state->unwind_error);
  }

  switch (state->error_metric) {
  case -1:
    // No Error
    break;
  case metricID_UnwindNativeErrWrongTextSection:;
    u64 pid_tgid = (u64)trace->pid << 32 | trace->tid;
    if (report_pid(ctx, pid_tgid, record->ratelimitAction)) {
      increment_metric(metricID_NumUnknownPC);
    }
    // Fallthrough to report the error
  default: increment_metric(state->error_metric);
  }

  // TEMPORARY HACK
  //
  // If we ended up with a trace that consists of only a single error frame, drop it.
  // This is required as long as the process manager provides the option to filter out
  // error frames, to prevent empty traces from being sent. While it might seem that this
  // filtering should belong into the HA code that does the filtering, it is actually
  // surprisingly hard to implement that way: since traces and their counts are reported
  // through different data structures, we'd have to keep a list of known empty traces to
  // also prevent the corresponding trace counts to be sent out. OTOH, if we do it here,
  // this is trivial.
  if (trace->stack_len == 1 && trace->kernel_stack_id < 0 && state->unwind_error) {
    if (drop_error_only_traces) {
      return 0;
    }
  }
  // TEMPORARY HACK END

  // Must be last since it may not return (it will call send_trace).
  maybe_add_go_custom_labels(ctx, record);

  send_trace(ctx, trace);

  return 0;
}
MULTI_USE_FUNC(unwind_stop)

char _license[] SEC("license") = "GPL";
// this number will be interpreted by the elf loader
// to set the current running kernel version
u32 _version SEC("version")    = 0xFFFFFFFE;
