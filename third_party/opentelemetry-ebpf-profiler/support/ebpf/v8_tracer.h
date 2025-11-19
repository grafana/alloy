// This file contains definitions for the V8 tracer

// V8 constants for the tags. Hard coded to optimize code size and speed.
// They are unlikely to change, and likely require larger modifications on change.

// https://chromium.googlesource.com/v8/v8.git/+/refs/heads/9.2.230/include/v8-internal.h#52
#define V8_SmiTag            0x0
// https://chromium.googlesource.com/v8/v8.git/+/refs/heads/9.2.230/include/v8-internal.h#54
#define V8_SmiTagMask        0x1
// https://chromium.googlesource.com/v8/v8.git/+/refs/heads/9.2.230/include/v8-internal.h#91
#define V8_SmiTagShift       1
// https://chromium.googlesource.com/v8/v8.git/+/refs/heads/9.2.230/include/v8-internal.h#98
#define V8_SmiValueShift     32
// https://chromium.googlesource.com/v8/v8.git/+/refs/heads/9.2.230/include/v8-internal.h#39
#define V8_HeapObjectTag     0x1
// https://chromium.googlesource.com/v8/v8.git/+/refs/heads/9.2.230/include/v8-internal.h#42
#define V8_HeapObjectTagMask 0x3

// The Trace 'file' field is split to object pointer (aligned to 8 bytes),
// and the zero bits due to alignment are re-used as the following flags.
#define V8_FILE_TYPE_MARKER        0x0
#define V8_FILE_TYPE_BYTECODE      0x1
#define V8_FILE_TYPE_NATIVE_SFI    0x2
#define V8_FILE_TYPE_NATIVE_CODE   0x3
#define V8_FILE_TYPE_NATIVE_JSFUNC 0x4
#define V8_FILE_TYPE_MASK          0x7

// The Trace 'line' field is split to two 32-bit fields: cookie and PC-delta
#define V8_LINE_COOKIE_SHIFT 32
#define V8_LINE_COOKIE_MASK  0xffffffff00000000
#define V8_LINE_DELTA_MASK   0x00000000ffffffff
