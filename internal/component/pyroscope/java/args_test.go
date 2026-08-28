package java

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArguments_Validate(t *testing.T) {
	t.Run("tlab without alloc is invalid", func(t *testing.T) {
		args := DefaultArguments()
		args.ProfilingConfig.Tlab = true
		args.ProfilingConfig.Alloc = ""
		require.Error(t, args.Validate())
	})

	t.Run("tlab with alloc is valid", func(t *testing.T) {
		args := DefaultArguments()
		args.ProfilingConfig.Tlab = true
		args.ProfilingConfig.Alloc = "512k"
		require.NoError(t, args.Validate())
	})

	t.Run("tlab false without alloc is valid", func(t *testing.T) {
		args := DefaultArguments()
		args.ProfilingConfig.Tlab = false
		args.ProfilingConfig.Alloc = ""
		require.NoError(t, args.Validate())
	})
}
