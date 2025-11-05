package string

import (
	"context"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestString(t *testing.T) {
	opts := component.Options{
		Logger: log.NewNopLogger(),
	}

	type arguments struct {
		expected string
		args     Arguments
	}
	receiver := loki.NewLogsReceiver()
	argsArray := []arguments{
		{
			expected: "{\"data\":\"pass1\"}",
			args: Arguments{
				Source:    "{\"data\":\"pass1\"}",
				ForwardTo: receiver,
			},
		},
		{
			expected: "{\"key\":\"pass\",\"nestedData\":{\"nestedKey\":\"pass\"}}",
			args: Arguments{
				Source:    "{\"key\":\"pass\",\"nestedData\":{\"nestedKey\":\"pass\"}}",
				ForwardTo: receiver,
			},
		},
	}

	initArgs := arguments{
		expected: "{\"data\":\"init\"}",
		args: Arguments{
			Source:    "{\"data\":\"init\"}",
			ForwardTo: receiver,
		},
	}

	comp, err := New(opts, initArgs.args)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err = comp.Run(ctx)
	}()

	// Make sure the first argument is received
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		select {
		case received := <-receiver.Chan():
			assert.Equal(c, initArgs.expected, received.Line)
		default:
			assert.Fail(c, "log entry not received yet")
		}
	}, time.Second, 10*time.Millisecond)

	// Subsequent update should be received
	for _, testArgs := range argsArray {
		comp.Update(testArgs.args)

		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			select {
			case received := <-receiver.Chan():
				assert.Equal(c, testArgs.expected, received.Line)
			default:
				assert.Fail(c, "log entry not received yet")
			}
		}, time.Second, 10*time.Millisecond)
	}
}
