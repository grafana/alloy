// This file contains the code and map definitions for the Perl tracer

// Read the interpreterperl.go for generic discussion of the Perl VM.
//
// The trace is extracted from the Perl Context Stack (see perlguts for explanation).
// Basically this stack contains a node for each sub/function/method/regexp/block
// that requires tracking VM context state. The unwinder simply walks these structures
// from top to bottom. During the walk we do two things:
//  1) parse all 'sub' nodes, which represent a function entry point. This node has
//     available the activated function's name derived from the runtime object stash.
//     The unwinder will resolve this CV object to the canonical EGV (Effective GV) it
//     refers to.
//     Note: there is no information about where this 'sub' was defined in, or where
//     the execution inside it, is currently. The file/line is then taken from the first
//     COP seen earlier. See next step.
//  2) parse all nodes of 'block' type (includes also the 'sub' nodes) and records
//     deepest available "oldcop" field which basically is the pointer to the "COP"
//     (Control OPS) structure. COP is basically the source file AST parse node for
//     an expression. It contains the filename and line number of this expression,
//     and its where we extract the current source file/line from.
//
// So, when walking the Context Stack, we first expect to see a 'COP' and store it.
// Additional less deep COPs might be seen and ignored. A 'sub' entry indicates
// a function boundary, which is then recorded as the stack frame along with
// the outmost COP seen earlier for the file/line information.
//
// The unwinder will also synchronize so that the context walking is stopped and
// native unwinding is continued based on what the Perl Context Stack indicates.
// This allows synchronizing the Perl functions to the right position in the trace.

#include "bpfdefs.h"
#include "tracemgmt.h"
#include "tsd.h"
#include "types.h"

// The number of Perl frames to unwind per frame-unwinding eBPF program.
#define PERL_FRAMES_PER_PROGRAM 12

// PERL SI types definitions
// https://github.com/Perl/perl5/blob/v5.32.0/cop.h#L1017-L1035
#define PERLSI_MAIN 1

// PERL_CONTEXT type definitions
// https://github.com/Perl/perl5/blob/v5.32.0/cop.h#L886-L909
#define CXTYPEMASK 0xf
#define CXt_SUB    9
#define CXt_FORMAT 10
#define CXt_EVAL   11
#define CXt_SUBST  12

// Flags for CXt_SUB (and FORMAT)
// https://github.com/Perl/perl5/blob/v5.32.0/cop.h#L912-L917
#define CXp_SUB_RE_FAKE 0x80

// Scalar Value types (SVt)
// https://github.com/Perl/perl5/blob/v5.32.0/sv.h#L132-L166
#define SVt_MASK 0x1f
#define SVt_PVGV 9
#define SVt_PVCV 13

// https://github.com/Perl/perl5/blob/v5.32.0/sv.h#L375-L377
#define SVpgv_GP 0x00008000

// Code Value flags (CVf)
// https://github.com/Perl/perl5/blob/v5.32.0/cv.h#L115-L140
#define CVf_NAMED 0x8000

// Map from Perl process IDs to a structure containing addresses of variables
// we require in order to build the stack trace
struct perl_procs_t {
  __uint(type, BPF_MAP_TYPE_HASH);
  __type(key, pid_t);
  __type(value, PerlProcInfo);
  __uint(max_entries, 1024);
} perl_procs SEC(".maps");

// Record a Perl frame
static EBPF_INLINE ErrorCode push_perl(Trace *trace, u64 file, u64 line)
{
  DEBUG_PRINT("Pushing perl frame cop=0x%lx, cv=0x%lx", (unsigned long)file, (unsigned long)line);
  return _push(trace, file, line, FRAME_MARKER_PERL);
}

// resolve_cv_egv() takes in a CV* and follows the pointers to resolve this CV's
// EGV to be reported for HA. This basically maps the internal code value, to its
// canonical symbol name. This mapping is done in EBPF because it seems the CV*
// can get undefined once it goes out of scope, but the EGV should be more permanent.
static EBPF_INLINE void *resolve_cv_egv(const PerlProcInfo *perlinfo, const void *cv)
{
  // First check the CV's type
  u32 cv_flags;
  if (bpf_probe_read_user(&cv_flags, sizeof(cv_flags), cv + perlinfo->sv_flags)) {
    goto err;
  }

  if ((cv_flags & SVt_MASK) != SVt_PVCV) {
    DEBUG_PRINT("CV is not a PVCV, flags 0x%x", cv_flags);
    return 0;
  }

  // Follow the any pointer for the XPVCV body
  void *xpvcv;
  if (bpf_probe_read_user(&xpvcv, sizeof(xpvcv), cv + perlinfo->sv_any)) {
    goto err;
  }

  u32 xcv_flags;
  if (bpf_probe_read_user(&xcv_flags, sizeof(xcv_flags), xpvcv + perlinfo->xcv_flags)) {
    goto err;
  }

  if ((xcv_flags & CVf_NAMED) == CVf_NAMED) {
    // NAMED CVs are created when a function gets undefined, but someone is
    // still holding reference to them. Perl VM should ensure that these are
    // not seen in the Context Stack.
    DEBUG_PRINT("Unexpected NAMED CV, flags 0x%x/0x%x", cv_flags, xcv_flags);
    return 0;
  }

  // At this point we have CV with GV (symbol). This is expected of all seen CVs
  // inside the Context Stack.
  void *gv;
  if (bpf_probe_read_user(&gv, sizeof(gv), xpvcv + perlinfo->xcv_gv) || !gv) {
    goto err;
  }

  DEBUG_PRINT("Found GV at 0x%lx", (unsigned long)gv);

  // Make sure we read GV with a GP
  u32 gv_flags;
  if (bpf_probe_read_user(&gv_flags, sizeof(gv_flags), gv + perlinfo->sv_flags)) {
    goto err;
  }

  if ((gv_flags & (SVt_MASK | SVpgv_GP)) != (SVt_PVGV | SVpgv_GP)) {
    // Perl VM should also ensure that we see only GV-with-GP type variables
    // via the Context stack.
    DEBUG_PRINT("Unexpected GV-without-GP, flags 0x%x", gv_flags);
    return 0;
  }

  // Follow GP pointer
  void *gp;
  if (bpf_probe_read_user(&gp, sizeof(gp), gv + perlinfo->svu_gp)) {
    goto err;
  }

  // Read the Effective GV (EGV) from the GP to be reported for HA
  void *egv;
  if (bpf_probe_read_user(&egv, sizeof(egv), gp + perlinfo->gp_egv)) {
    goto err;
  }

  if (egv) {
    DEBUG_PRINT("Found EGV at 0x%lx", (unsigned long)egv);
    return egv;
  }
  return gv;

err:
  DEBUG_PRINT("Bad bpf_probe_read_user() in resolve_cv_egv");
  increment_metric(metricID_UnwindPerlResolveEGV);
  return 0;
}

static EBPF_INLINE int
process_perl_frame(PerCPURecord *record, const PerlProcInfo *perlinfo, const void *cx)
{
  Trace *trace = &record->trace;
  int unwinder = PROG_UNWIND_PERL;

  // Per S_dopoptosub_at() we are interested only in specific SUB/FORMAT
  // context entries. Others are non-functions, or helper entries.
  // https://github.com/Perl/perl5/blob/v5.32.0/pp_ctl.c#L1432-L1462
  u8 type;
  if (bpf_probe_read_user(&type, sizeof(type), cx + perlinfo->context_type)) {
    goto err;
  }

  DEBUG_PRINT("Got perl cx 0x%x", type);
  switch (type & CXTYPEMASK) {
  case CXt_SUBST:
    // SUBST is special case, it is the only type using different union portion
    // of 'struct context' and does not have COP pointer in it.
    // Skip these completely.
    return unwinder;
  case CXt_SUB:
  case CXt_FORMAT:
    // FORMAT and SUB blocks are quite identical, and the ones we want to show
    // in the backtrace.

    // In sub foo { /(?{...})/ }, foo ends up on the CX stack twice; the first for
    // the normal foo() call, and the second for a faked up re-entry into the sub
    // to execute the code block. Hide this faked entry from the world like perl does.
    //   https://github.com/Perl/perl5/blob/v5.32.0/pp_ctl.c#L1432-L1462
    if (type & CXp_SUB_RE_FAKE) {
      return unwinder;
    }

    if (get_next_unwinder_after_interpreter() != PROG_UNWIND_STOP) {
      // If generating mixed traces, use 'sub_retop' to detect if this is the
      // C->Perl boundary. This is the value returned as next opcode at
      //   https://github.com/Perl/perl5/blob/v5.32.0/pp_hot.c#L4952-L4955
      // and then used by the mainloop to determine if it's time to exit and
      // return to the next native frame:
      //   https://github.com/Perl/perl5/blob/v5.32.0/run.c#L41
      u64 retop;
      if (bpf_probe_read_user(&retop, sizeof(retop), cx + perlinfo->context_blk_sub_retop)) {
        goto err;
      }
      if (retop == 0) {
        unwinder = get_next_unwinder_after_interpreter();
      }
    }

    // Extract the functions Code Value for symbolization
    void *cv;
    if (bpf_probe_read_user(&cv, sizeof(cv), cx + perlinfo->context_blk_sub_cv)) {
      goto err;
    }

    void *egv = resolve_cv_egv(perlinfo, cv);
    if (!egv) {
      goto err;
    }
    if (push_perl(trace, (u64)egv, (u64)record->perlUnwindState.cop) != ERR_OK) {
      return PROG_UNWIND_STOP;
    }
    record->perlUnwindState.cop = 0;
    break;
  default:
    // Some other block context type.
    break;
  }

  // Record the first valid COP from block contexts to determine current
  // line number inside the sub/format block.
  if (!record->perlUnwindState.cop) {
    if (bpf_probe_read_user(
          &record->perlUnwindState.cop,
          sizeof(record->perlUnwindState.cop),
          cx + perlinfo->context_blk_oldcop)) {
      goto err;
    }
    DEBUG_PRINT("COP from context stack 0x%lx", (unsigned long)record->perlUnwindState.cop);
  }
  return unwinder;

err:
  // Perl context stack topmost entry might be bogus: the item count is updated
  // first and the content is filled later. Thus there is small window to read
  // garbage values on the topmost entry. We likely get here for those entries.
  // Since this is known race, just continue reading the context stack if nothing
  // happened, and rest of the reads should be just fine.
  DEBUG_PRINT("Failed to read context stack entry at %p", cx);
  increment_metric(metricID_UnwindPerlReadContextStackEntry);
  return PROG_UNWIND_PERL;
}

static EBPF_INLINE void prepare_perl_stack(PerCPURecord *record, const PerlProcInfo *perlinfo)
{
  const void *si = record->perlUnwindState.stackinfo;
  // cxstack contains the base of the current context stack which is an array of PERL_CONTEXT
  // structures, while cxstack_ix is the index of the current frame within that stack.
  s32 cxix;
  void *cxstack;

  if (
    bpf_probe_read_user(&cxstack, sizeof(cxstack), si + perlinfo->si_cxstack) ||
    bpf_probe_read_user(&cxix, sizeof(cxix), si + perlinfo->si_cxix)) {
    DEBUG_PRINT("Failed to read stackinfo at 0x%lx", (unsigned long)si);
    unwinder_mark_done(record, PROG_UNWIND_PERL);
    increment_metric(metricID_UnwindPerlReadStackInfo);
    return;
  }

  DEBUG_PRINT("New stackinfo, cxbase 0x%lx, cxix %d", (unsigned long)cxstack, cxix);
  record->perlUnwindState.cxbase = cxstack;
  record->perlUnwindState.cxcur  = cxstack + cxix * perlinfo->context_sizeof;
}

static EBPF_INLINE int walk_perl_stack(PerCPURecord *record, const PerlProcInfo *perlinfo)
{
  const void *si = record->perlUnwindState.stackinfo;

  // If Perl stackinfo is not available, all frames have been processed, then
  // continue with native unwinding.
  if (!si) {
    return get_next_unwinder_after_interpreter();
  }

  int unwinder       = PROG_UNWIND_PERL;
  const void *cxbase = record->perlUnwindState.cxbase;
  UNROLL for (u32 i = 0; i < PERL_FRAMES_PER_PROGRAM; ++i)
  {
    // Test first the stack 'cxcur' validity. Some stacks can have 'cxix=-1'
    // when they are being constructed or ran.
    if (record->perlUnwindState.cxcur < cxbase) {
      // End of a stackinfo. Resume to native unwinder if it's active.
      break;
    }
    // Parse one context stack entry.
    unwinder = process_perl_frame(record, perlinfo, record->perlUnwindState.cxcur);
    record->perlUnwindState.cxcur -= perlinfo->context_sizeof;
    if (unwinder == PROG_UNWIND_STOP) {
      // Failed to read context stack entry.
      break;
    }
    increment_metric(metricID_UnwindPerlFrames);
    if (unwinder != PROG_UNWIND_PERL) {
      // Perl context frame which returns to next native frame.
      break;
    }
  }

  if (record->perlUnwindState.cxcur < cxbase) {
    // Current Perl context stack exhausted. Check if there's more to unwind.
    Trace *trace = &record->trace;

    // If we have still a valid COP cached, it should be reported as the root frame.
    // In this case we don't have valid function context, and this implies an anonymous
    // or global level code block (e.g. code in file not inside function).
    u64 cop = (u64)record->perlUnwindState.cop;
    if (cop) {
      DEBUG_PRINT("End of perl stack - pushing main 0x%lx", (unsigned long)cop);
      if (push_perl(trace, 0, cop) != ERR_OK) {
        return PROG_UNWIND_STOP;
      }
      record->perlUnwindState.cop = 0;
    }

    // If the current stackinfo is of type PERLSI_MAIN, we should stop unwinding
    // the context stack. Potential stackinfos below are not part of the real
    // Perl call stack.
    s32 type = 0;
    if (
      bpf_probe_read_user(&type, sizeof(type), si + perlinfo->si_type) || type == PERLSI_MAIN ||
      bpf_probe_read_user(&si, sizeof(si), si + perlinfo->si_next) || si == NULL) {
      // Stop walking stacks if main stack is finished, or something went wrong.
      DEBUG_PRINT("Perl stackinfos done");
      unwinder_mark_done(record, PROG_UNWIND_PERL);
    } else {
      DEBUG_PRINT("Perl next stackinfo: type %d", type);
      record->perlUnwindState.stackinfo = si;
      prepare_perl_stack(record, perlinfo);
    }
    unwinder = get_next_unwinder_after_interpreter();
  }

  // Stack completed. Prepare the next one.
  DEBUG_PRINT(
    "Perl unwind done, next stackinfo 0x%lx, 0x%lx 0x%lx",
    (unsigned long)si,
    (unsigned long)record->perlUnwindState.cxbase,
    (unsigned long)record->perlUnwindState.cxcur);
  return unwinder;
}

// unwind_perl is the entry point for tracing when invoked from the native tracer
// or interpreter dispatcher. It does not reset the trace object and will append the
// Perl stack frames to the trace object for the current CPU.
static EBPF_INLINE int unwind_perl(struct pt_regs *ctx)
{
  PerCPURecord *record = get_per_cpu_record();
  if (!record) {
    return -1;
  }

  Trace *trace = &record->trace;
  u32 pid      = trace->pid;
  DEBUG_PRINT("unwind_perl()");

  PerlProcInfo *perlinfo = bpf_map_lookup_elem(&perl_procs, &pid);
  if (!perlinfo) {
    DEBUG_PRINT("Can't build Perl stack, no address info");
    return 0;
  }

  int unwinder = get_next_unwinder_after_interpreter();
  DEBUG_PRINT("Building Perl stack for 0x%x", perlinfo->version);

  if (!record->perlUnwindState.stackinfo) {
    // First Perl main loop encountered. Extract first the Interpreter state.
    increment_metric(metricID_UnwindPerlAttempts);

    void *interpreter;
    if (perlinfo->stateInTSD) {
      void *tsd_base;
      if (tsd_get_base(&tsd_base)) {
        DEBUG_PRINT("Failed to get TSD base address");
        goto err_tsd;
      }

      int tsd_key;
      if (bpf_probe_read_user(&tsd_key, sizeof(tsd_key), (void *)perlinfo->stateAddr)) {
        DEBUG_PRINT("Failed to read tsdKey from 0x%lx", (unsigned long)perlinfo->stateAddr);
        goto err_tsd;
      }

      if (tsd_read(&perlinfo->tsdInfo, tsd_base, tsd_key, &interpreter)) {
      err_tsd:
        increment_metric(metricID_UnwindPerlTSD);
        goto exit;
      }

      DEBUG_PRINT("TSD Base 0x%lx, TSD Key %d", (unsigned long)tsd_base, tsd_key);
    } else {
      interpreter = (void *)perlinfo->stateAddr;
    }
    DEBUG_PRINT("PerlInterpreter 0x%lx", (unsigned long)interpreter);

    if (
      bpf_probe_read_user(
        &record->perlUnwindState.stackinfo,
        sizeof(record->perlUnwindState.stackinfo),
        (void *)interpreter + perlinfo->interpreter_curstackinfo) ||
      bpf_probe_read_user(
        &record->perlUnwindState.cop,
        sizeof(record->perlUnwindState.cop),
        (void *)interpreter + perlinfo->interpreter_curcop)) {
      DEBUG_PRINT("Failed to read interpreter state");
      increment_metric(metricID_UnwindPerlReadStackInfo);
      goto exit;
    }
    DEBUG_PRINT("COP from interpreter state 0x%lx", (unsigned long)record->perlUnwindState.cop);

    prepare_perl_stack(record, perlinfo);
  }

  // Unwind one call stack or unrolled length, and continue
  unwinder = walk_perl_stack(record, perlinfo);

exit:
  tail_call(ctx, unwinder);
  return -1;
}
MULTI_USE_FUNC(unwind_perl)
