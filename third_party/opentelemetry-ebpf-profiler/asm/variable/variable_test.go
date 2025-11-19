package variable

import (
	"math"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVariable(t *testing.T) {
	t.Run("add sort-summ-immediate", func(t *testing.T) {
		v := Var("v")
		assertEqualRecursive(t,
			Add(v, Imm(14)),
			Add(Imm(1), Imm(3), Imm(1), v, Imm(9)),
		)
	})

	t.Run("add 0", func(t *testing.T) {
		v := Var("v")
		assertEqualRecursive(t,
			v,
			Add(Imm(0), v),
		)
	})

	t.Run("add nested", func(t *testing.T) {
		s1 := Var("s1")
		s2 := Var("s2")
		s3 := Var("s3")
		assertEqualRecursive(t,
			Add(Add(s1, s3), s2),
			Add(s1, s3, s2),
		)
		assertEqualRecursive(t,
			Add(Add(s1, s3), s2),
			Add(s2, s3, s1),
		)
	})

	t.Run("add opt", func(t *testing.T) {
		v := Var("v")
		assertEqualRecursive(t,
			Add(Add(Imm(2), v), Imm(7)),
			Add(v, Imm(9)),
		)
	})

	t.Run("add 1 element", func(t *testing.T) {
		assertEqualRecursive(t,
			Add(Imm(2)),
			Imm(2),
		)
	})

	t.Run("mul immediate", func(t *testing.T) {
		v := Var("v")
		assertEqualRecursive(t,
			Multiply(v, Imm(27)),
			Multiply(Imm(1), Imm(3), Imm(1), v, Imm(9)),
		)
	})

	t.Run("mul 1", func(t *testing.T) {
		v := Var("v")

		assertEqualRecursive(t,
			v,
			Multiply(Imm(1), v),
		)
	})

	t.Run("mul add", func(t *testing.T) {
		v1 := Var("v1")
		v2 := Var("v2")
		v3 := Var("v3")
		assertEqualRecursive(t,
			Add(Multiply(v1, v3), Multiply(v2, v3)),
			Multiply(Add(v1, v2), v3),
		)
	})

	t.Run("mul order", func(t *testing.T) {
		v := Var("v")

		assertEqualRecursive(t,
			&op{opMul, []Expression{v, Imm(239)}},
			Multiply(Imm(239), v),
		)
	})

	t.Run("mul 0", func(t *testing.T) {
		v := Var("v")

		assertEqualRecursive(t,
			Imm(0),
			Multiply(Imm(0), Imm(3), Imm(1), v, Imm(9)),
		)
	})

	t.Run("extend nested", func(t *testing.T) {
		v := Var("v")

		assertEqualRecursive(t,
			ZeroExtend(v, 7),
			ZeroExtend(ZeroExtend(v, 7), 7),
		)
	})

	t.Run("extend nested smaller", func(t *testing.T) {
		v := Var("v")

		assertEqualRecursive(t,
			ZeroExtend(v, 5),
			ZeroExtend(ZeroExtend(v, 7), 5),
		)
	})
	t.Run("extend nested smaller", func(t *testing.T) {
		v := Var("v")

		assertEqualRecursive(t,
			ZeroExtend(v, 5),
			ZeroExtend(ZeroExtend(v, 5), 7),
		)
	})

	t.Run("extend max1", func(t *testing.T) {
		maxFF := Var("ff").SetMaxValue(0xff)
		assertEqualRecursive(t,
			maxFF,
			ZeroExtend(maxFF, 11),
		)
	})

	t.Run("extend max value", func(t *testing.T) {
		maxFF := Var("ff").SetMaxValue(0xff)
		assert.EqualValues(t,
			0b1111111,
			ZeroExtend(maxFF, 7).MaxValue(),
		)
	})

	t.Run("extend max value", func(t *testing.T) {
		v := Var("v")

		assert.EqualValues(t,
			math.MaxUint32,
			ZeroExtend(v, 32).MaxValue(),
		)
	})

	t.Run("extend max value", func(t *testing.T) {
		v := Var("v")

		assert.Equal(t,
			uint64(math.MaxUint64),
			ZeroExtend(v, 64).MaxValue(),
		)
	})

	t.Run("add max value overflow", func(t *testing.T) {
		assert.Equal(t,
			uint64(math.MaxUint64),
			Add(Var("max64"), Var("max1").SetMaxValue(1)).MaxValue(),
		)
	})

	t.Run("any", func(t *testing.T) {
		assert.False(t,
			Any().Match(Var("v1")),
		)
		assert.True(t,
			Var("v1").Match(Any()),
		)
	})

	t.Run("extend 0", func(t *testing.T) {
		assert.EqualValues(t,
			0,
			ZeroExtend(Var("v1"), 0).MaxValue(),
		)
		assertEqualRecursive(t,
			Imm(0),
			ZeroExtend(Var("v1"), 0),
		)
	})

	t.Run("nested extend ", func(t *testing.T) {
		v1 := Var("v1")
		assertEqualRecursive(t,
			ZeroExtend(v1, 8),
			ZeroExtend(ZeroExtend(v1, 8), 8),
		)
	})

	t.Run("any matches ops", func(t *testing.T) {
		assert.True(t, Add(Var("v1"), Var("2")).Match(Any()))
	})
}

func assertEqualRecursive(t *testing.T, a, b Expression) {
	if !equalRecursive(a, b) {
		t.Errorf("expected %s to be recursive equal to %s", a.DebugString(), b.DebugString())
	}
}

func equalRecursive(a, b Expression) bool {
	if ima, aok := a.(*immediate); aok {
		if imb, bok := b.(*immediate); bok {
			return ima.Value == imb.Value
		}
		return false
	}
	if ima, aok := a.(*Variable); aok {
		if imb, bok := b.(*Variable); bok {
			return ima == imb
		}
		return false
	}
	if ima, aok := a.(*mem); aok {
		if imb, bok := b.(*mem); bok {
			return ima.segment == imb.segment && equalRecursive(ima.at, imb.at)
		}
		return false
	}
	if ima, aok := a.(*extend); aok {
		if imb, bok := b.(*extend); bok {
			return ima.bits == imb.bits && equalRecursive(ima.v, imb.v)
		}
		return false
	}
	if ima, aok := a.(*op); aok {
		if imb, bok := b.(*op); bok {
			return ima.typ == imb.typ && equalOperands(ima.operands, imb.operands)
		}
		return false
	}
	return false
}

func equalOperands(a, b operands) bool {
	if len(a) != len(b) {
		return false
	}
	acopy := make(operands, len(a))
	copy(acopy, a)
	for _, it := range b {
		eq := false
		for j, jit := range acopy {
			if equalRecursive(it, jit) {
				eq = true
				acopy = slices.Delete(acopy, j, j+1)
				break
			}
		}
		if !eq {
			return false
		}
	}
	return len(acopy) == 0
}
