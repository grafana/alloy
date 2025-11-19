// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expression

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpression(t *testing.T) {
	t.Run("add sort-summ-immediate", func(t *testing.T) {
		v := Named("v")
		require.Equal(t, Add(v, Imm(14)), Add(Imm(1), Imm(3), Imm(1), v, Imm(9)))
	})

	t.Run("named match", func(t *testing.T) {
		n := Named("v")
		require.True(t, n.Match(n))
		require.False(t, n.Match(Imm(239)))
	})

	t.Run("add 0", func(t *testing.T) {
		v := Named("v")
		require.Equal(t, v, Add(Imm(0), v))
	})

	t.Run("add nested", func(t *testing.T) {
		s1 := Named("s1")
		s2 := Named("s2")
		s3 := Named("s3")
		performAssertions := func(e Expression) {
			opp, ok := e.(*op)
			require.True(t, ok)
			require.Len(t, opp.operands, 3)
			require.Contains(t, opp.operands, s1)
			require.Contains(t, opp.operands, s2)
			require.Contains(t, opp.operands, s3)
		}
		performAssertions(Add(Add(s1, s3), s2))
		performAssertions(Add(Add(s1, s3), s2))
	})

	t.Run("add opt", func(t *testing.T) {
		v := Named("v")
		require.Equal(t, Add(Add(Imm(2), v), Imm(7)), Add(v, Imm(9)))
	})

	t.Run("add 1 element", func(t *testing.T) {
		require.Equal(t, Add(Imm(2)), Imm(2))
	})

	t.Run("mul immediate", func(t *testing.T) {
		v := Named("v")
		require.Equal(t, Multiply(v, Imm(27)), Multiply(Imm(1), Imm(3), Imm(1), v, Imm(9)))
	})

	t.Run("mul 1", func(t *testing.T) {
		v := Named("v")

		require.Equal(t, v, Multiply(Imm(1), v))
	})

	t.Run("mul add", func(t *testing.T) {
		v1 := Named("v1")
		v2 := Named("v2")
		v3 := Named("v3")
		require.Equal(t, Add(Multiply(v1, v3), Multiply(v2, v3)), Multiply(Add(v1, v2), v3))
	})

	t.Run("op order", func(t *testing.T) {
		v := Named("v")
		v2 := Mem8(Named("v2"))
		require.True(t, Multiply(v, v2).Match(Multiply(v2, v)))
	})

	t.Run("mul order", func(t *testing.T) {
		v := Named("v")

		var a Expression = &op{opMul, []Expression{v, Imm(239)}}
		require.Equal(t, a, Multiply(Imm(239), v))
	})

	t.Run("mul 0", func(t *testing.T) {
		v := Named("v")

		require.Equal(t, Imm(0), Multiply(Imm(0), Imm(3), Imm(1), v, Imm(9)))
	})

	t.Run("extend nested", func(t *testing.T) {
		v := Named("v")

		require.Equal(t, ZeroExtend(v, 7), ZeroExtend(ZeroExtend(v, 7), 7))
	})

	t.Run("extend nested smaller", func(t *testing.T) {
		v := Named("v")

		require.Equal(t, ZeroExtend(v, 5), ZeroExtend(ZeroExtend(v, 7), 5))
	})
	t.Run("extend nested smaller", func(t *testing.T) {
		v := Named("v")

		require.Equal(t, ZeroExtend(v, 5), ZeroExtend(ZeroExtend(v, 5), 7))
	})

	t.Run("extend 0", func(t *testing.T) {
		require.Equal(t, Imm(0), ZeroExtend(Named("v1"), 0))
	})

	t.Run("nested extend ", func(t *testing.T) {
		v1 := Named("v1")
		require.Equal(t, ZeroExtend(v1, 8), ZeroExtend(ZeroExtend(v1, 8), 8))
	})
}
