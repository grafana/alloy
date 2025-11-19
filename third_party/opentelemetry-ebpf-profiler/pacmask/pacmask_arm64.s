//go:build arm64

// func PACIA(ptr, modifier uint64) uint64;
//
// This particular implementation of this intrinsic uses the `paciasp`
// instruction rather than the actual `pacia` instruction, even if that makes
// the implementation more complex due to the required register shuffling. The
// reason here is that `paciasp` is encoded in a space that was previously a
// `nop`, meaning that it is backward compatible to devices without PAC support.
// This isn't the case for the more generic `pacia` instruction.
TEXT ·PACIA(SB),$0-16
	// Backup original LR and SP.
	MOVD	LR, R1
	MOVD	RSP, R2

	// Move `ptr` into LR
	MOVD	ptr+0(FP), LR

	// Move `modifier` into SP.
	MOVD	modifier+8(FP), R0
	MOVD	R0, RSP

	// `PACIASP` instruction. Go assembler doesn't support it yet.
	WORD 	$0xD503233F

	// Temporarily place PAC'ed LR into X0, since the stack ptr isn't restored, yet.
	MOVD	LR, R0

	// Restore original SP and LR.
	MOVD	R2, RSP
	MOVD	R1, LR

	// Place the return value on stack.
	MOVD	R0, r1+16(FP)

	RET
