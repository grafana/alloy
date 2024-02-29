package common_test

import (
	"testing"

	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/river"
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

var _ river.Defaulter = (*defaultingType)(nil)

func (dt *defaultingType) SetToDefault() {
	dt.Number = 42
}
