package kubernetes

import (
	"testing"
	"time"

	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestRiverUnmarshal(t *testing.T) {
	riverCfg := `
		name = "foo"
		namespace = "bar"
		poll_frequency = "10m"
		poll_timeout = "1s"`

	var args Arguments
	err := river.Unmarshal([]byte(riverCfg), &args)
	require.NoError(t, err)

	assert.Equal(t, 10*time.Minute, args.PollFrequency)
	assert.Equal(t, time.Second, args.PollTimeout)
	assert.Equal(t, "foo", args.Name)
	assert.Equal(t, "bar", args.Namespace)
}
func TestValidate(t *testing.T) {
	t.Run("0 Poll Freq", func(t *testing.T) {
		args := Arguments{}
		args.SetToDefault()
		args.PollFrequency = 0
		err := args.Validate()
		require.ErrorContains(t, err, "poll_frequency must be greater than 0")
	})
	t.Run("negative Poll timeout", func(t *testing.T) {
		args := Arguments{}
		args.SetToDefault()
		args.PollTimeout = 0
		err := args.Validate()
		require.ErrorContains(t, err, "poll_timeout must not be greater than 0")
	})
}
