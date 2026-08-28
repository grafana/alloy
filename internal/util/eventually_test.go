package util

import (
	"testing"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/require"
)

func TestEventually(t *testing.T) {
	var bcfg = backoff.Config{
		MinBackoff: 10 * time.Millisecond,
		MaxBackoff: 10 * time.Millisecond,
		MaxRetries: 5,
	}

	t.Run("No errors", func(t *testing.T) {
		EventuallyWithBackoff(t, func(t require.TestingT) {
			require.True(t, true)
		}, bcfg)
	})

	t.Run("Fails once", func(t *testing.T) {
		var runs int

		EventuallyWithBackoff(t, func(t require.TestingT) {
			if runs > 0 {
				return
			}
			runs++

			require.True(t, false)
		}, bcfg)
	})

	t.Run("Always fails", func(t *testing.T) {
		var et eventuallyT

		defer func() {
			err := recover()
			if err == nil {
				require.Fail(t, "expected panic")
			}

			_, aborted := err.(testAbort)
			if !aborted {
				require.Fail(t, "expected test abort")
			}

			require.Len(t, et.errors, 1)
		}()

		EventuallyWithBackoff(&et, func(t require.TestingT) {
			require.True(t, false)
		}, bcfg)
	})
}
