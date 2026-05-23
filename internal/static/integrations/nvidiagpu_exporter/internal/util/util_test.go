package util_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/static/integrations/nvidiagpu_exporter/internal/util"
)

func TestToSnakeCase(t *testing.T) {
	t.Parallel()

	snakeCase := util.ToSnakeCase("aaaAAA_aaaAaa")

	assert.Equal(t, "aaa_aaa_aaa_aaa", snakeCase)
}

func TestHexToDecimal(t *testing.T) {
	t.Parallel()

	decimal, err := util.HexToDecimal("0x40051458")

	require.NoError(t, err)
	assert.True(t, almostEqual(decimal, 1074074712.0))
}

func TestHexToDecimalError(t *testing.T) {
	t.Parallel()

	_, err := util.HexToDecimal("SOMETHING")

	require.Error(t, err)
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}
