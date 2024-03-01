package vault

import (
	"time"
)

// ticker is a wrapper around time.Ticker which allows the tick time to be 0.
// ticker is not goroutine safe; do not call Chan at the same time as Reset.
type ticker struct {
	ch <-chan time.Time

	inner *time.Ticker
}

func newTicker(d time.Duration) *ticker {
	var t ticker
	t.Reset(d)

	return &t
}

func (t *ticker) Chan() <-chan time.Time { return t.ch }

func (t *ticker) Reset(d time.Duration) {
	if d == 0 {
		t.Stop()
		return
	}

	if t.inner == nil {
		t.inner = time.NewTicker(d)
		t.ch = t.inner.C
	} else {
		t.inner.Reset(d)
	}
}

func (t *ticker) Stop() {
	if t.inner != nil {
		t.inner.Stop()
		t.inner = nil
	}
}
