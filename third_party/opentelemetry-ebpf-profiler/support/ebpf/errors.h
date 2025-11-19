/* WARNING: this file is auto-generated, DO NOT CHANGE MANUALLY */

#ifndef OPTI_ERRORS_H
#define OPTI_ERRORS_H

typedef enum ErrorCode {
  // Sentinel value for success: not actually an error
  ERR_OK = 0,

  // Entered code that was believed to be unreachable
  ERR_UNREACHABLE = 1,

  // The stack trace has reached its maximum length and could not be unwound further
  ERR_STACK_LENGTH_EXCEEDED = 2,

  // The trace stack was empty after unwinding completed
  ERR_EMPTY_STACK = 3,

  // Deprecated: Failed to lookup entry in the per-CPU frame list
  ERR_LOOKUP_PER_CPU_FRAME_LIST = 4,

  // Maximum number of tail calls was reached
  ERR_MAX_TAIL_CALLS = 5,

  // Hotspot: Failure to get CodeBlob address (no heap or bad segmap)
  ERR_HOTSPOT_NO_CODEBLOB = 1000,

  // Hotspot: Failure to unwind interpreter due to invalid FP
  ERR_HOTSPOT_INTERPRETER_FP = 1001,

  // Hotspot: Failure to unwind because return address was not found with heuristic
  ERR_HOTSPOT_INVALID_RA = 1002,

  // Hotspot: Failure to get codeblob data or matching it to current unwind state
  ERR_HOTSPOT_INVALID_CODEBLOB = 1003,

  // Hotspot: Unwind instructions requested LR unwinding mid-trace (nonsensical)
  ERR_HOTSPOT_LR_UNWINDING_MID_TRACE = 1004,

  // Python: Unable to read current PyCodeObject
  ERR_PYTHON_BAD_CODE_OBJECT_ADDR = 2000,

  // Python: No entry for this process exists in the Python process info array
  ERR_PYTHON_NO_PROC_INFO = 2001,

  // Python: Unable to read current PyFrameObject
  ERR_PYTHON_BAD_FRAME_OBJECT_ADDR = 2002,

  // Python: Unable to read _PyCFrame.current_frame
  ERR_PYTHON_BAD_CFRAME_CURRENT_FRAME_ADDR = 2003,

  // Python: Unable to read the thread state pointer from TLD
  ERR_PYTHON_READ_THREAD_STATE_ADDR = 2004,

  // Python: The thread state pointer read from TSD is zero
  ERR_PYTHON_ZERO_THREAD_STATE = 2005,

  // Python: Unable to read the frame pointer from the thread state object
  ERR_PYTHON_BAD_THREAD_STATE_FRAME_ADDR = 2006,

  // Python: Unable to read autoTLSkey
  ERR_PYTHON_BAD_AUTO_TLS_KEY_ADDR = 2007,

  // Python: Unable to determine the base address for thread-specific data
  ERR_PYTHON_READ_TSD_BASE = 2008,

  // Ruby: No entry for this process exists in the Ruby process info array
  ERR_RUBY_NO_PROC_INFO = 3000,

  // Ruby: Unable to read the stack pointer from the Ruby context
  ERR_RUBY_READ_STACK_PTR = 3001,

  // Ruby: Unable to read the size of the VM stack from the Ruby context
  ERR_RUBY_READ_STACK_SIZE = 3002,

  // Ruby: Unable to read the control frame pointer from the Ruby context
  ERR_RUBY_READ_CFP = 3003,

  // Ruby: Unable to read the expression path from the Ruby frame
  ERR_RUBY_READ_EP = 3004,

  // Ruby: Unable to read instruction sequence body
  ERR_RUBY_READ_ISEQ_BODY = 3005,

  // Ruby: Unable to read the instruction sequence encoded size
  ERR_RUBY_READ_ISEQ_ENCODED = 3006,

  // Ruby: Unable to read the instruction sequence size
  ERR_RUBY_READ_ISEQ_SIZE = 3007,

  // Native: Unable to find the code section in the stack delta page info map
  ERR_NATIVE_LOOKUP_TEXT_SECTION = 4000,

  // Native: Unable to look up the outer stack delta map (invalid map ID)
  ERR_NATIVE_LOOKUP_STACK_DELTA_OUTER_MAP = 4001,

  // Native: Unable to look up the inner stack delta map (unknown text section ID)
  ERR_NATIVE_LOOKUP_STACK_DELTA_INNER_MAP = 4002,

  // Native: Exceeded the maximum number of binary search steps during stack delta lookup
  ERR_NATIVE_EXCEEDED_DELTA_LOOKUP_ITERATIONS = 4003,

  // Native: Unable to look up the stack delta from the inner map
  ERR_NATIVE_LOOKUP_RANGE = 4004,

  // Native: The stack delta read from the delta map is marked as invalid
  ERR_NATIVE_STACK_DELTA_INVALID = 4005,

  // Native: The stack delta read from the delta map is a stop record
  ERR_NATIVE_STACK_DELTA_STOP = 4006,

  // Native: Unable to read the next instruction pointer from memory
  ERR_NATIVE_PC_READ = 4007,

  // Native: Unwind instructions requested LR unwinding mid-trace (nonsensical)
  ERR_NATIVE_LR_UNWINDING_MID_TRACE = 4008,

  // Native: Unable to read the kernel-mode registers
  ERR_NATIVE_READ_KERNELMODE_REGS = 4009,

  // Native: Unable to read the IRQ stack link
  ERR_NATIVE_CHASE_IRQ_STACK_LINK = 4010,

  // Native: Unexpectedly encountered a kernel mode pointer while attempting to unwind user-mode
  // stack
  ERR_NATIVE_UNEXPECTED_KERNEL_ADDRESS = 4011,

  // Native: Unable to locate the PID page mapping for the current instruction pointer
  ERR_NATIVE_NO_PID_PAGE_MAPPING = 4012,

  // Native: Unexpectedly encountered a instruction pointer of zero
  ERR_NATIVE_ZERO_PC = 4013,

  // Native: The instruction pointer is too small to be valid
  ERR_NATIVE_SMALL_PC = 4014,

  // Native: Encountered an invalid unwind_info_array index
  ERR_NATIVE_BAD_UNWIND_INFO_INDEX = 4015,

  // Native: Code is running in ARM 32-bit compat mode.
  ERR_NATIVE_AARCH64_32BIT_COMPAT_MODE = 4016,

  // Native: Code is running in x86_64 32-bit compat mode.
  ERR_NATIVE_X64_32BIT_COMPAT_MODE = 4017,

  // V8: Encountered a bad frame pointer during V8 unwinding
  ERR_V8_BAD_FP = 5000,

  // V8: The JavaScript function object read from memory is invalid
  ERR_V8_BAD_JS_FUNC = 5001,

  // V8: No entry for this process exists in the V8 process info array
  ERR_V8_NO_PROC_INFO = 5002,

  // Dotnet: No entry for this process exists in the dotnet process info array
  ERR_DOTNET_NO_PROC_INFO = 6000,

  // Dotnet: Encountered a bad frame pointer during dotnet unwinding
  ERR_DOTNET_BAD_FP = 6001,

  // Dotnet: Failed to find or read CodeHeader
  ERR_DOTNET_CODE_HEADER = 6002,

  // Dotnet: Code object was too large to unwind in eBPF
  ERR_DOTNET_CODE_TOO_LARGE = 6003
} ErrorCode;

#endif // OPTI_ERRORS_H
