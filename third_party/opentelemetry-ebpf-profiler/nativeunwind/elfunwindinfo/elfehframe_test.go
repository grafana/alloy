// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elfunwindinfo

import (
	"testing"

	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	sdtypes "go.opentelemetry.io/ebpf-profiler/nativeunwind/stackdeltatypes"
	"go.opentelemetry.io/ebpf-profiler/support"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ehtester struct {
	t     *testing.T
	res   map[uintptr]sdtypes.UnwindInfo
	found int
}

func (e *ehtester) fdeUnsorted() {
}

func (e *ehtester) fdeHook(cie *cieInfo, fde *fdeInfo, _ *sdtypes.StackDeltaArray) bool {
	e.t.Logf("FDE ciePos %x, ip %x...%x, ipLen %d (enc %x, cf %d, df %d, ra %d)",
		fde.ciePos, fde.ipStart, fde.ipStart+fde.ipLen, fde.ipLen,
		cie.enc, cie.codeAlign, cie.dataAlign, cie.regRA)
	e.t.Logf("   LOC           CFA          rbp   ra")
	return true
}

func (e *ehtester) deltaHook(ip uintptr, regs *vmRegs, delta sdtypes.StackDelta) {
	e.t.Logf("%016x %-12s %-5s %s",
		ip,
		regs.cfa.String(),
		regs.fp.String(),
		regs.ra.String())
	if expected, ok := e.res[ip]; ok {
		assert.Equal(e.t, expected, delta.Info)
		e.found++
	}
}

func (e *ehtester) golangHook(_, _ uintptr) {
}

func genDelta(opcode uint8, cfa, rbp int32) sdtypes.UnwindInfo {
	res := sdtypes.UnwindInfo{
		Opcode: opcode,
		Param:  cfa,
	}
	if rbp != 0 {
		res.FPOpcode = support.UnwindOpcodeBaseCFA
		res.FPParam = -rbp
	}
	return res
}

func deltaRSP(cfa, rbp int32) sdtypes.UnwindInfo {
	return genDelta(support.UnwindOpcodeBaseSP, cfa, rbp)
}

func deltaRBP(cfa, rbp int32) sdtypes.UnwindInfo {
	return genDelta(support.UnwindOpcodeBaseFP, cfa, rbp)
}

func TestEhFrame(t *testing.T) {
	tests := map[string]struct {
		elfFile string
		// Some selected stack delta matches to verify that the ehframe
		// machine is working correctly.
		res map[uintptr]sdtypes.UnwindInfo
	}{
		// test.so is openssl libcrypto.so.1.1's stripped to contain only .eh_frame and
		// .eh_frame_hdr. The current ELF is imported from Alpine Linux
		// openssl-1.1.1g-r0 package's libcrypto.so.1.1:
		//   objcopy -j .eh_frame -j .eh_frame_hdr /lib/libcrypto.so.1.1 test.so
		"libcrypto": {elfFile: "testdata/test.so",
			res: map[uintptr]sdtypes.UnwindInfo{
				0x07631f: deltaRSP(8, 0),
				0x07a0d4: deltaRSP(160, 24),
				0x07b1ec: deltaRSP(8, 0),
				0x088e72: deltaRSP(64, 48),
				0x0a89d9: deltaRBP(16, 16),
				0x0b2ad4: deltaRBP(8, 24),
				0x1c561f: deltaRSP(2160, 48),
			}},
		// schrodinger-libpython3.8.so.1.0 is a stripped version containing only .eh_frame and
		// .eh_frame_hdr from /exports/schrodinger/internal/lib/libpython3.8.so.1.0 - see PF-1538.
		//   objcopy -j .eh_frame -j .eh_frame_hdr /lib/libcrypto.so.1.1 test.so
		"schrodinger-libpython": {elfFile: "testdata/schrodinger-libpython3.8.so.1.0",
			res: map[uintptr]sdtypes.UnwindInfo{
				0x6f805:  deltaRSP(80, 48),
				0x7077c:  deltaRSP(24, 0),
				0x83194:  deltaRSP(64, 16),
				0x954b4:  deltaRSP(48, 48),
				0xc8b9e:  deltaRSP(112, 48),
				0xd2f5e:  deltaRSP(56, 48),
				0xf01cf:  deltaRSP(24, 24),
				0x1a87b2: deltaRSP(40, 40),
				0x23f555: deltaRSP(56, 48),
			}},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ef, err := pfelf.Open(test.elfFile)
			require.NoError(t, err)
			defer ef.Close()

			tester := ehtester{t, test.res, 0}
			ee := elfExtractor{
				file:   ef,
				deltas: &sdtypes.StackDeltaArray{},
				hooks:  &tester,
			}
			err = ee.parseEHFrame()
			require.NoError(t, err)
			assert.Equal(t, len(test.res), tester.found)
		})
	}
}

func TestParseCIE(t *testing.T) {
	tests := map[string]struct {
		data       []byte
		expected   *cieInfo
		debugFrame bool
	}{
		// Call frame information example for version 4.
		// http://dwarfstd.org/doc/DWARF5.pdf Table D.5 "Call frame information example"
		"cie 4": {
			debugFrame: true,
			expected: &cieInfo{
				dataAlign: sleb128(-4),
				codeAlign: uleb128(4),
				regRA:     uleb128(8),
			},
			data: []byte{36, 0, 0, 0, // length
				255, 255, 255, 255, // CIE_id
				4,        // version
				0,        // augmentation
				4,        // address size
				0,        // segment size
				4,        // code_alignment_factor
				124,      // data_alignment_factor
				8,        // R8 is the return address
				12, 7, 0, // CFA = [R7]+0
				8, 0, // R0 not modified
				7, 1, // R1 scratch
				7, 2, // R2 scratch
				7, 3, // R3 scratch
				8, 4, // R4 preserve
				8, 5, // R5 preserve
				8, 6, // R6 preserve
				8, 7, // R7 preserve
				9, 8, 1, // R8 is in R1
				0, // DW_CFA_nop
				0, // DW_CFA_nop
				0, // DW_CFA_nop
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fakeReader := &reader{
				debugFrame: tc.debugFrame,
				data:       tc.data,
				end:        uintptr(len(tc.data)),
			}
			extracted := &cieInfo{}
			err := fakeReader.parseCIE(extracted)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, extracted)
		})
	}
}
