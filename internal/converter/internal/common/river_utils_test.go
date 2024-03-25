package common_test

import (
	"testing"

	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestDefaultValue(t *testing.T) {
	var explicitDefault defaultingType
	explicitDefault.SetToDefault()

	require.Equal(t, explicitDefault, common.DefaultValue[defaultingType]())
}

type defaultingType struct {
	Number int
}

var _ syntax.Defaulter = (*defaultingType)(nil)

func (dt *defaultingType) SetToDefault() {
	dt.Number = 42
}
