package structwalk

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type LevelA struct {
	Field1 bool
	Field2 string
	Field3 int
	Nested LevelB
}

type LevelB struct {
	Level1 bool
	Level2 string
	Field3 int
	Nested LevelC
}

type LevelC struct {
	Level1 bool
	Level2 string
	Field3 int
}

func TestWalk(t *testing.T) {
	var (
		iteration int
		fv        FuncVisitor
	)
	fv = func(val interface{}) Visitor {
		iteration++

		// After visiting all 3 structs, should receive a w.Visit(nil) for each level
		if iteration >= 4 {
			require.Nil(t, val)
			return nil
		}

		switch iteration {
		case 1:
			require.IsType(t, LevelA{}, val)
		case 2:
			require.IsType(t, LevelB{}, val)
		case 3:
			require.IsType(t, LevelC{}, val)
		default:
			require.FailNow(t, "unexpected iteration")
		}

		return fv
	}

	var val LevelA
	Walk(fv, val)
}

type FuncVisitor func(v interface{}) Visitor

func (fv FuncVisitor) Visit(v interface{}) Visitor { return fv(v) }
