// This file contains the code and map definitions for the Dotnet tracer
//
// Core unwinding of frames is simple, as all the generated code uses frame pointers,
// and all the interesting data is directly accessible via FP.
//
// See the host agent interpreter/dotnet/ for more references.

#include "bpfdefs.h"
#include "tracemgmt.h"
#include "types.h"

// The number of dotnet frames to unwind per frame-unwinding eBPF program.
#define DOTNET_FRAMES_PER_PROGRAM 6

// The maximum dotnet frame length used in heuristic to validate FP
#define DOTNET_MAX_FRAME_LENGTH 8192

// Keep in sync with dotnet interpreter code
#define DOTNET_CODE_JIT       0x1f
#define DOTNET_CODE_FLAG_LEAF 0x80

// Map from dotnet process IDs to a structure containing addresses of variables
// we require in order to build the stack trace
struct dotnet_procs_t {
  __uint(type, BPF_MAP_TYPE_HASH);
  __type(key, pid_t);
  __type(value, DotnetProcInfo);
  __uint(max_entries, 1024);
} dotnet_procs SEC(".maps");

// Nibble map tunables
// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/inc/nibblemapmacros.h
#define DOTNET_CODE_ALIGN             4
#define DOTNET_CODE_NIBBLES_PER_ENTRY 8  // 8nibbles * 4 bits/nibble = 32bit word
#define DOTNET_CODE_BYTES_PER_NIBBLE  32 // one nibble maps to 32 bytes of code
#define DOTNET_CODE_BYTES_PER_ENTRY   (DOTNET_CODE_BYTES_PER_NIBBLE * DOTNET_CODE_NIBBLES_PER_ENTRY)

// Find method code header using a dotnet coreclr "NibbleMap"
// Currently this technically could require an unbounded for loop to scan through the nibble map.
// The make things work in the eBPF the number of elements we parse are limited by the scratch
// buffer size. This needs to be in eBPF for the methods which may be Garbage Collected (typically
// short runtime generated IL code). If we start seeing "code too large" errors, we can also do
// this same lookup from the Host Agent because most generated code (especially large pieces) are
// currently not Garbage Collected by the runtime. Though, we have submitted also an enhancement
// request to fix the nibble map format to something sane, and this might get implemented.
// see: https://github.com/dotnet/runtime/issues/93550
static EBPF_INLINE ErrorCode dotnet_find_code_start(PerCPURecord *record, u64 pc, u64 *code_start)
{
  // This is an ebpf optimized implementation of EEJitManager::FindMethodCode()
  // https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/codeman.cpp#L4115
  // The support code setups the page mapping so that:
  //   text_section_base = pHp->mapBase (base address of the JIT area)
  //   text_section_id   = pHp->pHdrMap (pointer to the nibble map)
  const UnwindState *state          = &record->state;
  DotnetUnwindScratchSpace *scratch = &record->dotnetUnwindScratch;
  const int map_elements            = sizeof(scratch->map) / sizeof(scratch->map[0]) / 2;
  u64 pc_base                       = state->text_section_bias;
  u64 pc_delta                      = pc - pc_base;
  u64 map_start                     = state->text_section_id;

  DEBUG_PRINT(
    "dotnet:  --> find code start for %lx: pc_base %lx, map_start %lx",
    (unsigned long)pc_delta,
    (unsigned long)pc_base,
    (unsigned long)map_start);
  pc_delta &= ~(DOTNET_CODE_ALIGN - 1);

  // Read the nibble map data

  // Calculate read to offset based on map_start so that end of scratch->map corresponds to pc_delta
  long offs = (long)map_elements - pc_delta / DOTNET_CODE_BYTES_PER_ENTRY - 1;
  if (offs < 0) {
    // We can read full scratch buffer, adjust map_start so that last entry read corresponds
    // pc_delta
    map_start +=
      pc_delta / DOTNET_CODE_BYTES_PER_ENTRY * sizeof(u32) - sizeof(scratch->map) + sizeof(u32);
    offs = 0;
  }
  offs %= map_elements;
  if (bpf_probe_read_user(&scratch->map[offs], sizeof(scratch->map) / 2, (void *)map_start)) {
    goto bad_code_header;
  }

  // Determine if the first map entry contains the start region
  int pos = map_elements;
  u32 val = scratch->map[--pos];
  DEBUG_PRINT("dotnet:  --> find code start for %lx: first entry %x", (unsigned long)pc_delta, val);
  val >>= 28 - ((pc_delta / DOTNET_CODE_BYTES_PER_NIBBLE) % DOTNET_CODE_NIBBLES_PER_ENTRY) * 4;
  if (val != 0) {
    // Adjust pc_delta to beginning of the positioned nibble of 'val'
    pc_delta &= ~(DOTNET_CODE_BYTES_PER_NIBBLE - 1);
  } else {
    // Adjust delta to end of previous map entry
    pc_delta &= ~(DOTNET_CODE_BYTES_PER_ENTRY - 1);
    pc_delta -= DOTNET_CODE_BYTES_PER_NIBBLE;
    val = scratch->map[--pos];
    DEBUG_PRINT(
      "dotnet:  --> find code start for %lx: second entry %x", (unsigned long)pc_delta, val);

    // Find backwards the first non-zero entry as it marks function start
    // This is unrolled several times, so it needs to be minimal in size.
    // And currently this is the major limit for DOTNET_FRAMES_PER_PROGRAM.
    int orig_pos = pos;
    if (val == 0) {
      // pos is fixed to map_elements-2 at this point, and is even.
      // convert it from u32 to u64 offset
      int pos64 = (map_elements - 2) / 2;
      u64 val64 = scratch->map64[--pos64];

      // the loop iterations is number of u64 elements minus two:
      // - the last element special handled earlier
      // - the second last element which is preloaded immediately above
      UNROLL for (int i = 0; i < map_elements / 2 - 2; i++)
      {
        if (val64 != 0) {
          break;
        }
        val64 = scratch->map64[--pos64];
      }
      // convert u64 offset back to u32 offset
      pos = pos64 * 2;
      // use the upper half entry if it is non-zero
      if (val64 >> 32 != 0) {
        val64 >>= 32;
        pos++;
      }
      val = val64;
    }

    // Adjust pc_delta based on how many iterations were done
    u64 pc_skipped = DOTNET_CODE_BYTES_PER_ENTRY * (orig_pos - pos);
    if (pc_delta < pc_skipped) {
      DEBUG_PRINT("dotnet: nibble map search went below pc_base");
      goto bad_code_header;
    }
    pc_delta -= pc_skipped;
    DEBUG_PRINT(
      "dotnet:  --> find code start for %lx: skipped %d, entry %x",
      (unsigned long)pc_delta,
      orig_pos - pos,
      val);
    if (val == 0) {
      increment_metric(metricID_UnwindDotnetErrCodeTooLarge);
      return ERR_DOTNET_CODE_TOO_LARGE;
    }
  }

  // Decode the code start info from the entry
  UNROLL for (int i = 0; i < DOTNET_CODE_NIBBLES_PER_ENTRY; i++)
  {
    u8 nybble = val & 0xf;
    if (nybble != 0) {
      *code_start = pc_base + pc_delta + (nybble - 1) * DOTNET_CODE_ALIGN;
      DEBUG_PRINT(
        "dotnet:  --> pc_delta = %lx, val=%x, ret=%lx",
        (unsigned long)pc_delta,
        nybble,
        (unsigned long)*code_start);
      return ERR_OK;
    }
    val >>= 4;
    pc_delta -= DOTNET_CODE_BYTES_PER_NIBBLE;
  }

bad_code_header:
  DEBUG_PRINT("dotnet: not found");
  increment_metric(metricID_UnwindDotnetErrCodeHeader);
  return ERR_DOTNET_CODE_HEADER;
}

// Record a Dotnet frame
static EBPF_INLINE ErrorCode
push_dotnet(Trace *trace, u64 code_header_ptr, u64 pc_offset, bool return_address)
{
  return _push_with_return_address(
    trace, code_header_ptr, pc_offset, FRAME_MARKER_DOTNET, return_address);
}

// Unwind one dotnet frame
static EBPF_INLINE ErrorCode unwind_one_dotnet_frame(PerCPURecord *record)
{
  UnwindState *state = &record->state;
  Trace *trace       = &record->trace;
  u64 regs[2], sp = state->sp, fp = state->fp, pc = state->pc;
  bool return_address = state->return_address;

  // All dotnet frames have frame pointer. Check that the FP looks valid.
  DEBUG_PRINT(
    "dotnet: pc: %lx, sp: %lx, fp: %lx", (unsigned long)pc, (unsigned long)sp, (unsigned long)fp);

  if (fp < sp || fp >= sp + DOTNET_MAX_FRAME_LENGTH) {
    DEBUG_PRINT(
      "dotnet: frame pointer too far off %lx / %lx", (unsigned long)fp, (unsigned long)sp);
    increment_metric(metricID_UnwindDotnetErrBadFP);
    return ERR_DOTNET_BAD_FP;
  }

  // Default to R2R/stub code_start.
  u64 type            = state->text_section_id;
  u64 code_start      = state->text_section_bias;
  u64 code_header_ptr = pc;

  unwinder_mark_nonleaf_frame(state);

  if (type < 0x100 && (type & DOTNET_CODE_FLAG_LEAF)) {
    // Stub frame that does not do calls.
    // For arm this is unwind with LR, and for x86-64 unwind with RA only.
    if (bpf_probe_read_user(&state->pc, sizeof(state->pc), (void *)state->sp)) {
      DEBUG_PRINT("dotnet:  --> bad stack pointer");
      increment_metric(metricID_UnwindDotnetErrBadFP);
      return ERR_DOTNET_BAD_FP;
    }
    state->sp += 8;
    type &= 0x7f;
    goto push_frame;
  }

  // Unwind with frame pointer. On Linux the frame pointers are always on.
  // https://github.com/dotnet/runtime/blob/main/docs/design/coreclr/botr/clr-abi.md#system-v-x86_64-support
  // FIXME: Early prologue and epilogue may skip a frame. Seems prologue is fixed, consider
  // using heuristic to handle prologue when the new frame is not yet pushed to stack.
  if (bpf_probe_read_user(regs, sizeof(regs), (void *)fp)) {
    DEBUG_PRINT("dotnet:  --> bad frame pointer");
    increment_metric(metricID_UnwindDotnetErrBadFP);
    return ERR_DOTNET_BAD_FP;
  }
  state->sp = fp + sizeof(regs);
  state->fp = regs[0];
  state->pc = regs[1];
  DEBUG_PRINT(
    "dotnet: pc: %lx, sp: %lx, fp: %lx",
    (unsigned long)state->pc,
    (unsigned long)state->sp,
    (unsigned long)state->fp);

  if (type < 0x100) {
    // Not a JIT frame. A R2R frame at this point.
    type &= 0x7f;
    goto push_frame;
  }

  // JIT generated code, locate code start
  ErrorCode error = dotnet_find_code_start(record, pc, &code_start);
  if (error != ERR_OK) {
    DEBUG_PRINT("dotnet:  --> code_start failed with %d", error);
    // dotnet_find_code_start incremented the metric already
    if (error != ERR_DOTNET_CODE_TOO_LARGE) {
      return error;
    }
    return _push(trace, 0, ERR_DOTNET_CODE_TOO_LARGE, FRAME_MARKER_DOTNET | FRAME_MARKER_ERROR_BIT);
  }

  // code_start points to beginning of the JIT generated code. This is preceded by a CodeHeader
  // structure. The platforms we care define USE_INDIRECT_CODEHEADER, so the data is defined at:
  // https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/codeman.h#L246-L248
  // This just reads the single pointer to the RealCodeHeader.
  if (bpf_probe_read_user(
        &code_header_ptr, sizeof(code_header_ptr), (void *)code_start - sizeof(u64))) {
    DEBUG_PRINT("dotnet:  --> bad code header");
    increment_metric(metricID_UnwindDotnetErrCodeHeader);
    return ERR_DOTNET_CODE_HEADER;
  }
  type = DOTNET_CODE_JIT;

push_frame:
  DEBUG_PRINT(
    "dotnet:  --> code_start = %lx, code_header = %lx, pc_offset = %lx",
    (unsigned long)code_start,
    (unsigned long)code_header_ptr,
    (unsigned long)(pc - code_start));
  error = push_dotnet(trace, (code_header_ptr << 5) + type, pc - code_start, return_address);
  if (error) {
    return error;
  }

  increment_metric(metricID_UnwindDotnetFrames);
  return ERR_OK;
}

// unwind_dotnet is the entry point for tracing when invoked from the native tracer
// or interpreter dispatcher. It does not reset the trace object and will append the
// dotnet stack frames to the trace object for the current CPU.
static EBPF_INLINE int unwind_dotnet(struct pt_regs *ctx)
{
  PerCPURecord *record = get_per_cpu_record();
  if (!record) {
    return -1;
  }

  Trace *trace = &record->trace;
  u32 pid      = trace->pid;
  DEBUG_PRINT("==== unwind_dotnet %d ====", trace->stack_len);

  int unwinder       = PROG_UNWIND_STOP;
  ErrorCode error    = ERR_OK;
  DotnetProcInfo *vi = bpf_map_lookup_elem(&dotnet_procs, &pid);
  if (!vi) {
    DEBUG_PRINT("dotnet: no DotnetProcInfo for this pid");
    error = ERR_DOTNET_NO_PROC_INFO;
    increment_metric(metricID_UnwindDotnetErrNoProcInfo);
    goto exit;
  }

  record->ratelimitAction = RATELIMIT_ACTION_FAST;
  increment_metric(metricID_UnwindDotnetAttempts);

  UNROLL for (int i = 0; i < DOTNET_FRAMES_PER_PROGRAM; i++)
  {
    unwinder = PROG_UNWIND_STOP;

    error = unwind_one_dotnet_frame(record);
    if (error) {
      break;
    }

    error = get_next_unwinder_after_native_frame(record, &unwinder);
    if (error || unwinder != PROG_UNWIND_DOTNET) {
      break;
    }
  }

exit:
  record->state.unwind_error = error;
  tail_call(ctx, unwinder);
  DEBUG_PRINT("dotnet: tail call for next frame unwinder (%d) failed", unwinder);
  return -1;
}
MULTI_USE_FUNC(unwind_dotnet)
