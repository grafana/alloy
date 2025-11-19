// This file contains the code for extracting custom labels from Go runtime.

#include "bpfdefs.h"
#include "kernel.h"
#include "tracemgmt.h"
#include "tsd.h"
#include "types.h"

static EBPF_INLINE bool
get_go_custom_labels_from_slice(PerCPURecord *record, void *labels_slice_ptr)
{
  // https://github.com/golang/go/blob/80e2e474/src/runtime/pprof/label.go#L20
  struct GoSlice labels_slice;
  if (bpf_probe_read_user(&labels_slice, sizeof(struct GoSlice), labels_slice_ptr)) {
    DEBUG_PRINT("cl: failed to read value for labels slice (%lx)", (unsigned long)labels_slice_ptr);
    return false;
  }

  CustomLabelsArray *out = &record->trace.custom_labels;
  // len is number of pairs, ie its a vector of key/val structs.
  u8 num_to_read         = MIN(labels_slice.len, MAX_CUSTOM_LABELS);
  if (bpf_probe_read_user(
        &record->labels, sizeof(struct GoString) * 2 * num_to_read, labels_slice.array)) {
    DEBUG_PRINT(
      "cl: failed to read strings from labels slice (%lx)", (unsigned long)labels_slice.array);
    return false;
  }

  for (u8 i = 0; i < MAX_CUSTOM_LABELS; i++) {
    if (i >= labels_slice.len)
      break;
    CustomLabel *lbl = &out->labels[i];
    u8 klen          = MIN(record->labels[i * 2].len, CUSTOM_LABEL_MAX_KEY_LEN - 1);
    if (bpf_probe_read_user(lbl->key, klen, record->labels[i * 2].str)) {
      DEBUG_PRINT(
        "cl: failed to read key for custom label (%lx)", (unsigned long)record->labels[i * 2].str);
      return false;
    }
    u8 vlen = MIN(record->labels[i * 2 + 1].len, CUSTOM_LABEL_MAX_VAL_LEN - 1);
    if (bpf_probe_read_user(lbl->val, vlen, record->labels[i * 2 + 1].str)) {
      DEBUG_PRINT(
        "cl: failed to read key for custom label (%lx)",
        (unsigned long)record->labels[i * 2 + 1].str);
      return false;
    }
  }
  out->len = num_to_read;

  return true;
}

// https://github.com/golang/go/blob/6885bad7dd86880be6929c02085/src/internal/abi/map.go#L12
#define GO_MAP_BUCKET_SIZE 8

static EBPF_INLINE bool
get_go_custom_labels_from_map(PerCPURecord *record, void *labels_map_ptr_ptr, GoLabelsOffsets *offs)
{
  void *labels_map_ptr;
  if (bpf_probe_read_user(&labels_map_ptr, sizeof(labels_map_ptr), labels_map_ptr_ptr)) {
    DEBUG_PRINT(
      "cl: failed to read value for labels_map_ptr (%lx)", (unsigned long)labels_map_ptr_ptr);
    return false;
  }

  u64 labels_count = 0;
  if (bpf_probe_read_user(&labels_count, sizeof(labels_count), labels_map_ptr + offs->hmap_count)) {
    DEBUG_PRINT("cl: failed to read value for labels_count");
    return false;
  }
  if (labels_count == 0) {
    DEBUG_PRINT("cl: no labels");
    return false;
  }

  unsigned char log_2_bucket_count;
  if (bpf_probe_read_user(
        &log_2_bucket_count,
        sizeof(log_2_bucket_count),
        labels_map_ptr + offs->hmap_log2_bucket_count)) {
    DEBUG_PRINT("cl: failed to read value for bucket_count");
    return false;
  }
  void *label_buckets;
  if (bpf_probe_read_user(
        &label_buckets, sizeof(label_buckets), labels_map_ptr + offs->hmap_buckets)) {
    DEBUG_PRINT("cl: failed to read value for label_buckets");
    return false;
  }

  CustomLabelsArray *out = &record->trace.custom_labels;
  // If the map has more than 16 buckets we just don't support it, pprof maps are typically
  // small and if its a problem upgrading to Go 1.24+ is a potential solution.
  u8 bucket_count        = 1 << log_2_bucket_count;
  for (u8 b = 0; b < 16; b++) {
    if (b >= bucket_count)
      break;
    GoMapBucket *map_value = &record->goMapBucket;
    if (bpf_probe_read_user(
          map_value, sizeof(GoMapBucket), label_buckets + (b * sizeof(GoMapBucket)))) {
      return false;
    }

    for (u8 i = 0; i < GO_MAP_BUCKET_SIZE; i++) {
      if (out->len >= MAX_CUSTOM_LABELS)
        return true;
      CustomLabel *lbl = &out->labels[out->len];
      char tophash     = map_value->tophash[i];
      char *kstr       = map_value->keys[i].str;
      unsigned klen    = map_value->keys[i].len;
      char *vstr       = map_value->values[i].str;
      unsigned vlen    = map_value->values[i].len;
      if (tophash != 0 && kstr != NULL) {
        if (bpf_probe_read_user(lbl->key, MIN(klen, CUSTOM_LABEL_MAX_KEY_LEN - 1), kstr)) {
          DEBUG_PRINT("cl: failed to read key for custom label (%lx)", (unsigned long)kstr);
          return false;
        }
        if (bpf_probe_read_user(lbl->val, MIN(vlen, CUSTOM_LABEL_MAX_VAL_LEN - 1), vstr)) {
          DEBUG_PRINT("cl: failed to read value for custom label");
          return false;
        }
        out->len++;
      }
    }
  }

  return true;
}

// Go processes store the current goroutine in thread local store. From there
// this reads the g (aka goroutine) struct, then the m (the actual operating
// system thread) of that goroutine, and finally curg (current goroutine). This
// chain is necessary because getg().m.curg points to the current user g
// assigned to the thread (curg == getg() when not on the system stack). curg
// may be nil if there is no user g, such as when running in the scheduler. If
// curg is nil, then g is either a system stack (called g0) or a signal handler
// g (gsignal). Neither one will ever have label.
static EBPF_INLINE bool get_go_custom_labels(PerCPURecord *record, GoLabelsOffsets *offs)
{
  size_t curg_ptr_addr;
  if (bpf_probe_read_user(
        &curg_ptr_addr,
        sizeof(void *),
        (void *)(record->customLabelsState.go_m_ptr + offs->curg))) {
    DEBUG_PRINT("cl: failed to read value for m_ptr->curg");
    return false;
  }

  void *labels_ptr;
  if (bpf_probe_read_user(&labels_ptr, sizeof(void *), (void *)(curg_ptr_addr + offs->labels))) {
    DEBUG_PRINT(
      "cl: failed to read value for curg->labels (%lx->%lx)",
      (unsigned long)curg_ptr_addr,
      (unsigned long)offs->labels);
    return false;
  }

  if (offs->hmap_buckets == 0) {
    // go 1.24+ labels is a slice
    return get_go_custom_labels_from_slice(record, labels_ptr);
  }

  // go 1.23- labels is a map
  return get_go_custom_labels_from_map(record, labels_ptr, offs);
}

// go_labels is the entrypoint for extracting custom labels from Go runtime.
static EBPF_INLINE int go_labels(struct pt_regs *ctx)
{
  PerCPURecord *record = get_per_cpu_record();
  if (!record)
    return -1;

  u32 pid                  = record->trace.pid;
  GoLabelsOffsets *offsets = bpf_map_lookup_elem(&go_labels_procs, &pid);
  if (!offsets) {
    DEBUG_PRINT("cl: no offsets, %d not recognized as a go binary", pid);
    return -1;
  }
  DEBUG_PRINT(
    "cl: go offsets found, %d recognized as a go binary: m_ptr: %lx",
    pid,
    (unsigned long)record->customLabelsState.go_m_ptr);
  bool success = get_go_custom_labels(record, offsets);
  if (!success) {
    increment_metric(metricID_UnwindGoLabelsFailures);
  }

  send_trace(ctx, &record->trace);
  return 0;
}
MULTI_USE_FUNC(go_labels)
