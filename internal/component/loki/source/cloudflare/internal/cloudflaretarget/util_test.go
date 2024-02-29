package cloudflaretarget

// This code is copied from Promtail (a1c1152b79547a133cc7be520a0b2e6db8b84868).
// The cloudflaretarget package is used to configure and run a target that can
// read from the Cloudflare Logpull API and forward entries to other loki
// components.

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/grafana/cloudflare-go"
	"github.com/stretchr/testify/mock"
)

var ErrorLogpullReceived = errors.New("error logpull received")

type fakeCloudflareClient struct {
	mut sync.RWMutex
	mock.Mock
}

func (f *fakeCloudflareClient) CallCount() int {
	var actualCalls int
	f.mut.RLock()
	for _, call := range f.Calls {
		if call.Method == "LogpullReceived" {
			actualCalls++
		}
	}
	f.mut.RUnlock()
	return actualCalls
}

type fakeLogIterator struct {
	logs    []string
	current string

	err error
}

func (f *fakeLogIterator) Next() bool {
	if len(f.logs) == 0 {
		return false
	}
	f.current = f.logs[0]
	if f.current == `error` {
		f.err = errors.New("error")
		return false
	}
	f.logs = f.logs[1:]
	return true
}
func (f *fakeLogIterator) Err() error                         { return f.err }
func (f *fakeLogIterator) Line() []byte                       { return []byte(f.current) }
func (f *fakeLogIterator) Fields() (map[string]string, error) { return nil, nil }
func (f *fakeLogIterator) Close() error {
	if f.err == ErrorLogpullReceived {
		f.err = nil
	}
	return nil
}

func newFakeCloudflareClient() *fakeCloudflareClient {
	return &fakeCloudflareClient{}
}

func (f *fakeCloudflareClient) LogpullReceived(ctx context.Context, start, end time.Time) (cloudflare.LogpullReceivedIterator, error) {
	f.mut.Lock()
	defer f.mut.Unlock()

	r := f.Called(ctx, start, end)
	if r.Get(0) != nil {
		it := r.Get(0).(cloudflare.LogpullReceivedIterator)
		if it.Err() == ErrorLogpullReceived {
			return it, it.Err()
		}
		return it, nil
	}
	return nil, r.Error(1)
}
