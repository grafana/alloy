package util

import (
	"context"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/require"
)

var backoffRetry = backoff.Config{
	MinBackoff: 10 * time.Millisecond,
	MaxBackoff: 1 * time.Second,
	MaxRetries: 10,
}

// Eventually calls the check function several times until it doesn't report an
// error. Failing the test in the t provided to check does not fail the test
// until the provided backoff.Config is out of retries.
func Eventually(t require.TestingT, check func(t require.TestingT)) {
	EventuallyWithBackoff(t, check, backoffRetry)
}

func EventuallyWithBackoff(t require.TestingT, check func(t require.TestingT), bc backoff.Config) {
	bo := backoff.New(context.Background(), bc)

	var (
		lastErrors  []testError
		shouldAbort bool
	)

	for bo.Ongoing() {
		ev := invokeCheck(check)

		lastErrors = ev.errors
		shouldAbort = ev.aborted

		if len(ev.errors) == 0 {
			break
		}

		bo.Wait()
	}

	if bo.Err() != nil {
		// Forward the last set of received errors back to our real test.
		for _, err := range lastErrors {
			t.Errorf(err.format, err.args...)
		}

		if shouldAbort {
			t.FailNow()
		}
	}
}

func invokeCheck(check func(t require.TestingT)) (result eventuallyT) {
	defer func() {
		if err := recover(); err != nil {
			if _, ok := err.(testAbort); ok {
				return
			}
			// Unexpected panic; raise it back.
			panic(err)
		}
	}()

	check(&result)
	return
}

type eventuallyT struct {
	// Populated by calls to Errorf.
	errors  []testError
	aborted bool
}

// Helper types for eventuallyT,.
type (
	testError struct {
		format string
		args   []interface{}
	}

	// testAbort is sent when FailNow is called.
	testAbort struct{}
)

var _ require.TestingT = (*eventuallyT)(nil)

func (et *eventuallyT) Errorf(format string, args ...interface{}) {
	et.errors = append(et.errors, testError{
		format: format,
		args:   args,
	})
}

func (et *eventuallyT) FailNow() {
	et.aborted = true
	panic(testAbort{})
}
