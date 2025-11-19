#ifndef OPTI_TSD_H
#define OPTI_TSD_H

#include "bpfdefs.h"

// tpbase_offset is declared in native_stack_trace.ebpf.c
extern u64 tpbase_offset;

// tsd_read reads from the Thread Specific Data location associated with the provided key.
static inline EBPF_INLINE int
tsd_read(const TSDInfo *tsi, const void *tsd_base, int key, void **out)
{
  const void *tsd_addr = tsd_base + tsi->offset;
  if (tsi->indirect) {
    // Read the memory pointer that contains the per-TSD key data
    if (bpf_probe_read_user(&tsd_addr, sizeof(tsd_addr), tsd_addr)) {
      goto err;
    }
  }

  tsd_addr += key * tsi->multiplier;

  DEBUG_PRINT("readTSD key %d from address 0x%lx", key, (unsigned long)tsd_addr);
  if (bpf_probe_read_user(out, sizeof(*out), tsd_addr)) {
    goto err;
  }
  return 0;

err:
  DEBUG_PRINT("Failed to read TSD from 0x%lx", (unsigned long)tsd_addr);
  increment_metric(metricID_UnwindErrBadTSDAddr);
  return -1;
}

// tsd_get_base looks up the base address for TSD variables (TPBASE).
static inline EBPF_INLINE int tsd_get_base(void **tsd_base)
{
#ifdef TESTING_COREDUMP
  *tsd_base = (void *)__cgo_ctx->tp_base;
  return 0;
#else
  struct task_struct *task = (struct task_struct *)bpf_get_current_task();

  // We need to read task->thread.fsbase (on x86_64), but we can't do so because
  // we might have been compiled with different kernel headers, so the struct layout
  // is likely to be different.
  // tpbase_offset is populated with the offset of `fsbase` or equivalent field
  // relative to a `task_struct`, so we use that instead.
  void *tpbase_ptr = ((char *)task) + tpbase_offset;
  if (bpf_probe_read_kernel(tsd_base, sizeof(void *), tpbase_ptr)) {
    DEBUG_PRINT("Failed to read tpbase value");
    increment_metric(metricID_UnwindErrBadTPBaseAddr);
    return -1;
  }

  return 0;
#endif
}

#endif // OPTI_TSD_H
