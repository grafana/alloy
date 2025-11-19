// This file contains the code and map definitions for the Python tracer

#include "bpfdefs.h"
#include "errors.h"
#include "tracemgmt.h"
#include "tsd.h"
#include "types.h"

// The number of Python frames to unwind per frame-unwinding eBPF program. If
// we start running out of instructions in the walk_python_stack program, one
// option is to adjust this number downwards.
#define FRAMES_PER_WALK_PYTHON_STACK 64

// Forward declaration to avoid warnings like
// "declaration of 'struct pt_regs' will not be visible outside of this function [-Wvisibility]".
struct pt_regs;

// Map from Python process IDs to a structure containing addresses of variables
// we require in order to build the stack trace
struct py_procs_t {
  __uint(type, BPF_MAP_TYPE_HASH);
  __type(key, pid_t);
  __type(value, PyProcInfo);
  __uint(max_entries, 1024);
} py_procs SEC(".maps");

// Record a Python frame
static EBPF_INLINE ErrorCode push_python(Trace *trace, u64 file, u64 line)
{
  return _push(trace, file, line, FRAME_MARKER_PYTHON);
}

static EBPF_INLINE u64 py_encode_lineno(u32 object_id, u32 f_lasti)
{
  return (object_id | (((u64)f_lasti) << 32));
}

static EBPF_INLINE ErrorCode process_python_frame(
  PerCPURecord *record,
  const PyProcInfo *pyinfo,
  void **py_frameobjectptr,
  bool *continue_with_next)
{
  Trace *trace               = &record->trace;
  const void *py_frameobject = *py_frameobjectptr;
  u64 lineno = FUNC_TYPE_UNKNOWN, file_id = UNKNOWN_FILE;
  u32 codeobject_id;

  *continue_with_next = false;

  // Vars used in extracting data from the Python interpreter
  PythonUnwindScratchSpace *pss = &record->pythonUnwindScratch;

  // Make verifier happy for PyFrameObject offsets
  if (
    pyinfo->PyFrameObject_f_code > sizeof(pss->frame) - sizeof(void *) ||
    pyinfo->PyFrameObject_f_back > sizeof(pss->frame) - sizeof(void *) ||
    pyinfo->PyFrameObject_f_lasti > sizeof(pss->frame) - sizeof(u64) ||
    pyinfo->PyFrameObject_entry_member > sizeof(pss->frame) - sizeof(u8)) {
    return ERR_UNREACHABLE;
  }

  // Read PyFrameObject
  if (bpf_probe_read_user(pss->frame, sizeof(pss->frame), py_frameobject)) {
    DEBUG_PRINT("Failed to read PyFrameObject 0x%lx", (unsigned long)py_frameobject);
    increment_metric(metricID_UnwindPythonErrBadFrameCodeObjectAddr);
    return ERR_PYTHON_BAD_FRAME_OBJECT_ADDR;
  }

  void *py_codeobject = *(void **)(&pss->frame[pyinfo->PyFrameObject_f_code]);
  *py_frameobjectptr  = *(void **)(&pss->frame[pyinfo->PyFrameObject_f_back]);

  // Stop unwinding if `f_executable` is None. See comment when getting the
  // ´noneStruct´ address in python.go for details.
  if (pyinfo->version >= 0x30d && py_codeobject == (void *)pyinfo->noneStructAddr) {
    *continue_with_next = true;
    return ERR_OK;
  }

  // See experiments/python/README.md for a longer version of this. In short, we
  // cannot directly obtain the correct Python line number. It has to be calculated
  // using information found in the PyCodeObject for the current frame. This
  // calculation involves iterating over potentially unbounded data, and so we don't
  // want to do it in eBPF. Instead, we log the bytecode instruction that is being
  // executed, and then convert this to a line number in the user-land component.
  // Bytecode instructions are identified as an offset within a code object. The
  // offset is easy to retrieve (PyFrameObject->f_lasti). Code objects are a little
  // more tricky. We need to log enough information to uniquely identify the code
  // object for the current frame, so that in the user-land component we can load
  // it from the .pyc. There is no unique identifier for code objects though, so we
  // try to construct one below by hashing together a few fields. These fields are
  // selected in the *hope* that no collisions occur between code objects.

  int py_f_lasti = 0;
  if (pyinfo->version >= 0x030b) {
    // With Python 3.11 the element f_lasti not only got renamed but also its
    // type changed from int to a _Py_CODEUNIT* and needs to be translated to lastI.
    // It is a direct pointer to the bytecode, so calculate the byte code index.
    // sizeof(_Py_CODEUNIT) == 2.
    // https://github.com/python/cpython/commit/ef6a482b0285870c45f39c9b17ed827362b334ae
    u64 prev_instr = *(u64 *)(&pss->frame[pyinfo->PyFrameObject_f_lasti]);
    s64 instr_diff = (s64)prev_instr - (s64)py_codeobject - pyinfo->PyCodeObject_sizeof;
    if (instr_diff < -2 || instr_diff > 0x10000000)
      instr_diff = -2;
    py_f_lasti = (int)instr_diff >> 1;

    // Python 3.11+ the frame object has some field that can be used to determine
    // if this is the last frame in the interpreter loop. This generalized test
    // works on 3.11 and 3.12 though the actual struct members are different.
    if (
      *(u8 *)(&pss->frame[pyinfo->PyFrameObject_entry_member]) == pyinfo->PyFrameObject_entry_val) {
      *continue_with_next = true;
    }
  } else {
    py_f_lasti = *(int *)(&pss->frame[pyinfo->PyFrameObject_f_lasti]);
  }

  if (!py_codeobject) {
    DEBUG_PRINT(
      "Null codeobject for PyFrameObject 0x%lx 0x%lx",
      (unsigned long)py_frameobject,
      (unsigned long)(py_frameobject + pyinfo->PyFrameObject_f_code));
    increment_metric(metricID_UnwindPythonZeroFrameCodeObject);
    goto push_frame;
  }

  // Make verifier happy for PyCodeObject offsets
  if (
    pyinfo->PyCodeObject_co_argcount > sizeof(pss->code) - sizeof(int) ||
    pyinfo->PyCodeObject_co_kwonlyargcount > sizeof(pss->code) - sizeof(int) ||
    pyinfo->PyCodeObject_co_flags > sizeof(pss->code) - sizeof(int) ||
    pyinfo->PyCodeObject_co_firstlineno > sizeof(pss->code) - sizeof(int)) {
    return ERR_UNREACHABLE;
  }

  // Read PyCodeObject
  if (bpf_probe_read_user(pss->code, sizeof(pss->code), py_codeobject)) {
    DEBUG_PRINT("Failed to read PyCodeObject at 0x%lx", (unsigned long)(py_codeobject));
    increment_metric(metricID_UnwindPythonErrBadCodeObjectArgCountAddr);
    return ERR_PYTHON_BAD_CODE_OBJECT_ADDR;
  }

  int py_argcount       = *(int *)(&pss->code[pyinfo->PyCodeObject_co_argcount]);
  int py_kwonlyargcount = *(int *)(&pss->code[pyinfo->PyCodeObject_co_kwonlyargcount]);
  int py_flags          = *(int *)(&pss->code[pyinfo->PyCodeObject_co_flags]);
  int py_firstlineno    = *(int *)(&pss->code[pyinfo->PyCodeObject_co_firstlineno]);

  codeobject_id =
    (py_argcount << 25) + (py_kwonlyargcount << 18) + (py_flags << 10) + py_firstlineno;

  file_id = (u64)py_codeobject;
  lineno  = py_encode_lineno(codeobject_id, (u32)py_f_lasti);

push_frame:
  DEBUG_PRINT("Pushing Python %lx %lu", (unsigned long)file_id, (unsigned long)lineno);
  ErrorCode error = push_python(trace, file_id, lineno);
  if (error) {
    DEBUG_PRINT("failed to push python frame");
    return error;
  }
  increment_metric(metricID_UnwindPythonFrames);
  return ERR_OK;
}

static EBPF_INLINE ErrorCode
walk_python_stack(PerCPURecord *record, const PyProcInfo *pyinfo, int *unwinder)
{
  void *py_frame  = record->pythonUnwindState.py_frame;
  ErrorCode error = ERR_OK;
  *unwinder       = PROG_UNWIND_STOP;

  UNROLL for (u32 i = 0; i < FRAMES_PER_WALK_PYTHON_STACK; ++i)
  {
    bool continue_with_next;
    error = process_python_frame(record, pyinfo, &py_frame, &continue_with_next);
    if (error) {
      goto stop;
    }
    if (continue_with_next && pyinfo->continue_with_next_unwinder) {
      *unwinder = get_next_unwinder_after_interpreter();
      goto stop;
    }
    if (!py_frame) {
      goto stop;
    }
  }

  *unwinder = PROG_UNWIND_PYTHON;

stop:
  // Set up the state for the next invocation of this unwinding program.
  if (error || !py_frame) {
    unwinder_mark_done(record, PROG_UNWIND_PYTHON);
  }
  record->pythonUnwindState.py_frame = py_frame;
  return error;
}

// get_PyThreadState retrieves the PyThreadState* for the current thread.
//
// Python sets the thread_state using pthread_setspecific with the key
// stored in a global variable autoTLSkey.
static EBPF_INLINE ErrorCode get_PyThreadState(
  const PyProcInfo *pyinfo, void *tsd_base, void *autoTLSkeyAddr, void **thread_state)
{
  int key;
  if (bpf_probe_read_user(&key, sizeof(key), autoTLSkeyAddr)) {
    DEBUG_PRINT("Failed to read autoTLSkey from 0x%lx", (unsigned long)autoTLSkeyAddr);
    increment_metric(metricID_UnwindPythonErrBadAutoTlsKeyAddr);
    return ERR_PYTHON_BAD_AUTO_TLS_KEY_ADDR;
  }

  if (tsd_read(&pyinfo->tsdInfo, tsd_base, key, thread_state)) {
    increment_metric(metricID_UnwindPythonErrReadThreadStateAddr);
    return ERR_PYTHON_READ_THREAD_STATE_ADDR;
  }

  return ERR_OK;
}

static EBPF_INLINE ErrorCode get_PyFrame(const PyProcInfo *pyinfo, void **frame)
{
  void *tsd_base;
  if (tsd_get_base(&tsd_base)) {
    DEBUG_PRINT("Failed to get TSD base address");
    increment_metric(metricID_UnwindPythonErrReadTsdBase);
    return ERR_PYTHON_READ_TSD_BASE;
  }
  DEBUG_PRINT(
    "TSD Base 0x%lx, autoTLSKeyAddr 0x%lx",
    (unsigned long)tsd_base,
    (unsigned long)pyinfo->autoTLSKeyAddr);

  // Get the PyThreadState from TSD
  void *py_tsd_thread_state;
  ErrorCode error =
    get_PyThreadState(pyinfo, tsd_base, (void *)pyinfo->autoTLSKeyAddr, &py_tsd_thread_state);
  if (error) {
    return error;
  }

  if (!py_tsd_thread_state) {
    DEBUG_PRINT("PyThreadState is 0x0");
    increment_metric(metricID_UnwindPythonErrZeroThreadState);
    return ERR_PYTHON_ZERO_THREAD_STATE;
  }

  if (pyinfo->version >= 0x30b) {
    // Starting with 3.11 we have to do an additional step to get to _PyInterpreterFrame, formerly
    // known as PyFrameObject.

    // Get PyThreadState.cframe
    void *cframe_ptr;
    if (bpf_probe_read_user(
          &cframe_ptr, sizeof(void *), py_tsd_thread_state + pyinfo->PyThreadState_frame)) {
      DEBUG_PRINT(
        "Failed to read PyThreadState.cframe at 0x%lx",
        (unsigned long)(py_tsd_thread_state + pyinfo->PyThreadState_frame));
      increment_metric(metricID_UnwindPythonErrBadThreadStateFrameAddr);
      return ERR_PYTHON_BAD_THREAD_STATE_FRAME_ADDR;
    }

    // Get _PyCFrame.current_frame.
    //
    // Starting with Python 3.13, the indirection through _PyCFrame is removed.
    // See commit 006e44f9 in the CPython repo.
    if (pyinfo->version < 0x30d) {
      if (bpf_probe_read_user(frame, sizeof(void *), cframe_ptr + pyinfo->PyCFrame_current_frame)) {
        DEBUG_PRINT(
          "Failed to read _PyCFrame.current_frame at 0x%lx",
          (unsigned long)(cframe_ptr + pyinfo->PyCFrame_current_frame));
        increment_metric(metricID_UnwindPythonErrBadCFrameFrameAddr);
        return ERR_PYTHON_BAD_CFRAME_CURRENT_FRAME_ADDR;
      }
    } else {
      *frame = cframe_ptr;
    }
  } else {
    // Get PyThreadState.frame
    if (bpf_probe_read_user(
          frame, sizeof(void *), py_tsd_thread_state + pyinfo->PyThreadState_frame)) {
      DEBUG_PRINT(
        "Failed to read PyThreadState.frame at 0x%lx",
        (unsigned long)(py_tsd_thread_state + pyinfo->PyThreadState_frame));
      increment_metric(metricID_UnwindPythonErrBadThreadStateFrameAddr);
      return ERR_PYTHON_BAD_THREAD_STATE_FRAME_ADDR;
    }
  }

  return ERR_OK;
}

// unwind_python is the entry point for tracing when invoked from the native tracer
// or interpreter dispatcher. It does not reset the trace object and will append the
// Python stack frames to the trace object for the current CPU.
static EBPF_INLINE int unwind_python(struct pt_regs *ctx)
{
  PerCPURecord *record = get_per_cpu_record();
  if (!record)
    return -1;

  ErrorCode error = ERR_OK;
  int unwinder    = get_next_unwinder_after_interpreter();
  Trace *trace    = &record->trace;
  u32 pid         = trace->pid;

  DEBUG_PRINT("unwind_python()");

  const PyProcInfo *pyinfo = bpf_map_lookup_elem(&py_procs, &pid);
  if (!pyinfo) {
    // Not a Python process that we have info on
    DEBUG_PRINT("Can't build Python stack, no address info");
    increment_metric(metricID_UnwindPythonErrNoProcInfo);
    return ERR_PYTHON_NO_PROC_INFO;
  }

  DEBUG_PRINT("Building Python stack for 0x%x", pyinfo->version);
  if (!record->pythonUnwindState.py_frame) {
    increment_metric(metricID_UnwindPythonAttempts);
    error = get_PyFrame(pyinfo, &record->pythonUnwindState.py_frame);
    if (error) {
      goto exit;
    }
  }
  if (!record->pythonUnwindState.py_frame) {
    DEBUG_PRINT("  -> Python frames are handled");
    unwinder_mark_done(record, PROG_UNWIND_PYTHON);
    goto exit;
  }

  error = walk_python_stack(record, pyinfo, &unwinder);

exit:
  record->state.unwind_error = error;
  tail_call(ctx, unwinder);
  return -1;
}
MULTI_USE_FUNC(unwind_python)
