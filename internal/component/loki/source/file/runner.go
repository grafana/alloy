package file

import (
	"context"
	"time"

	"github.com/grafana/alloy/internal/runner"
	"github.com/grafana/dskit/backoff"
)

var _ runner.Task = (*runnerTask)(nil)

type runnerTask struct {
	reader     reader
	path       string
	labels     string
	readerHash uint64
}

func (r *runnerTask) Hash() uint64 {
	return r.readerHash
}

func (r *runnerTask) Equals(other runner.Task) bool {
	otherTask := other.(*runnerTask)

	if r == otherTask {
		return true
	}

	return r.readerHash == otherTask.readerHash
}

// runnerReader is a wrapper around a reader (tailer or decompressor)
// It is responsible for running the reader. If the reader stops running,
// it will retry it after a few seconds. This is useful to handle log file rotation
// when a file might be gone for a very short amount of time.
// The runner is only stopped when the corresponding target is gone or when the component is stopped.
type runnerReader struct {
	reader reader
}

func (r *runnerReader) Run(ctx context.Context) {
	backoff := backoff.New(
		ctx,
		backoff.Config{
			MinBackoff: 1 * time.Second,
			MaxBackoff: 10 * time.Second,
			MaxRetries: 0,
		},
	)

	for {
		r.reader.Run(ctx)
		backoff.Wait()
		if !backoff.Ongoing() {
			break
		}
	}
}
