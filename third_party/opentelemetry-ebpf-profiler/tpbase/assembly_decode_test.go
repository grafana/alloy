// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tpbase

import (
	"debug/elf"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFSBase(t *testing.T) {
	testCases := map[string]struct {
		machine  elf.Machine
		funcName string
		code     []byte
		fsBase   uint32
	}{
		"gcc recent": {
			machine:  elf.EM_X86_64,
			funcName: "aout_dump_debugregs",
			// kernels 4.19 -> 5.6 with gcc 8.3 -> 10.0
			//
			// 31 c0                                     xor eax,eax
			// 45 31 c0                                  xor r8d,r8d
			// 41 ba 02 00 00 00                         mov r10d,0x2
			// 65 4c 8b 0c 25 45 23 01 00                mov r9,QWORD PTR gs:0x12345
			// 49 8b 94 c1 de 0c 00 00                   mov rdx,QWORD PTR [r9+rax*8+0xcde]
			code: []byte{
				0x0f, 0x1f, 0x44, 0x00, 0x00, // 5-byte nop from kernel ftrace infrastructure
				0x31, 0xc0,
				0x45, 0x31, 0xc0,
				0x41, 0xba, 0x02, 0x00, 0x00, 0x00,
				0x65, 0x4c, 0x8b, 0x0c, 0x25, 0x45, 0x23, 0x01, 0x00,
				0x49, 0x8b, 0x94, 0xc1, 0xde, 0x0c, 0x00, 0x00,
			},
			fsBase: 3278,
		},
		"GKE clang9": {
			machine:  elf.EM_X86_64,
			funcName: "aout_dump_debugregs",
			//nolint:lll
			// Linux version 4.19.112+ (builder@c9d55aaf8a8b) (Chromium OS 9.0_pre361749_p20190714-r4 clang version 9.0.0 (/var/cache/chromeos-cache/distfiles/host/egit-src/llvm-project c11de5eada2decd0a495ea02676b6f4838cd54fb) (based on LLVM 9.0.0svn)) #1 SMP Tue Dec 29 13:50:37 PST 2020
			//
			// 55                            push   rbp
			// 48 89 e5                      mov    rbp,rsp
			// 65 48 8b 04 25 00 4d 01 00    mov    rax,QWORD PTR gs:0x14d00
			// 48 8b 90 78 0a 00 00          mov    rdx,QWORD PTR [rax+0xa78]
			code: []byte{
				0x55,
				0x48, 0x89, 0xe5,
				0x65, 0x48, 0x8b, 0x04, 0x25, 0x00, 0x4d, 0x01, 0x00,
				0x48, 0x8b, 0x90, 0x78, 0x0a, 0x00, 0x00,
			},
			fsBase: 2664,
		},
		"Amazon linux": {
			machine:  elf.EM_X86_64,
			funcName: "aout_dump_debugregs",
			// Kernel 4.14, gcc 7.3
			//
			// 31 c0                                     xor eax,eax
			// 31 f6                                     xor esi,esi
			// 41 b9 02 00 00 00                         mov r9d,0x2
			// 65 4c 8b 04 25 56 34 02 00                mov r8,QWORD PTR gs:0x23456
			// 49 8b 94 c0 ea 0d 00 00                   mov rdx,QWORD PTR [r8+rax*8+0xdea]
			code: []byte{
				0x31, 0xc0,
				0x31, 0xf6,
				0x41, 0xb9, 0x02, 0x00, 0x00, 0x00,
				0x65, 0x4c, 0x8b, 0x04, 0x25, 0x56, 0x34, 0x02, 0x00,
				0x49, 0x8b, 0x94, 0xc0, 0xea, 0x0d, 0x00, 0x00,
			},
			fsBase: 3546,
		},
		"Ubuntu Bionic": {
			machine:  elf.EM_X86_64,
			funcName: "aout_dump_debugregs",
			// kernel 4.15, gcc 7.5
			//
			// 55                                        push rbp
			// 31 c0                                     xor eax,eax
			// 31 f6                                     xor esi,esi
			// 41 b9 02 00 00 00                         mov r9d,0x2
			// 48 89 e5                                  mov rbp,rsp
			// 65 4c 8b 04 25 34 12 00 00                mov r8,QWORD PTR gs:0x1234
			// 49 8b 94 c0 aa 0a 00 00                   mov rdx,QWORD PTR [r8+rax*8+0xaaa]
			code: []byte{
				0x55,
				0x31, 0xc0,
				0x31, 0xf6,
				0x41, 0xb9, 0x02, 0x00, 0x00, 0x00,
				0x48, 0x89, 0xe5,
				0x65, 0x4c, 0x8b, 0x04, 0x25, 0x34, 0x12, 0x00, 0x00,
				0x49, 0x8b, 0x94, 0xc0, 0xaa, 0x0a, 0x00, 0x00,
			},
			fsBase: 2714,
		},
		"Ubuntu Focal Fossa (AWS)": {
			machine:  elf.EM_X86_64,
			funcName: "aout_dump_debugregs",
			// kernel 5.4.0-1029-aws
			//
			// 55                                           push   rbp
			// 31 c0                                        xor    eax,eax
			// 45 31 c0                                     xor    r8d,r8d
			// 41 ba 02 00 00 00                            mov    r10d,0x2
			// 65 4c 8b 0c 25 c0 6b 01 00                   mov    r9,QWORD PTR gs:0x16bc0
			// 48 89 e5                                     mov    rbp,rsp
			// 49 8b 94 c1 38 13 00 00                      mov    rdx,QWORD PTR [r9+rax*8+0x1338]
			code: []byte{
				0x55,
				0x31, 0xc0,
				0x45, 0x31, 0xc0,
				0x41, 0xba, 0x02, 0x00, 0x00, 0x00,
				0x65, 0x4c, 0x8b, 0x0c, 0x25, 0xc0, 0x6b, 0x01, 0x00,
				0x48, 0x89, 0xe5,
				0x49, 0x8b, 0x94, 0xc1, 0x38, 0x13, 0x00, 0x00,
			},
			fsBase: 4904,
		},
		"RHEL / gcc 4.8.5-39": {
			machine:  elf.EM_X86_64,
			funcName: "aout_dump_debugregs",
			// from booking.com
			// 55                           push   rbp
			// be 10 00 00 00               mov    esi,0x10
			// 31 c0                        xor    eax,eax
			// 65 48 8b 14 25 80 5c 01 00   mov    rdx,QWORD PTR gs:0x15c80
			// 4c 8d 8a c0 12 00 00         lea    r9,[rdx+0x12c0]
			// 45 31 c0                     xor    r8d,r8d
			// 41 ba 02 00 00 00            mov    r10d,0x2
			// 48 89 e5                     mov    rbp,rsp
			// 49 8b 54 c1 38               mov    rdx,QWORD PTR [r9+rax*8+0x38]
			// 48 85 d2                     test   rdx,rdx
			code: []byte{
				0x55,
				0xbe, 0x10, 0x00, 0x00, 0x00,
				0x31, 0xc0,
				0x65, 0x48, 0x8b, 0x14, 0x25, 0x80, 0x5c, 0x01, 0x00,
				0x4c, 0x8d, 0x8a, 0xc0, 0x12, 0x00, 0x00,
				0x45, 0x31, 0xc0,
				0x41, 0xba, 0x02, 0x00, 0x00, 0x00,
				0x48, 0x89, 0xe5,
				0x49, 0x8b, 0x54, 0xc1, 0x38,
				0x48, 0x85, 0xd2,
			},
			fsBase: 4840,
		},
		"AL / Kernel 5.10+": {
			machine:  elf.EM_X86_64,
			funcName: "x86_fsbase_write_task",
			// Extracted from Alpine Linux (kernel 5.10+), but should be similar on all
			// kernels 4.20+ due to x86-64 calling conventions.
			//
			// 48 89 b7 2a 0f 00 00         mov    QWORD PTR [rdi+0xf2a],rsi
			code: []byte{
				0x48, 0x89, 0xb7, 0x2a, 0x0f, 0x00, 0x00,
			},
			fsBase: 3882,
		},
		"tls_set / arm64": {
			machine:  elf.EM_AARCH64,
			funcName: "tls_set",
			// HINT #0x22
			// MOV X9, X30
			// NOP
			// HINT #0x19
			// STP X29, X30, [SP,#-64]!
			// MRS X1, S3_0_C4_C1_0
			// MOV X29, SP
			// STP X21, X22, [SP,#32]
			// MOV X21, X0				1. Register X0 moved to X21
			// LDR X0, [X1,#1816]
			// STR X0, [SP,#56]
			// MOV X0, #0x0
			// LDR X1, [X21,#3440]			2. #3440 is the offset we want
			code: []byte{
				0x5f, 0x24, 0x03, 0xd5, 0xe9, 0x03, 0x1e, 0xaa,
				0x1f, 0x20, 0x03, 0xd5, 0x3f, 0x23, 0x03, 0xd5,
				0xfd, 0x7b, 0xbc, 0xa9, 0x01, 0x41, 0x38, 0xd5,
				0xfd, 0x03, 0x00, 0x91, 0xf5, 0x5b, 0x02, 0xa9,
				0xf5, 0x03, 0x00, 0xaa, 0x20, 0x8c, 0x43, 0xf9,
				0xe0, 0x1f, 0x00, 0xf9, 0x00, 0x00, 0x80, 0xd2,
				0xa1, 0xba, 0x46, 0xf9, 0xe1, 0x1b, 0x00, 0xf9,
				0x83, 0x01, 0x00, 0x34, 0xf3, 0x53, 0x01, 0xa9,
			},
			fsBase: 3440,
		},
		"Alpine 3.18.4 EC2 / arm64": {
			machine:  elf.EM_AARCH64,
			funcName: "tls_set",
			// HINT #0x19
			// STP X29, X30, [SP,#-96]!
			// MRS X6, S3_0_C4_C1_0
			// MOV X29, SP
			// STP X19, X20, [SP,#16]
			// MOV X20, X0
			// MOV W19, W3
			// STP X21, X22, [SP,#32]
			// MOV W21, W2
			// MOV X22, X4
			// STR X23, [SP,#48]
			// MOV X23, X5
			// LDR X0, [X6,#1896]
			// STR X0, [SP,#88]
			// MOV X0, #0x0
			// STP XZR, XZR, [SP,#72]
			// LDR X3, [X20,#3632]
			code: []byte{
				0x3f, 0x23, 0x03, 0xd5, 0xfd, 0x7b, 0xba, 0xa9,
				0x06, 0x41, 0x38, 0xd5, 0xfd, 0x03, 0x00, 0x91,
				0xf3, 0x53, 0x01, 0xa9, 0xf4, 0x03, 0x00, 0xaa,
				0xf3, 0x03, 0x03, 0x2a, 0xf5, 0x5b, 0x02, 0xa9,
				0xf5, 0x03, 0x02, 0x2a, 0xf6, 0x03, 0x04, 0xaa,
				0xf7, 0x1b, 0x00, 0xf9, 0xf7, 0x03, 0x05, 0xaa,
				0xc0, 0xb4, 0x43, 0xf9, 0xe0, 0x2f, 0x00, 0xf9,
				0x00, 0x00, 0x80, 0xd2, 0xff, 0xff, 0x04, 0xa9,
				0x83, 0x1a, 0x47, 0xf9, 0xe3, 0x27, 0x00, 0xf9,
			},
			fsBase: 3632,
		},
		"Linux 6.5.11 compiled with LLVM-17": {
			//nolint:lll
			// https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/commit/?h=v6.5.11&id=799441832db16b99e400ccbec55db801e6992819
			machine:  elf.EM_AARCH64,
			funcName: "tls_set",
			code: []byte{
				0x3f, 0x23, 0x03, 0xd5, // paciasp
				0xff, 0x83, 0x01, 0xd1, // sub	sp, sp, #0x60
				0xfd, 0x7b, 0x03, 0xa9, // stp	x29, x30, [sp, #48]
				0xf6, 0x57, 0x04, 0xa9, // stp	x22, x21, [sp, #64]
				0xf4, 0x4f, 0x05, 0xa9, // stp	x20, x19, [sp, #80]
				0xfd, 0xc3, 0x00, 0x91, // add	x29, sp, #0x30
				0x08, 0x41, 0x38, 0xd5, // mrs	x8, sp_el0
				0xf5, 0x03, 0x05, 0xaa, // mov	x21, x5
				0x08, 0x31, 0x45, 0xf9, // ldr	x8, [x8, #2656]
				0xf4, 0x03, 0x04, 0xaa, // mov	x20, x4
				0xf3, 0x03, 0x00, 0xaa, // mov	x19, x0
				0xa8, 0x83, 0x1f, 0xf8, // stur	x8, [x29, #-8]
				0x08, 0x98, 0x45, 0xf9, // ldr	x8, [x0, #2864]
				0xe8, 0xff, 0x01, 0xa9, // stp	x8, xzr, [sp, #24]
				0x1f, 0x20, 0x03, 0xd5, // nop
				0x63, 0x02, 0x00, 0x34, // cbz	w3, 0x1c630
			},
			fsBase: 2864,
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			var analyzers []Analyzer
			switch test.machine {
			case elf.EM_X86_64:
				analyzers = getAnalyzersX86()
			case elf.EM_AARCH64:
				analyzers = getAnalyzersARM()
			}
			if analyzers == nil {
				t.Skip("tests not available on this platform")
			}
			for _, a := range analyzers {
				if a.FunctionName != test.funcName {
					continue
				}
				fsBase, err := a.Analyze(test.code)
				if assert.NoError(t, err) {
					assert.Equal(t, test.fsBase, fsBase, "Wrong fsbase extraction")
				}
				return
			}
			t.Errorf("no extractor for '%s'", test.funcName)
		})
	}
}
