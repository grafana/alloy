#ifndef OPTI_STACKDELTATYPES_H
#define OPTI_STACKDELTATYPES_H

// Command without arguments, the argument is instead an UNWIND_COMMAND_* value
#define UNWIND_OPCODE_COMMAND  0x00
// Expression with base value being the Canonical Frame Address (CFA)
#define UNWIND_OPCODE_BASE_CFA 0x01
// Expression with base value being the Stack Pointer
#define UNWIND_OPCODE_BASE_SP  0x02
// Expression with base value being the Frame Pointer
#define UNWIND_OPCODE_BASE_FP  0x03
// Expression with base value being the Link Register (ARM64)
#define UNWIND_OPCODE_BASE_LR  0x04
// Expression with base value being a Generic Register
#define UNWIND_OPCODE_BASE_REG 0x05
// An opcode flag to indicate that the value should be dereferenced
#define UNWIND_OPCODEF_DEREF   0x80

// Unsupported or no value for the register
#define UNWIND_COMMAND_INVALID       0
// For CFA: stop unwinding, this function is a stack root function
#define UNWIND_COMMAND_STOP          1
// Unwind a PLT entry
#define UNWIND_COMMAND_PLT           2
// Unwind a signal frame
#define UNWIND_COMMAND_SIGNAL        3
// Unwind using standard frame pointer
#define UNWIND_COMMAND_FRAME_POINTER 4

// If opcode has UNWIND_OPCODEF_DEREF set, the lowest bits of 'param' are used
// as second adder as post-deref operation. This contains the mask for that.
// This assumes that stack and CFA are aligned to register size, so that the
// lowest bits of the offsets are always unset.
#define UNWIND_DEREF_MASK 7

// The argument after dereference is multiplied by this to allow some range.
// This assumes register size offsets are used.
#define UNWIND_DEREF_MULTIPLIER 8

// For the UNWIND_OPCODE_BASE_REG, the bitmask reserved for the register
// number. Remaining bits are the offset.
#define UNWIND_REG_MASK 15

#endif
