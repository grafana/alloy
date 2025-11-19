// This file contains the code and map definitions for the Java Hotspot VM tracer
//
// Much of the code principles are derived from the Java's DTrace plugin:
// https://hg.openjdk.java.net/jdk-updates/jdk14u/file/default/src/java.base/solaris/native/libjvm_db/libjvm_db.c
// See also the host agent interpreterjvm.go for more references.

#include "bpfdefs.h"
#include "errors.h"
#include "tracemgmt.h"
#include "types.h"

// Information extracted from a JDK `CodeBlob` instance.
typedef struct CodeBlobInfo {
  // The start address of the CodeBlob.
  u64 address;
  // Value of the `CodeBlob::_code_start` field.
  u64 code_start;
  // Value of the `CodeBlob::_code_end` field.
  u64 code_end;
  // Value of the `nmethod::deopt_handler` field.
  // Only contains valid data if the CodeBlob is of `nmethod` type.
  u64 deopt_handler;
  // Determines the frame type. First 4 bytes of the string pointed to by `CodeBlob::_name`.
  u32 frame_type;
  // Value of the `nmethod::orig_pc_offset` field.
  // Only contains valid data if this CodeBlob is of `nmethod` type.
  u32 orig_pc_offset;
  // Value of the `CodeBlob::_frame_size` field.
  u32 frame_size;
  // Value of the `CodeBlob::_frame_complete_offset` field.
  u32 frame_comp;
  // Value of the `nmethod::compile_id` field.
  // Only contains valid data if this CodeBlob is of `nmethod` type.
  u32 compile_id;
} CodeBlobInfo;

// Context structure for information shared between all handlers in the HotSpot unwinder.
typedef struct HotspotUnwindInfo {
  u64 sp;
  u64 pc;
  u64 fp;
  // The value reported as the `file` field of the trace.
  u64 file;
  // The value reported as the `line` field of the trace.
  struct {
    // Subtype of the frame (JIT, interpreter).
    u8 subtype;
    // Either the delta between the code start and current PC (for compiled code) or the
    // bytecode index (for interpreted code).
    u32 pc_delta_or_bci;
    // Validation cookie for the stored pointer.
    // The value used here depends on the frame type.
    u32 ptr_check;
  } line;
} HotspotUnwindInfo;

// Returned by frame type handlers to decide how this frame should be unwound.
typedef enum HotspotUnwindAction {
  UA_UNWIND_INVALID,
#if defined(__aarch64__)
  UA_UNWIND_AARCH64_LR,
#endif
  UA_UNWIND_PC_ONLY,
  UA_UNWIND_FRAME_POINTER,
  UA_UNWIND_FP_PC,
  UA_UNWIND_FRAME,
  UA_UNWIND_REGS,
  UA_UNWIND_COMPLETE,
} HotspotUnwindAction;

// The number of hotspot frames to unwind per frame-unwinding eBPF program.
#define HOTSPOT_FRAMES_PER_PROGRAM 4

// The maximum number of HotSpot segmap lookup iterations. This is directly proportional
// to the size of JIT method code size. The longest sequence seen so far is from JDK8,
// and is 9 iterations. Include few extras.
#define HOTSPOT_SEGMAP_ITERATIONS 12

// The maximum number of JVM frame entries to search for a return address. In certain
// cases the JIT emits extra entries on the stack, and this controls the heuristic on
// how many extra entries are looked at. As reference the JVM async-profiler has similar
// heuristic and uses 7 slots on x86_64 (no search needed on aarch64).
#if defined(__x86_64__)
  #define HOTSPOT_RA_SEARCH_SLOTS 6
#endif

// The hotspot frame type is distinguished from the first 4 characters of the CodeBlob
// type name. This provides constants for the needed strings.
#define FRAMETYPE_nmethod        0x74656d6e // "nmethod"
#define FRAMETYPE_native_nmethod 0x6974616e // "native nmethod"
#define FRAMETYPE_Interpreter    0x65746e49 // "Interpreter"
#define FRAMETYPE_vtable_chunks  0x62617476 // "vtable chunks"

struct hotspot_procs_t {
  __uint(type, BPF_MAP_TYPE_HASH);
  __type(key, pid_t);
  __type(value, HotspotProcInfo);
  // This is the maximum number of JVM processes. Few machines should ever exceed 256 simultaneous
  // JVMs running. Increase this value if 256 turns out to be insufficient.
  __uint(max_entries, 256);
} hotspot_procs SEC(".maps");

// Record a HotSpot frame
static EBPF_INLINE ErrorCode push_hotspot(Trace *trace, u64 file, u64 line, bool return_address)
{
  return _push_with_return_address(trace, file, line, FRAME_MARKER_HOTSPOT, return_address);
}

// calc_line merges the three values to be encoded in a frame 'line'
static EBPF_INLINE u64 calc_line(u8 subtype, u32 pc_or_bci, u32 ptr_check)
{
  return ((u64)subtype << 60) | ((u64)pc_or_bci << 32) | (u64)ptr_check;
}

#ifdef __x86_64__
// hotspot_addr_in_codecache checks if given address belongs to the JVM JIT code cache
static EBPF_INLINE bool hotspot_addr_in_codecache(u32 pid, u64 addr)
{
  PIDPage key   = {};
  key.prefixLen = BIT_WIDTH_PID + BIT_WIDTH_PAGE;
  key.pid       = __constant_cpu_to_be32(pid);
  key.page      = __constant_cpu_to_be64(addr);

  // Check if we have the data for this virtual address
  PIDPageMappingInfo *val = bpf_map_lookup_elem(&pid_page_to_mapping_info, &key);
  if (!val) {
    return false;
  }

  // The address is valid only if it is hotspot unwindable code.
  int program;
  u64 bias;
  decode_bias_and_unwind_program(val->bias_and_unwind_program, &bias, &program);
  return program == PROG_UNWIND_HOTSPOT;
}
#endif

// hotspot_find_codeblob maps a given PC to the CodeBlob* that describes the
// JIT information regarding the method (or stub) this PC belongs to. This uses
// information from the PidPageMapping for the PC.
static EBPF_INLINE u64 hotspot_find_codeblob(const UnwindState *state, const HotspotProcInfo *ji)
{
  unsigned long segment, codeblob, segmap_start;
  u8 tag;

  DEBUG_PRINT(
    "jvm:  -> %lx in code start %lx, offset %lx",
    (unsigned long)state->pc,
    (unsigned long)state->text_section_bias,
    (unsigned long)state->text_section_offset);

  // The segment map contains information on finding the control data
  // structures given a PC. For documentation on this structure, see:
  // https://hg.openjdk.java.net/jdk-updates/jdk14u/file/default/src/hotspot/share/memory/heap.cpp#l376

  // Search for the code blob start using segmap. Hostagent will setup the mapping
  // so that bias is the code segment start, and thus text_section_offset will hold
  // the delta from start of the segment. It is shifted to get segment number.
  segment = state->text_section_offset >> ji->segment_shift;

  // Segment map start is put in to the PidPageMapping's file_id.
  segmap_start = (state->text_section_id >> HS_TSID_SEG_MAP_BIT) & HS_TSID_SEG_MAP_MASK;

  UNROLL for (int i = 0; i < HOTSPOT_SEGMAP_ITERATIONS; i++)
  {
    if (bpf_probe_read_user(&tag, sizeof(tag), (void *)(segmap_start + segment))) {
      return 0;
    }
    DEBUG_PRINT("jvm:    segment %lu, tag %u", segment, (unsigned)tag);

    // Stop if done or the segment is marked free
    if (tag == 0 || tag == 0xff) {
      break;
    }
    segment -= tag;
  }

  if (tag != 0) {
    // fail if we did not finish successfully
    return 0;
  }

  codeblob = state->text_section_bias + (segment << ji->segment_shift) + ji->heapblock_size;

  // We could check the HeapBlock::Header.used field, and possibly others
  // for further validation of still valid block.
  DEBUG_PRINT("jvm:  -> mapped to codeblob %lx", codeblob);
  return codeblob;
}

static EBPF_INLINE ErrorCode
hotspot_handle_vtable_chunks(HotspotUnwindInfo *ui, HotspotUnwindAction *action)
{
  DEBUG_PRINT("jvm:  -> unwind vtable");
  ui->line.subtype = FRAME_HOTSPOT_VTABLE;

#if defined(__x86_64__)
  // On x86 this has only the return address on stack. Code adapted from JDK-8178287.
  // This is something JVM itself does not handle right.
  *action = UA_UNWIND_PC_ONLY;
#elif defined(__aarch64__)
  // On ARM64, nothing is put on stack for this at all. Unwind via LR.
  *action = UA_UNWIND_AARCH64_LR;
#endif

  return ERR_OK;
}

static EBPF_INLINE ErrorCode hotspot_handle_interpreter(
  UnwindState *state,
  Trace *trace,
  HotspotUnwindInfo *ui,
  HotspotProcInfo *ji,
  HotspotUnwindAction *action)
{
  // Hotspot Interpreter has it's custom stack layout, and the unwinding is done based
  // on frame pointer. No frame information is in the CodeBlob header.
  // The Interpreter internal offsets seem relatively stable, but would need to be programmed
  // based on JVM version as they are not included in the introspection data.
  if (ui->fp < ui->sp || ui->fp >= ui->sp + 0x1000) {
    DEBUG_PRINT("jvm: fp too far away to be interpreter frame");
    goto error;
  }

  // Read the Interpreter stack frame registers
#define FP_OFFS 10
#if defined(__x86_64__)
  // https://hg.openjdk.org/jdk-updates/jdk14u/file/default/src/hotspot/cpu/x86/frame_x86.hpp#l77
  #define BCP_SLOT_JVM9 8
  // https://github.com/openjdk/jdk8u/blob/master/hotspot/src/cpu/x86/vm/frame_x86.hpp#L117
  #define BCP_SLOT_JVM8 7
  // https://hg.openjdk.org/jdk-updates/jdk14u/file/default/src/hotspot/cpu/x86/templateInterpreterGenerator_x86.cpp#l66
  #define BCP_REGISTER  r13
#elif defined(__aarch64__)
  // https://hg.openjdk.org/jdk-updates/jdk14u/file/default/src/hotspot/cpu/aarch64/frame_aarch64.hpp#l88
  #define BCP_SLOT_JVM9 9
  // https://github.com/openjdk/jdk8u/blob/master/hotspot/src/cpu/aarch64/vm/frame_aarch64.hpp#L125
  #define BCP_SLOT_JVM8 7
  // https://hg.openjdk.org/jdk-updates/jdk14u/file/default/src/hotspot/cpu/aarch64/assembler_aarch64.hpp#l136
  #define BCP_REGISTER  r22
#endif
  u64 regs[FP_OFFS + 2];
  if (bpf_probe_read_user(regs, sizeof(regs), (void *)(ui->fp - sizeof(u64[FP_OFFS])))) {
    DEBUG_PRINT("jvm: failed to read interpreter frame");
    goto error;
  }

  u64 bcp;
  if (trace->stack_len) {
    // Interpreter frame has the BCP value stored
    if (ji->jvm_version >= 9) {
      // JDK9+ frame has new 'mirror' slot which offsets the BCP slot by one
      bcp = regs[FP_OFFS - BCP_SLOT_JVM9];
    } else {
      // JDK8 and earlier
      bcp = regs[FP_OFFS - BCP_SLOT_JVM8];
    }
  } else {
    // When Interpreter frame code is interrupted, the real BCP is kept in
    // a register for performance. On x86_64 ABI it's on r13.
    bcp = state->BCP_REGISTER;
  }

  // Extract information from the frame
  u64 method = regs[FP_OFFS - 3];
  ui->sp     = regs[FP_OFFS - 1];
  ui->fp     = regs[FP_OFFS];
  ui->pc     = regs[FP_OFFS + 1];

  // Convert Byte Code Pointer (BCP) to Byte Code Index (BCI); that is, convert the pointer to
  // be offset of the byte code. Mainly to reduce the amount needed for this data from 64-bits
  // to 16-bits as the bytecode size is limited by JVM to 0xFFFE.
  u64 cmethod;
  if (bpf_probe_read_user(&cmethod, sizeof(cmethod), (void *)(method + ji->method_constmethod))) {
    DEBUG_PRINT("jvm: failed to read interpreter cmethod");
    goto error;
  }
  if (bcp >= cmethod + ji->cmethod_size) {
    // Convert Code Pointer to Index (offset)
    bcp -= cmethod + ji->cmethod_size;
  }
  DEBUG_PRINT(
    "jvm:  -> method = 0x%lx, cmethod = 0x%lx, bcp = %lx",
    (unsigned long)method,
    (unsigned long)cmethod,
    (unsigned long)bcp);
  if (bcp >= 0xffff) {
    // Range check, and mark BCI invalid if outside JVM spec range
    bcp = 0xffff;
  }

  // Interpreted frames send different pointers to host agent than other frame types.
  ui->file                 = method;
  ui->line.subtype         = FRAME_HOTSPOT_INTERPRETER;
  ui->line.pc_delta_or_bci = bcp;
  ui->line.ptr_check       = cmethod >> 3;

  *action = UA_UNWIND_COMPLETE;
  return ERR_OK;

error:
  increment_metric(metricID_UnwindHotspotErrInterpreterFP);
  return ERR_HOTSPOT_INTERPRETER_FP;
}

#if defined(__x86_64__)
static EBPF_INLINE void breadcrumb_fixup(UNUSED HotspotUnwindInfo *ui)
{
  // Nothing to do: breadcrumbs are not a thing on X86.
}
#elif defined(__aarch64__)
static EBPF_INLINE void breadcrumb_fixup(HotspotUnwindInfo *ui)
{
  // On ARM64, for some calls, the JVM pushes "breadcrumbs" onto the stack to make unwinding
  // easier for them. In the process, they unfortunately make it harder for us, since we have
  // to detect these cases and fix up SP accordingly. Fortunately, the code-gen is very static,
  // so it is easy to detect.
  //
  // The inserted code looks like this:
  //
  //   adr x9, ret_label
  //   lea x8, RuntimeAddress(entry)  ;; pseudo instruction, expands to series of mov/movk insns
  //   stp zr, r11, [sp, #-16]!
  //   blr x8
  // ret_label:
  //   add sp, sp, 16
  //
  // Note: x8 and x9 are JVM reserved scratch registers.
  //
  // The actual code generating this lives here:
  // https://github.com/openjdk/jdk/blob/jdk-17%2B35/src/hotspot/cpu/aarch64/aarch64.ad#L3731

  u64 lookback;
  if (bpf_probe_read_user(&lookback, sizeof(lookback), (void *)(ui->pc - sizeof(lookback)))) {
    return;
  }
  if (lookback == 0xd63f0100a9bf27ffULL /* stp; blr */) {
    ui->sp += 0x10;
  }
}
#endif

#if defined(__x86_64__)
static EBPF_INLINE ErrorCode
hotspot_handle_prologue(const CodeBlobInfo *cbi, HotspotUnwindInfo *ui, HotspotUnwindAction *action)
{
  // In the prologue code. It generally consists of stack 'banging' (check for stack
  // overflow), pushing FP, and finally allocating rest of the stack of 'frame_size'.
  if (ui->pc >= cbi->code_start + cbi->frame_comp - 4) {
    // Almost complete frame. Assume FP and PC on stack, and it's only the
    // final stack allocation opcodes to be executed (add sp).
    // TODO(tteras): This check is incomplete. There is some nasty variations
    // which require looking at the prologue opcodes.
    DEBUG_PRINT("jvm:  -> unwinding incomplete frame (fp+pc)");
    *action = UA_UNWIND_FP_PC;
    return ERR_OK;
  }
  // early in the prologue. assume only return address on stack
  DEBUG_PRINT("jvm:  -> unwinding incomplete frame (pc)");
  *action = UA_UNWIND_PC_ONLY;
  return ERR_OK;
}
#elif defined(__aarch64__)
static EBPF_INLINE ErrorCode
hotspot_handle_prologue(const CodeBlobInfo *cbi, HotspotUnwindInfo *ui, HotspotUnwindAction *action)
{
  // On ARM64, the prologue consists of various assembly snippets, most of which we aren't really
  // concerned with. This includes stuff like stack banging (which, other than the name might
  // suggest, doesn't actually write SP directly), initializing SVE registers and similar setup
  // stuff. It ends with instructions generated according to the following pseudo-code:
  //
  // >>> if frame_size < (1 << 9) + 16:
  // >>>   sub sp, sp, frame_size
  // >>>   stp fp, lr, [sp, frame_size - 16]
  // >>>   if jdk_option_enabled(PreserveFramePointer):
  // >>>     add fp, sp, frame_size - 16
  // >>> else:
  // >>>   stp fp, lr, [sp, -16]!
  // >>>   if jdk_option_enabled(PreserveFramePointer):
  // >>>     mov fp, sp
  // >>>   if frame_size < (1 << 12) + 16:
  // >>>     sub sp, sp, frame_size - 16
  // >>>   else:
  // >>>     # Note: x8 is reserved as a scratch register
  // >>>     mov x8, frame_size - 16
  // >>>     sub sp, sp, x8
  //
  // This general logic lives in the aarch64 variant of `MachPrologNode::emit`:
  // https://github.com/openjdk/jdk/blob/jdk-17%2B35/src/hotspot/cpu/aarch64/aarch64.ad#L1883
  // The part that we care about resides in `MacroAssembler::build_frame`:
  // https://github.com/openjdk/jdk/blob/jdk-17%2B35/src/hotspot/cpu/aarch64/macroAssembler_aarch64.cpp#L4445
  //
  // Frame sizes larger than (1 << 9) are exceedingly rare, so in practice, pretty much all
  // prologues end like this (assuming `PreserveFramePointer` isn't being used):
  //
  // >>> sub sp, sp, frame_size
  // >>> stp fp, lr, [sp, frame_size - 16]
  //
  // To unwind this prologue, all we need to do is to check whether the `sub` has already been
  // executed, and, if it was, to fix up the stack pointer accordingly. After that, we simply
  // unwind via the return address in the LR register.

  // Is the PC on the `stp` instruction?
  if (ui->pc == cbi->code_start + cbi->frame_comp - 4) {
    ui->sp += cbi->frame_size;
  }

  *action = UA_UNWIND_AARCH64_LR;
  return ERR_OK;
}
#endif

#if defined(__x86_64__)
static EBPF_INLINE bool hotspot_handle_epilogue(
  UNUSED const CodeBlobInfo *cbi, HotspotUnwindInfo *ui, HotspotUnwindAction *action)
{
  // On X86, use a heuristic to catch the likely spots of the epilogue.
  #define CODE_CUR 1
  u8 code[14];

  if (bpf_probe_read_user(code, sizeof(code), (void *)(ui->pc - CODE_CUR))) {
    return false;
  }

  // Current instruction is 'ret'
  if (code[CODE_CUR] == 0xc3) {
    DEBUG_PRINT("jvm:  -> epilogue on 'ret'");
    goto pc_only;
  }

  // Is 'ret' instruction *possible* in the next 'code' bytes?
  // NOTE: This can find false positives because x86 is variable length
  // instruction set.
  UNROLL for (int i = CODE_CUR + 1; i < sizeof(code); i++)
  {
    if (code[i] == 0xc3) {
      goto found_ret;
    }
  }
  // 'ret' not found, not an epilogue
  return false;

found_ret:
  // Current instruction is 'pop rbp'
  if (code[CODE_CUR] == 0x5d) {
    DEBUG_PRINT("jvm:  -> epilogue on 'pop rbp'");
    *action = UA_UNWIND_FP_PC;
    return true;
  }
  // Current instructions: 'testl %eax, (%r10)' + 'ret'
  // seen in the safe point polling, see:
  // https://hg.openjdk.org/jdk-updates/jdk14u/file/default/src/hotspot/cpu/x86/c1_LIRAssembler_x86.cpp#l558
  if (
    code[CODE_CUR] == 0x41 && code[CODE_CUR + 1] == 0x85 && code[CODE_CUR + 2] == 0x02 &&
    code[CODE_CUR + 3] == 0xc3) {
    DEBUG_PRINT("jvm:  -> epilogue on safepoint check'");
    goto pc_only;
  }
  // Current instruction: 'jne ...' + 'ret'
  // seen in native wrappers, see:
  // https://hg.openjdk.org/jdk-updates/jdk14u/file/default/src/hotspot/cpu/x86/sharedRuntime_x86_64.cpp#l2744
  if (code[CODE_CUR] == 0x0f && code[CODE_CUR + 1] == 0x85 && code[CODE_CUR + 6] == 0xc3) {
    DEBUG_PRINT("jvm:  -> epilogue on native wrapper exception check'");
    goto pc_only;
  }
  // Previous instruction was 'leave' or 'pop rbp'
  if (code[CODE_CUR - 1] == 0x5d || code[CODE_CUR - 1] == 0xc9) {
    DEBUG_PRINT("jvm:  -> epilogue after leave'");
    goto pc_only;
  }
  return false;
pc_only:
  *action = UA_UNWIND_PC_ONLY;
  return true;
}
#elif defined(__aarch64__)
static EBPF_INLINE bool
hotspot_handle_epilogue(const CodeBlobInfo *cbi, HotspotUnwindInfo *ui, HotspotUnwindAction *action)
{
  // On ARM64, the epilogue code is generated roughly like this:
  //
  // >>> remove_frame:
  // >>>   if framesize < (1 << 9) + 16:
  // >>>     ldp  fp, lr, [sp, #(frame_size - 16)]
  // >>>     add  sp, sp, frame_size
  // >>>   elif frame_size < (1 << 12) + 16:
  // >>>     add sp, sp, (frame_size - 16)
  // >>>     ldp fp, lr, [sp, #16]!
  // >>>   else:
  // >>>     mov rN, frame_size - 16
  // >>>     add sp, sp, rN
  // >>>     ldp fp, lr, [sp, #16]!
  // >>> safepoint_poll:
  // >>>   ldr  x8, [x28, <polling word offset>]
  // >>>   cmp  sp, x8
  // >>>   b.hi <slow_path>
  // >>> generated by unknown code:
  // >>>   ret
  //
  // In Java, it is extremely hard to create a function with a frame size larger than a few words.
  // Handling the cases for the larger stack sizes is not really worth the instructions it would
  // take up in the eBPF binary. The code below thus only handles the case where the frame size is
  // smaller than `(1 << 9) + 16`.

  if (cbi->frame_size >= (1 << 9) + 16) {
    // Frame sizes larger than this are extremely rare: skip these for now.
    increment_metric(metricID_UnwindHotspotUnsupportedFrameSize);
    return false;
  }

  // Determine the search pattern for the epilogue begin of this function by assembling the aarch64
  // instructions that we expect the JRE to generate for the epilogue.

  // Encode `ldp  fp, lr, [sp, #(frame_size - 16)]`. The OR inserts the immediate.
  // https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/LDP--Load-Pair-of-Registers-
  u64 ldp = 0xa9407bfd | ((((u64)cbi->frame_size - 16) / 8) << 15);

  // Encode `add  sp, sp, frame_size`. The OR again places the immediate.
  // https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/ADD--immediate---Add--immediate--
  u64 add = 0x910003ff | ((u64)cbi->frame_size << 10);

  #define EPI_LOOKBACK 6
  #define INSN_LEN     4

  // Scan for the epilogue pattern, using a 64-bit wide sliding window with a 32-bit stride.
  u8 find_offset = 0;
  u32 window[EPI_LOOKBACK];
  u64 needle = ldp | (add << 32);
  if (bpf_probe_read_user(window, sizeof(window), (void *)(ui->pc - sizeof(window) + INSN_LEN))) {
    return false;
  }

  UNROLL for (; find_offset < EPI_LOOKBACK - 1; ++find_offset)
  {
    if (*(u64 *)&window[find_offset] == needle) {
      goto pattern_found;
    }
  }

  // Still here? Pattern not found, give up.
  return false;

pattern_found:;

  // Index   Epilogue code                           Action to take when PC on instruction
  // -----   -------------                           -------------------------------------
  // 0       ldp  fp, lr, [sp, #(frame_size - 16)]   Bail out and let other code handle this case.
  // 1       add  sp, sp, frame_size                 Fix SP, then LR based unwinding.
  // 2       ldr  x8, [x28, <polling word>]          LR based unwinding.
  // 3       cmp  sp, x8                             LR based unwinding.
  // 4       b.hi <slow_path>                        LR based unwinding.
  // 5       ret                                     LR based unwinding.
  //
  // When we find the ldp/add pattern in our look-back window, it thus means that we need to perform
  // LR based unwinding. Since the look-back window ends at PC, the previous pattern search will not
  // find the pattern and have bailed out when the PC is on the `ldp`, which implicitly handles the
  // unwind action for the `ldp`.

  // If we're on the `add sp, sp, frame_size`, we need to fix up SP. The -1 is because the pattern
  // is two instructions wide.
  u8 epi_idx = EPI_LOOKBACK - 1 - find_offset;
  if (epi_idx == 1) {
    ui->sp += cbi->frame_size;
  }

  DEBUG_PRINT("jvm: epilogue case");
  *action = UA_UNWIND_AARCH64_LR;
  return true;

  #undef INSN_LEN
  #undef EPI_LOOKBACK
}
#endif

static EBPF_INLINE ErrorCode hotspot_handle_nmethod(
  const CodeBlobInfo *cbi,
  UNUSED Trace *trace,
  HotspotUnwindInfo *ui,
  HotspotProcInfo *ji,
  HotspotUnwindAction *action,
  bool topmost)
{
  // setup frame subtype, and get the native method _compile_id as pointer cookie
  // as it is unique to the compilation result

  ui->line.subtype   = FRAME_HOTSPOT_NATIVE;
  ui->line.ptr_check = cbi->compile_id;

  u64 deopt_handler = cbi->deopt_handler;
  if (ji->nmethod_uses_offsets) {
    // JDK7/8/23+: Deoptimization handler is an uint32 offset from the code blob start
    deopt_handler = cbi->address + (deopt_handler & 0xffffffff);
  }
  if (ui->pc == deopt_handler) {
    // If the PC where execution is to continue is the deoptimization handler, the frame
    // has been deoptimized.  This happens when something happened in the upper frames,
    // that broke the assumptions used at JIT compile time.
    // In practice the JVM rewrote the return address at the callers frame. It also stores
    // original PC before rewriting. This code retrieves that. For the deoptimization handler
    // generation look at:
    // https://hg.openjdk.java.net/jdk-updates/jdk14u/file/default/src/hotspot/cpu/x86/sharedRuntime_x86_64.cpp#l2906
    // Similar fixup is strategy for external unwinding is in:
    // https://hg.openjdk.java.net/jdk-updates/jdk14u/file/default/src/java.base/solaris/native/libjvm_db/libjvm_db.c#l1059
    u64 orig;
    if (
      bpf_probe_read_user(&orig, sizeof(orig), (void *)(ui->sp + cbi->orig_pc_offset)) ||
      orig < cbi->code_start || orig >= cbi->code_end) {
      // Just keep using the deoptimization point PC. It usually unwinds ok, and symbolizes
      // to the correct function. Potentially inlined scopes, and source line number is lost.
      DEBUG_PRINT("jvm:  -> deoptimized frame, pc recovery failed");
    } else {
      DEBUG_PRINT(
        "jvm:  -> deoptimized frame, pc recovered as 0x%lx (from sp+%d)",
        (unsigned long)orig,
        (s32)cbi->orig_pc_offset);
      ui->pc                   = orig;
      ui->line.pc_delta_or_bci = ui->pc - cbi->code_start;
    }
  }

  // Are we in the prologue?
  if (ui->pc < cbi->code_start + cbi->frame_comp) {
    return hotspot_handle_prologue(cbi, ui, action);
  }

  // Attempt prologue unwinding.
  if (topmost && hotspot_handle_epilogue(cbi, ui, action)) {
    return ERR_OK;
  }

  if (ui->fp >= ui->sp && ui->fp < ui->sp + cbi->frame_size + sizeof(u64[6])) {
    // FP is in a "sane" range for a frame-pointer based function:
    //   Between SP and SP+frame_size+few extra words.
    // That is, FP points to valid stack position that could be the frame. If it FP was used
    // as a general-purpose register, it would likely be something outside this range.
    // The native functions always store FP. It is valid frame pointer if this is the topmost
    // native frame after Interpreter, or always with -XX:+PreserveFramePointer.
    // NOTE: some other instances used frame_size * 2, but that can cause false positives when
    // frame_size is large. The FP would look valid, but if using it, we'd be actually jumping
    // over one or more stack frames. This happens when none of the function in between modify
    // FP. Also, if we skipped the functions, we would not be able to restore FP from
    // the skipped frames and potentially cause the whole unwinding to fail in later stage.
    DEBUG_PRINT("jvm:  -> using frame pointer (frame size %ld)", (long)(ui->fp - ui->sp));
    *action = UA_UNWIND_FRAME_POINTER;
    return ERR_OK;
  }

  // The real JVM has the same limitation. async-profiler has some heuristic examples for this.
  breadcrumb_fixup(ui);

  // Assume complete frame without frame pointer, use the CodeBlob frame_size.
  ui->sp += cbi->frame_size;

#ifndef HOTSPOT_RA_SEARCH_SLOTS
  // Frame size can be trusted.
  *action = UA_UNWIND_REGS;
  return ERR_OK;
#else
  // On x86, the generated code can occasionally push extra words to the stack and it might
  // be more than the advertised `frame_size`. The official unwinder seems to not handle this
  // case properly. This follows the Hotspot frame::safe_for_sender and async-profiler heuristic
  // to assume that PC points to valid code location inside the CodeCache. This is true for all
  // native methods as they are always called by another native method or a stub.
  //
  // For EBPF simplicity, this just verifies that the PC address is inside the active memory
  // mapping area. Additional checking could be done to search for CodeBlob and to verify that
  // the value is actually inside the code area and that the CodeBlob is in valid state.
  u64 stack[HOTSPOT_RA_SEARCH_SLOTS];
  bpf_probe_read_user(stack, sizeof(stack), (void *)(ui->sp - sizeof(u64)));
  for (int i = 0; i < HOTSPOT_RA_SEARCH_SLOTS; i++, ui->sp += sizeof(u64)) {
    DEBUG_PRINT("jvm:    -> %u pc candidate 0x%lx", i, (unsigned long)stack[i]);
    if (hotspot_addr_in_codecache(trace->pid, stack[i])) {
      DEBUG_PRINT("jvm:  -> unwinding complete frame + %d words", i);
      *action = UA_UNWIND_REGS;
      return ERR_OK;
    }
  }
  increment_metric(metricID_UnwindHotspotErrInvalidRA);
  return ERR_HOTSPOT_INVALID_RA;
#endif
}

static EBPF_INLINE ErrorCode
hotspot_handle_stub_fallback(const CodeBlobInfo *cbi, HotspotUnwindAction *action)
{
  DEBUG_PRINT("jvm:  -> unwind stub fallback path");

  if (!cbi->frame_size) {
    // "StubRoutines (1)" and "StubRoutines (2)" will have zero frame_size,
    // but valid frame pointer.
    *action = UA_UNWIND_FRAME_POINTER;
    return ERR_OK;
  }

  *action = UA_UNWIND_FRAME;
  return ERR_OK;
}

static EBPF_INLINE ErrorCode hotspot_handle_stub(
  UNUSED const UnwindState *state,
  const CodeBlobInfo *cbi,
  HotspotUnwindInfo *ui,
  HotspotUnwindAction *action)
{
  ui->line.subtype = FRAME_HOTSPOT_STUB;

#ifdef __aarch64__
  u64 info = state->text_section_id;
  if (!(info & (1UL << HS_TSID_IS_STUB_BIT))) {
    return hotspot_handle_stub_fallback(cbi, action);
  }

  DEBUG_PRINT("jvm:  -> unwind stub with unwind info 0x%016llX", info);

  if (info & (1UL << HS_TSID_HAS_FRAME_BIT)) {
    *action = UA_UNWIND_FRAME_POINTER;
    return ERR_OK;
  }

  u64 delta = (info >> HS_TSID_STACK_DELTA_BIT);
  delta &= HS_TSID_STACK_DELTA_MASK;
  delta *= HS_TSID_STACK_DELTA_SCALE;

  ui->sp += delta;

  *action = UA_UNWIND_AARCH64_LR;
  return ERR_OK;
#else
  return hotspot_handle_stub_fallback(cbi, action);
#endif
}

static EBPF_INLINE ErrorCode hotspot_execute_unwind_action(
  CodeBlobInfo *cbi,
  HotspotUnwindAction action,
  HotspotUnwindInfo *ui,
  UnwindState *state,
  Trace *trace)
{
  switch (action) {
  case UA_UNWIND_INVALID: return ERR_UNREACHABLE;
#if defined(__aarch64__)
  case UA_UNWIND_AARCH64_LR:
    if (state->lr_invalid) {
      increment_metric(metricID_UnwindHotspotErrLrUnwindingMidTrace);
      return ERR_HOTSPOT_LR_UNWINDING_MID_TRACE;
    }
    ui->pc = state->lr;
    goto unwind_complete;
#endif
  case UA_UNWIND_PC_ONLY: cbi->frame_size = sizeof(u64); goto unwind_frame;
  case UA_UNWIND_FRAME_POINTER:
    ui->sp = ui->fp;
    // fallthrough
  case UA_UNWIND_FP_PC:
    cbi->frame_size = sizeof(u64[2]);
    // fallthrough
  case UA_UNWIND_FRAME:
  unwind_frame:
    ui->sp += cbi->frame_size;
    // fallthrough
  case UA_UNWIND_REGS: {
    u64 frame[2];
    bpf_probe_read_user(frame, sizeof(frame), (void *)(ui->sp - sizeof(frame)));
    ui->pc = frame[1];
    if (cbi->frame_size >= sizeof(frame)) {
      DEBUG_PRINT("jvm:  -> recover fp");
      ui->fp = frame[0];
    }
  } // fallthrough
  case UA_UNWIND_COMPLETE: {
  unwind_complete:;
    u64 line        = calc_line(ui->line.subtype, ui->line.pc_delta_or_bci, ui->line.ptr_check);
    ErrorCode error = push_hotspot(trace, ui->file, line, state->return_address);
    if (error) {
      return error;
    }

    DEBUG_PRINT(
      "jvm:  -> pc: %lx, sp: %lx, fp: %lx",
      (unsigned long)ui->pc,
      (unsigned long)ui->sp,
      (unsigned long)ui->fp);
    state->pc = ui->pc;
    state->sp = ui->sp;
    state->fp = ui->fp;
    unwinder_mark_nonleaf_frame(state);
    increment_metric(metricID_UnwindHotspotFrames);
  }
  }

  return ERR_OK;
}

// Reads information from the CodeBlob for the current PC location from the JVM process.
static EBPF_INLINE ErrorCode hotspot_read_codeblob(
  const UnwindState *state,
  const HotspotProcInfo *ji,
  HotspotUnwindScratchSpace *scratch,
  CodeBlobInfo *cbi)
{
  // Find the CodeBlob (JIT function metadata) for this PC.
  cbi->address = hotspot_find_codeblob(state, ji);
  if (!cbi->address) {
    DEBUG_PRINT("jvm: no codeblob matched for pc");
    increment_metric(metricID_UnwindHotspotErrNoCodeblob);
    return ERR_HOTSPOT_NO_CODEBLOB;
  }

  // Read the CodeBlob. Note that this is intentionally a memory over-read in most cases: we read
  // the entire size of our CodeBlob buffer despite the CodeBlob typically being smaller than that
  // buffer. This way, we don't have to do a second read for the frame type in order to determine
  // the exact CodeBlob/CompiledMethod/nmethod size. The CodeBlob is allocated in the JIT area,
  // preceding the actual JIT code and data for the function. It is thus exceedingly unlikely for
  // us to accidentally read into a guard / unallocated page despite the over-read.
  if (bpf_probe_read_user(scratch->codeblob, sizeof(scratch->codeblob), (void *)cbi->address)) {
    goto read_error_exit;
  }

  // Make the verifier happy. No bound checks required for the remaining offsets: they are u8, and
  // the verifier is aware that their maximum value is smaller than our `codeblob` buffer.
  if (
    ji->nmethod_deopt_offset + sizeof(u64) > sizeof(scratch->codeblob) ||
    ji->nmethod_compileid + sizeof(u32) > sizeof(scratch->codeblob) ||
    ji->nmethod_orig_pc_offset + sizeof(u64) > sizeof(scratch->codeblob)) {
    return ERR_UNREACHABLE;
  }

  // Extract the needed CodeBlob fields.
  cbi->code_start     = *(u64 *)(scratch->codeblob + ji->codeblob_codestart);
  cbi->code_end       = *(u64 *)(scratch->codeblob + ji->codeblob_codeend);
  cbi->frame_size     = *(u32 *)(scratch->codeblob + ji->codeblob_framesize) * 8;
  cbi->frame_comp     = *(u32 *)(scratch->codeblob + ji->codeblob_framecomplete);
  cbi->compile_id     = *(u32 *)(scratch->codeblob + ji->nmethod_compileid);
  cbi->orig_pc_offset = *(u32 *)(scratch->codeblob + ji->nmethod_orig_pc_offset);
  cbi->deopt_handler  = *(u64 *)(scratch->codeblob + ji->nmethod_deopt_offset);

  // `frame_type` is actually the first 4 characters of the CodeBlob type name.
  u64 code_name_addr = *(u64 *)(scratch->codeblob + ji->codeblob_name);
  if (bpf_probe_read_user(&cbi->frame_type, sizeof(cbi->frame_type), (void *)code_name_addr)) {
    goto read_error_exit;
  }

  // JDK7/8 and 23+: code start and end are actually uint32 offsets from the code blob start
  if (ji->nmethod_uses_offsets) {
    cbi->code_start = cbi->address + (cbi->code_start & 0xffffffff);
    cbi->code_end   = cbi->address + (cbi->code_end & 0xffffffff);
  }

  // JDK23+20+: frame_comp is uint16_t now.
  // https://github.com/openjdk/jdk/commit/b704e91241b0
  if (ji->jvm_version >= 23) {
    cbi->frame_comp &= 0xffff;
  }

  DEBUG_PRINT(
    "jvm:  -> code %lx-%lx", (unsigned long)cbi->code_start, (unsigned long)cbi->code_end);
  DEBUG_PRINT(
    "jvm:  -> frame_complete %u, frame_size %u, frame_type 0x%x",
    cbi->frame_comp,
    cbi->frame_size,
    cbi->frame_type);

  return 0;

read_error_exit:
  DEBUG_PRINT("jvm: failed to read codeblob");
  increment_metric(metricID_UnwindHotspotErrInvalidCodeblob);
  return ERR_HOTSPOT_INVALID_CODEBLOB;
}

// hotspot_unwind_one_frame fully unwinds one HotSpot frame
static EBPF_INLINE ErrorCode
hotspot_unwind_one_frame(PerCPURecord *record, HotspotProcInfo *ji, bool maybe_topmost)
{
  UnwindState *state = &record->state;
  Trace *trace       = &record->trace;
  HotspotUnwindInfo ui;

  increment_metric(metricID_UnwindHotspotAttempts);

  ui.pc = state->pc;
  ui.sp = state->sp;
  ui.fp = state->fp;

  // Read the CodeBlob.
  CodeBlobInfo cbi;
  ErrorCode err = hotspot_read_codeblob(state, ji, &record->hotspotUnwindScratch, &cbi);
  if (err) {
    return err;
  }

  // For most frame types, the CodeBlob address also serves as the file.
  ui.file                 = cbi.address;
  ui.line.ptr_check       = cbi.frame_type;
  ui.line.pc_delta_or_bci = ui.pc - cbi.code_start;

  HotspotUnwindAction action = UA_UNWIND_INVALID;
  switch (cbi.frame_type) {
  case FRAMETYPE_nmethod:        // JIT-compiled method
  case FRAMETYPE_native_nmethod: // stub to call C-implemented java method
    err = hotspot_handle_nmethod(
      &cbi, trace, &ui, ji, &action, maybe_topmost && !state->return_address);
    break;
  case FRAMETYPE_Interpreter: // main Interpreter program running byte code
    err = hotspot_handle_interpreter(state, trace, &ui, ji, &action);
    break;
  case FRAMETYPE_vtable_chunks: // megamorphic interface call site
    err = hotspot_handle_vtable_chunks(&ui, &action);
    break;
  default: // stubs and intrinsic functions (too many to list)
    err = hotspot_handle_stub(state, &cbi, &ui, &action);
  }

  if (err) {
    return err;
  }

  return hotspot_execute_unwind_action(&cbi, action, &ui, state, trace);
}

// unwind_hotspot is the entry point for tracing when invoked from the native tracer
// and it recursive unwinds all HotSpot frames and then jumps back to unwind further
// native frames that follow.
static EBPF_INLINE int unwind_hotspot(struct pt_regs *ctx)
{
  PerCPURecord *record = get_per_cpu_record();
  if (!record)
    return -1;

  Trace *trace = &record->trace;
  pid_t pid    = trace->pid;
  DEBUG_PRINT("==== jvm: unwind %d ====", trace->stack_len);

  HotspotProcInfo *ji = bpf_map_lookup_elem(&hotspot_procs, &pid);
  if (!ji) {
    DEBUG_PRINT("jvm: no HotspotProcInfo for this pid");
    return 0;
  }

  int unwinder    = PROG_UNWIND_STOP;
  ErrorCode error = ERR_OK;
  UNROLL for (int i = 0; i < HOTSPOT_FRAMES_PER_PROGRAM; i++)
  {
    unwinder = PROG_UNWIND_STOP;
    error    = hotspot_unwind_one_frame(record, ji, i == 0);
    if (error) {
      break;
    }

    error = get_next_unwinder_after_native_frame(record, &unwinder);
    if (error || unwinder != PROG_UNWIND_HOTSPOT) {
      break;
    }
  }

  record->state.unwind_error = error;
  tail_call(ctx, unwinder);
  DEBUG_PRINT("jvm: tail call for next frame unwinder (%d) failed", unwinder);
  return -1;
}
MULTI_USE_FUNC(unwind_hotspot)
