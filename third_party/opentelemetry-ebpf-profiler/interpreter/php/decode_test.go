// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package php

import (
	"debug/elf"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/libpf"
)

func TestRetrieveZendVMKind(t *testing.T) {
	testdata := []struct {
		code     []byte
		machine  elf.Machine
		expected uint
	}{
		{
			code: []byte{
				0xF3, 0x0F, 0x1E, 0xFA, // 	endbr64
				0xB8, 0x04, 0x00, 0x00, 0x00, // 	mov 	eax, 4
				0xC3, // 	ret
			},
			machine:  elf.EM_X86_64,
			expected: 4,
		}, {
			code: []byte{
				0x80, 0x00, 0x80, 0x52, // mov     w0,  #0x4
				0xc0, 0x03, 0x5f, 0xd6, // ret
			},
			machine:  elf.EM_AARCH64,
			expected: 4,
		},
	}
	for _, td := range testdata {
		var res uint
		var err error
		switch td.machine {
		case elf.EM_AARCH64:
			res, err = retrieveZendVMKindARM(td.code)
		case elf.EM_X86_64:
			res, err = retrieveZendVMKindX86(td.code)
		}
		require.NoError(t, err)
		assert.Equal(t, td.expected, res)
	}
}

func TestRetrieveExecuteExJumpLabelAddress(t *testing.T) {
	testdata := []struct {
		code        []byte
		codeAddress libpf.SymbolValue
		machine     elf.Machine
		expected    libpf.SymbolValue
	}{
		{
			code: []byte{
				// 387090:  f3 0f 1e fa              endbr64
				// 387094:  55                       push   ebp
				// 387095:  48                       dec    eax
				// 387096:  89 e5                    mov    ebp,  esp
				// 387098:  41                       inc    ecx
				// 387099:  55                       push   ebp
				// 38709a:  41                       inc    ecx
				// 38709b:  54                       push   esp
				// 38709c:  53                       push   ebx
				// 38709d:  48                       dec    eax
				// 38709e:  81 ec 88 00 00 00        sub    esp,  0x88
				// 3870a4:  64 48                    fs dec eax
				// 3870a6:  8b 04 25 28 00 00 00     mov    eax,  DWORD PTR [eiz*1+0x28]
				// 3870ad:  48                       dec    eax
				// 3870ae:  89 45 d8                 mov    DWORD PTR [ebp-0x28],  eax
				// 3870b1:  31 c0                    xor    eax,  eax
				// 3870b3:  4c                       dec    esp
				// 3870b4:  89 75 b8                 mov    DWORD PTR [ebp-0x48],  esi
				// 3870b7:  49                       dec    ecx
				// 3870b8:  89 fe                    mov    esi,  edi
				// 3870ba:  4c                       dec    esp
				// 3870bb:  89 7d b0                 mov    DWORD PTR [ebp-0x50],  edi
				// 3870be:  48                       dec    eax
				// 3870bf:  85 ff                    test   edi,  edi
				// 3870c1:  74 30                    je     0x3870f3
				// 3870c3:  4c                       dec    esp
				// 3870c4:  8b 3f                    mov    edi,  DWORD PTR [edi]
				// 3870c6:  48                       dec    eax
				// 3870c7:  8d 05 f3 4d 21 00        lea    eax,  ds:0x214df3
				// 3870cd:  0f b6 80 16 02 00 00     movzx  eax,  BYTE PTR [eax+0x216]
				// 3870d4:  84 c0                    test   al,  al
				// 3870d6:  0f 85 ec 28 00 00        jne    0x3899c8
				// 3870dc:  48                       dec    eax
				// 3870dd:  8d 1d dd 4d 21 00        lea    ebx,  ds:0x214ddd
				// 3870e3:  48                       dec    eax
				// 3870e4:  39 ab 30 02 00 00        cmp    DWORD PTR [ebx+0x230],  ebp
				// 3870ea:  0f 83 4c e7 d9 ff        jae    0x12583c
				// 3870f0:  41                       inc    ecx
				// 3870f1:  ff 27                    jmp    DWORD PTR [edi]
				// 3870f3:  48                       dec    eax       // <<<<
				// 3870f4:  8d 05 06 01 1e 00        lea    eax,  ds:0x1e0106
				// 3870fa:  66 0f ef c0              pxor   xmm0,  xmm0
				// 3870fe:  c7 05 c0 5a 21 00 95 0d 00 00    mov    DWORD PTR ds:0x215ac0,  0xd95
				// 387108:  48                       dec    eax
				// 387109:  8d 7d b0                 lea    edi,  [ebp-0x50]
				// 38710c:  48                       dec    eax
				// 38710d:  89                       .byte 0x89
				// 38710e:  05                       .byte 0x5
				// 38710f:  bd                       .byte 0xbd
				0xf3, 0x0f, 0x1e, 0xfa, 0x55, 0x48, 0x89, 0xe5, 0x41, 0x55, 0x41, 0x54,
				0x53, 0x48, 0x81, 0xec, 0x88, 0x00, 0x00, 0x00, 0x64, 0x48, 0x8b, 0x04,
				0x25, 0x28, 0x00, 0x00, 0x00, 0x48, 0x89, 0x45, 0xd8, 0x31, 0xc0, 0x4c,
				0x89, 0x75, 0xb8, 0x49, 0x89, 0xfe, 0x4c, 0x89, 0x7d, 0xb0, 0x48, 0x85,
				0xff, 0x74, 0x30, 0x4c, 0x8b, 0x3f, 0x48, 0x8d, 0x05, 0xf3, 0x4d, 0x21,
				0x00, 0x0f, 0xb6, 0x80, 0x16, 0x02, 0x00, 0x00, 0x84, 0xc0, 0x0f, 0x85,
				0xec, 0x28, 0x00, 0x00, 0x48, 0x8d, 0x1d, 0xdd, 0x4d, 0x21, 0x00, 0x48,
				0x39, 0xab, 0x30, 0x02, 0x00, 0x00, 0x0f, 0x83, 0x4c, 0xe7, 0xd9, 0xff,
				0x41, 0xff, 0x27, 0x48, 0x8d, 0x05, 0x06, 0x01, 0x1e, 0x00, 0x66, 0x0f,
				0xef, 0xc0, 0xc7, 0x05, 0xc0, 0x5a, 0x21, 0x00, 0x95, 0x0d, 0x00, 0x00,
				0x48, 0x8d, 0x7d, 0xb0, 0x48, 0x89, 0x05, 0xbd,
			},
			codeAddress: 0x387090,
			machine:     elf.EM_X86_64,
			expected:    0x3870f3,
		},
		{
			code: []byte{
				//4e4824:        d10243ff         sub     sp,  sp,  #0x90
				//4e4828:        d00058c1         adrp    x1,  0xffe000
				//4e482c:        f9477c21         ldr     x1,  [x1,  #3832]
				//4e4830:        a9047bfd         stp     x29,  x30,  [sp,  #64]
				//4e4834:        910103fd         add     x29,  sp,  #0x40
				//4e4838:        a90553f3         stp     x19,  x20,  [sp,  #80]
				//4e483c:        a9065bf5         stp     x21,  x22,  [sp,  #96]
				//4e4840:        f9400022         ldr     x2,  [x1]
				//4e4844:        f81f83a2         stur    x2,  [x29,  #-8]
				//4e4848:        d2800002         mov     x2,  #0x0                        // #0
				//4e484c:        a93defbc         stp     x28,  x27,  [x29,  #-40]
				//4e4850:        aa0003fb         mov     x27,  x0
				//4e4854:        b40001e0         cbz     x0,  0x4e4890
				//4e4858:        f00058c1         adrp    x1,  0xfff000
				//4e485c:        f940e021         ldr     x1,  [x1,  #448]
				//4e4860:        f940001c         ldr     x28,  [x0]
				//4e4864:        91085820         add     x0,  x1,  #0x216
				//4e4868:        08dffc00         ldarb   w0,  [x0]
				//4e486c:        72001c1f         tst     w0,  #0xff
				//4e4870:        54014321         b.ne    0x4e70d4  // b.any
				//4e4874:        f00058d3         adrp    x19,  0xfff000
				//4e4878:        f940e273         ldr     x19,  [x19,  #448]
				//4e487c:        f9411a60         ldr     x0,  [x19,  #560]
				//4e4880:        eb1d001f         cmp     x0,  x29
				//4e4884:        5405c5e2         b.cs    0x4f0140  // b.hs,  b.nlast
				//4e4888:        f9400380         ldr     x0,  [x28]
				//4e488c:        d61f0000         br      x0
				//4e4890:        f00058c3         adrp    x3,  0xfff000 // <<<<
				//4e4894:        f9459863         ldr     x3,  [x3,  #2864]
				//4e4898:        f00059e4         adrp    x4,  0x1023000
				//4e489c:        91224080         add     x0,  x4,  #0x890
				//4e48a0:        90000002         adrp    x2,  0x4e4000
				0xff, 0x43, 0x02, 0xd1, 0xc1, 0x58, 0x00, 0xd0, 0x21, 0x7c, 0x47, 0xf9, 0xfd, 0x7b,
				0x04, 0xa9, 0xfd, 0x03, 0x01, 0x91, 0xf3, 0x53, 0x05, 0xa9, 0xf5, 0x5b, 0x06, 0xa9,
				0x22, 0x00, 0x40, 0xf9, 0xa2, 0x83, 0x1f, 0xf8, 0x02, 0x00, 0x80, 0xd2, 0xbc, 0xef,
				0x3d, 0xa9, 0xfb, 0x03, 0x00, 0xaa, 0xe0, 0x01, 0x00, 0xb4, 0xc1, 0x58, 0x00, 0xf0,
				0x21, 0xe0, 0x40, 0xf9, 0x1c, 0x00, 0x40, 0xf9, 0x20, 0x58, 0x08, 0x91, 0x00, 0xfc,
				0xdf, 0x08, 0x1f, 0x1c, 0x00, 0x72, 0x21, 0x43, 0x01, 0x54, 0xd3, 0x58, 0x00, 0xf0,
				0x73, 0xe2, 0x40, 0xf9, 0x60, 0x1a, 0x41, 0xf9, 0x1f, 0x00, 0x1d, 0xeb, 0xe2, 0xc5,
				0x05, 0x54, 0x80, 0x03, 0x40, 0xf9, 0x00, 0x00, 0x1f, 0xd6, 0xc3, 0x58, 0x00, 0xf0,
				0x63, 0x98, 0x45, 0xf9, 0xe4, 0x59, 0x00, 0xf0, 0x80, 0x40, 0x22, 0x91, 0x02, 0x00,
				0x00, 0x90,
			},
			machine:     elf.EM_AARCH64,
			codeAddress: 0x4e4824,
			expected:    0x4e4890,
		},
	}
	for _, td := range testdata {
		var res libpf.SymbolValue
		var err error
		switch td.machine {
		case elf.EM_X86_64:
			res, err = retrieveExecuteExJumpLabelAddressX86(td.code, td.codeAddress)
		case elf.EM_AARCH64:
			res, err = retrieveExecuteExJumpLabelAddressARM(td.code, td.codeAddress)
		}
		require.NoError(t, err)
		assert.Equal(t, td.expected, res)
	}
}

func TestRetrieveJITBufferPtr(t *testing.T) {
	testdata := []struct {
		code         []byte
		machine      elf.Machine
		codeAddress  libpf.SymbolValue
		expectedBuf  libpf.SymbolValue
		expectedSize libpf.SymbolValue
	}{
		{
			code: []byte{
				// 000CC930 	F3 0F 1E FA 	endbr64
				// 000CC934 	48 F7 05 99 7E 03 00 20 01 00 00
				//					test 	qword ptr [rip + 0x37e99], 0x120
				// 000CC93F 	74 07 	je 	0xcc948
				// 000CC941 	C3 	ret
				// 000CC942 	66 0F 1F 44 00 00 	nop 	word ptr [rax + rax]
				// 000CC948 	55 	push 	rbp
				// 000CC949 	BA 03 00 00 00 	mov 	edx, 3
				// 000CC94E 	48 89 E5 	mov 	rbp, rsp
				// 000CC951 	53 	push 	rbx
				// 000CC952 	48 83 EC 08 	sub 	rsp, 8
				// 000CC956 	48 8B 35 9B 84 03 00 	mov 	rsi, qword ptr [rip + 0x3849b] <<
				// 000CC95D 	48 8B 3D AC 84 03 00 	mov 	rdi, qword ptr [rip + 0x384ac] <<
				// 000CC964 	E8 E7 B9 F4 FF 	call 	0x18350
				// 000CC969 	85 C0 	test 	eax, eax
				// 000CC96B 	75 0B 	jne 	0xcc978
				0xf3, 0x0f, 0x1e, 0xfa, 0x48, 0xf7, 0x05, 0x99, 0x7e, 0x03, 0x00,
				0x20, 0x01, 0x00, 0x00, 0x74, 0x07, 0xc3, 0x66, 0x0f, 0x1f, 0x44,
				0x00, 0x00, 0x55, 0xba, 0x03, 0x00, 0x00, 0x00, 0x48, 0x89, 0xe5,
				0x53, 0x48, 0x83, 0xec, 0x08, 0x48, 0x8b, 0x35, 0x9b, 0x84, 0x03,
				0x00, 0x48, 0x8b, 0x3d, 0xac, 0x84, 0x03, 0x00, 0xe8, 0xe7, 0xb9,
				0xf4, 0xff, 0x85, 0xc0, 0x75, 0x0b,
			},
			machine:      elf.EM_X86_64,
			codeAddress:  0xcc930,
			expectedBuf:  0x104e10,
			expectedSize: 0x104df8,
		},
		{
			code: []byte{
				//146720:        900005e1         adrp    x1,  0x202000
				//146724:        91048021         add     x1,  x1,  #0x120
				//146728:        d2802400         mov     x0,  #0x120                      // #288
				//14672c:        f9400c22         ldr     x2,  [x1,  #24]
				//146730:        ea00005f         tst     x2,  x0
				//146734:        54000040         b.eq    0x14673c  // b.none
				//146738:        d65f03c0         ret
				//14673c:        a9be7bfd         stp     x29,  x30,  [sp,  #-32]!
				//146740:        52800062         mov     w2,  #0x3                        // #3
				//146744:        910003fd         mov     x29,  sp
				//146748:        f941a820         ldr     x0,  [x1,  #848] // <<<<
				//14674c:        f943a021         ldr     x1,  [x1,  #1856] // <<<<
				//146750:        97fb1360         bl      0xb4d0
				//146754:        35000060         cbnz    w0,  0x146760
				//146758:        a8c27bfd         ldp     x29,  x30,  [sp],  #32
				//14675c:        d65f03c0         ret
				0xe1, 0x05, 0x00, 0x90, 0x21, 0x80, 0x04, 0x91, 0x00, 0x24, 0x80, 0xd2, 0x22, 0x0c,
				0x40, 0xf9, 0x5f, 0x00, 0x00, 0xea, 0x40, 0x00, 0x00, 0x54, 0xc0, 0x03, 0x5f, 0xd6,
				0xfd, 0x7b, 0xbe, 0xa9, 0x62, 0x00, 0x80, 0x52, 0xfd, 0x03, 0x00, 0x91, 0x20, 0xa8,
				0x41, 0xf9, 0x21, 0xa0, 0x43, 0xf9, 0x60, 0x13, 0xfb, 0x97, 0x60, 0x00, 0x00, 0x35,
				0xfd, 0x7b, 0xc2, 0xa8, 0xc0, 0x03, 0x5f, 0xd6,
			},
			machine:      elf.EM_AARCH64,
			codeAddress:  0x146720,
			expectedBuf:  0x202470,
			expectedSize: 0x202860,
		},
	}
	for _, td := range testdata {
		var buf libpf.SymbolValue
		var sz libpf.SymbolValue
		var err error
		switch td.machine {
		case elf.EM_AARCH64:
			buf, sz, err = retrieveJITBufferPtrARM(td.code, td.codeAddress)
		case elf.EM_X86_64:
			buf, sz, err = retrieveJITBufferPtrx86(td.code, td.codeAddress)
		}
		require.NoError(t, err)
		assert.Equal(t, td.expectedBuf, buf)
		assert.Equal(t, td.expectedSize, sz)
	}
}
