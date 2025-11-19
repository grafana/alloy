// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package amd

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/asm/expression"
)

func BenchmarkPythonInterpreter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testPythonInterpreter(b)
	}
}

func TestPythonInterpreter(t *testing.T) {
	testPythonInterpreter(t)
}

func testPythonInterpreter(t testing.TB) {
	// 00010000 	4D 89 F2 	mov 	r10, r14
	// 00010003 	45 0F B6 36 	movzx 	r14d, byte ptr [r14]
	// 00010007 	48 8D 05 2D B3 35 00 	lea 	rax, [rip + 0x35b32d]
	// 0001000E 	4C 8B 6C 24 08 	mov 	r13, qword ptr [rsp + 8]
	// 00010013 	48 89 C1 	mov 	rcx, rax
	// 00010016 	48 89 44 24 10 	mov 	qword ptr [rsp + 0x10], rax
	// 0001001B 	45 0F B6 5A 01 	movzx 	r11d, byte ptr [r10 + 1]
	// 00010020 	41 0F B6 C6 	movzx 	eax, r14b
	// 00010024 	48 8B 04 C1 	mov 	rax, qword ptr [rcx + rax*8]
	// 00010028 	FF E0 	jmp 	rax
	code := []byte{
		0x4d, 0x89, 0xf2, 0x45, 0x0f, 0xb6, 0x36, 0x48, 0x8d, 0x05, 0x2d, 0xb3, 0x35,
		0x00, 0x4c, 0x8b, 0x6c, 0x24, 0x08, 0x48, 0x89, 0xc1, 0x48, 0x89, 0x44, 0x24,
		0x10, 0x45, 0x0f, 0xb6, 0x5a, 0x01, 0x41, 0x0f, 0xb6, 0xc6, 0x48, 0x8b, 0x04,
		0xc1, 0xff, 0xe0,
	}
	it := NewInterpreterWithCode(code)
	it.CodeAddress = expression.Imm(0x8AF05)
	r14 := it.Regs.Get(R14)
	_, err := it.Loop()
	if err == nil || err != io.EOF {
		t.Fatal(err)
	}
	actual := it.Regs.Get(RAX)
	expected := expression.Mem(
		expression.Add(
			expression.Multiply(
				expression.ZeroExtend8(expression.Mem1(r14)),
				expression.Imm(8),
			),
			expression.NewImmediateCapture("switch table"),
		),
		8,
	)
	if !actual.Match(expected) {
		t.Fatal()
	}
}

func TestRecoverSwitchCase(t *testing.T) {
	blocks := []CodeBlock{
		{
			Address: expression.Imm(0x3310E3),
			// 003310E3 	48 8B 44 24 20 	mov 	rax, qword ptr [rsp + 0x20]
			// 003310E8 	48 89 18 	mov 	qword ptr [rax], rbx
			// 003310EB 	49 83 C2 02 	add 	r10, 2
			// 003310EF 	44 89 E0 	mov 	eax, r12d
			// 003310F2 	83 E0 03 	and 	eax, 3
			// 003310F5 	31 DB 	xor 	ebx, ebx
			// 003310F7 	41 F6 C4 04 	test 	r12b, 4
			// 003310FB 	4C 89 74 24 10 	mov 	qword ptr [rsp + 0x10], r14
			// 00331100 	74 08 	je 	0x33110a
			Code: []byte{0x48, 0x8b, 0x44, 0x24, 0x20, 0x48, 0x89, 0x18, 0x49,
				0x83, 0xc2, 0x02, 0x44, 0x89, 0xe0, 0x83, 0xe0, 0x03, 0x31, 0xdb,
				0x41, 0xf6, 0xc4, 0x04, 0x4c, 0x89, 0x74, 0x24, 0x10, 0x74, 0x08},
		},
		{
			Address: expression.Imm(0x33110a),
			// 0033110A 	4D 89 DC 	mov 	r12, r11
			// 0033110D 	4D 8D 47 F8 	lea 	r8, [r15 - 8]
			// 00331111 	4C 89 7C 24 60 	mov 	qword ptr [rsp + 0x60], r15
			// 00331116 	4D 8B 7F F8 	mov 	r15, qword ptr [r15 - 8]
			// 0033111A 	48 8B 0D 87 06 17 01 	mov 	rcx, qword ptr [rip + 0x1170687]
			// 00331121 	89 C0 	mov 	eax, eax
			// 00331123 	48 8D 15 02 E7 C0 00 	lea 	rdx, [rip + 0xc0e702]
			// 0033112A 	48 63 04 82 	movsxd 	rax, dword ptr [rdx + rax*4]
			// 0033112E 	48 01 D0 	add 	rax, rdx
			// 00331131 	4C 89 D5 	mov 	rbp, r10
			// 00331134 	4D 89 C5 	mov 	r13, r8
			// 00331137 	FF E0 	jmp 	rax
			Code: []byte{
				0x4d, 0x89, 0xdc, 0x4d, 0x8d, 0x47, 0xf8, 0x4c, 0x89, 0x7c, 0x24,
				0x60, 0x4d, 0x8b, 0x7f, 0xf8, 0x48, 0x8b, 0x0d, 0x87, 0x06, 0x17,
				0x01, 0x89, 0xc0, 0x48, 0x8d, 0x15, 0x02, 0xe7, 0xc0, 0x00, 0x48,
				0x63, 0x04, 0x82, 0x48, 0x01, 0xd0, 0x4c, 0x89, 0xd5, 0x4d, 0x89,
				0xc5, 0xff, 0xe0,
			},
		},
	}
	it := NewInterpreter()
	initR12 := it.Regs.Get(R12)
	it.ResetCode(blocks[0].Code, blocks[0].Address)
	_, err := it.Loop()
	require.ErrorIs(t, err, io.EOF)

	expected := expression.ZeroExtend(initR12, 2)
	assertEval(t, it.Regs.Get(RAX), expected)
	it.ResetCode(blocks[1].Code, blocks[1].Address)
	_, err = it.Loop()
	require.ErrorIs(t, err, io.EOF)
	table := expression.NewImmediateCapture("table")
	base := expression.NewImmediateCapture("base")
	expected = expression.Add(
		expression.SignExtend(
			expression.Mem(
				expression.Add(
					expression.Multiply(
						expression.ZeroExtend(initR12, 2),
						expression.Imm(4),
					),
					table,
				),
				4,
			),
			64,
		),
		base,
	)
	assertEval(t, it.Regs.Get(RAX), expected)
	assert.EqualValues(t, 0xf3f82c, table.CapturedValue())
	assert.EqualValues(t, 0xf3f82c, base.CapturedValue())
}

func assertEval(t *testing.T, left, right expression.Expression) {
	if !left.Match(right) {
		assert.Failf(t, "failed to eval %s to %s", left.DebugString(), right.DebugString())
		t.Logf("left  %s", left.DebugString())
		t.Logf("right %s", right.DebugString())
	}
}

func FuzzInterpreter(f *testing.F) {
	f.Fuzz(func(_ *testing.T, code []byte) {
		i := NewInterpreterWithCode(code)
		_, _ = i.Loop()
	})
}

func TestMoveSignExtend(t *testing.T) {
	i := NewInterpreterWithCode([]byte{
		// 00000000 	B8 01 00 00 00 	mov 	eax, 1
		// 00000005 	8B 40 04 	mov 	eax, dword ptr [rax + 4]
		// 00000008 	B8 02 00 00 00 	mov 	eax, 2
		// 0000000D 	48 0F B6 40 04 	movzx 	rax, byte ptr [rax + 4]
		// 00000012 	B8 03 00 00 00 	mov 	eax, 3
		// 00000017 	48 0F BF 40 04 	movsx 	rax, word ptr [rax + 4]
		0xB8, 0x01, 0x00, 0x00, 0x00, 0x8B, 0x40, 0x04,
		0xB8, 0x02, 0x00, 0x00, 0x00, 0x48, 0x0F, 0xB6,
		0x40, 0x04, 0xB8, 0x03, 0x00, 0x00, 0x00, 0x48,
		0x0F, 0xBF, 0x40, 0x04,
	})
	_, err := i.Loop()
	require.ErrorIs(t, err, io.EOF)
	pattern := expression.SignExtend(expression.Mem(expression.Imm(7), 2), 64)
	require.True(t, i.Regs.Get(RAX).Match(pattern))
}
