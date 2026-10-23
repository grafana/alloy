package loki

import (
	"context"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/stretchr/testify/require"
)

func TestFanout_UpdateChildrenDoesNotWaitForBlockedSend(t *testing.T) {
	removed, added := NewLogsReceiver(), NewLogsReceiver()
	f := NewFanout([]LogsReceiver{removed})

	sendErr := make(chan error, 1)
	go func() { sendErr <- f.Send(t.Context(), Entry{}) }()

	// Nothing reads removed, so Send is blocked on it.
	updated := make(chan struct{})
	go func() {
		f.UpdateChildren([]LogsReceiver{added})
		close(updated)
	}()
	select {
	case <-updated:
	case <-time.After(5 * time.Second):
		t.Fatal("UpdateChildren blocked on an in-flight Send")
	}

	// The entry goes to the new receiver instead.
	select {
	case <-added.Chan():
	case <-time.After(5 * time.Second):
		t.Fatal("entry was not redirected to the new receiver")
	}
	require.NoError(t, <-sendErr)
}

func TestFanout_KeptReceiverGetsEntryOnce(t *testing.T) {
	kept, removed, added := NewLogsReceiver(), NewLogsReceiver(), NewLogsReceiver()
	f := NewFanout([]LogsReceiver{kept, removed})

	sendErr := make(chan error, 1)
	go func() { sendErr <- f.Send(t.Context(), Entry{}) }()

	// kept gets the entry, then Send blocks on removed.
	<-kept.Chan()
	f.UpdateChildren([]LogsReceiver{kept, added})
	<-added.Chan()
	require.NoError(t, <-sendErr)

	select {
	case <-kept.Chan():
		t.Fatal("kept receiver got the entry twice")
	case <-removed.Chan():
		t.Fatal("removed receiver got the entry")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestFanout_DuplicateReceiverGetsEntryTwice(t *testing.T) {
	recv := NewLogsReceiver()
	f := NewFanout([]LogsReceiver{recv, recv})
	go func() { _ = f.Send(t.Context(), Entry{}) }()
	<-recv.Chan()
	<-recv.Chan()
}

func TestFanout_SendStopsOnContextCancel(t *testing.T) {
	f := NewFanout([]LogsReceiver{NewLogsReceiver()})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, f.Send(ctx, Entry{}), context.Canceled)
}

// TestFanout_ConcurrentUpdates sends from several goroutines while the
// receivers change at random, and checks that every entry is delivered and no
// receiver gets an entry twice.
func TestFanout_ConcurrentUpdates(t *testing.T) {
	const (
		receivers = 6
		senders   = 4
		perSender = 2000
	)

	type delivery struct{ recv, id int }
	var (
		mut       sync.Mutex
		delivered = map[delivery]int{}
		got       = map[int]bool{}
	)

	pool := make([]LogsReceiver, receivers)
	var readers sync.WaitGroup
	done := make(chan struct{})
	for i := range pool {
		pool[i] = NewLogsReceiver()
		readers.Go(func() {
			for {
				select {
				case e := <-pool[i].Chan():
					id := int(e.Timestamp.UnixNano())
					mut.Lock()
					delivered[delivery{i, id}]++
					got[id] = true
					mut.Unlock()
				case <-done:
					return
				}
			}
		})
	}

	randomChildren := func(r *rand.Rand) []LogsReceiver {
		var children []LogsReceiver
		for _, recv := range pool {
			if r.IntN(2) == 0 {
				children = append(children, recv)
			}
		}
		if len(children) == 0 {
			children = append(children, pool[r.IntN(len(pool))])
		}
		return children
	}

	r := rand.New(rand.NewPCG(1, 2))
	f := NewFanout(randomChildren(r))

	var sendersWG sync.WaitGroup
	for s := range senders {
		sendersWG.Go(func() {
			for i := range perSender {
				id := s*perSender + i
				require.NoError(t, f.Send(t.Context(), Entry{Entry: push.Entry{Timestamp: time.Unix(0, int64(id))}}))
			}
		})
	}

	stopUpdates := make(chan struct{})
	var updater sync.WaitGroup
	updater.Go(func() {
		r := rand.New(rand.NewPCG(3, 4))
		for {
			select {
			case <-stopUpdates:
				return
			default:
				f.UpdateChildren(randomChildren(r))
			}
		}
	})

	sendersWG.Wait()
	close(stopUpdates)
	updater.Wait()
	close(done)
	readers.Wait()

	require.Len(t, got, senders*perSender, "every entry must be delivered")
	for d, n := range delivered {
		require.Equal(t, 1, n, "receiver %d got entry %d %d times", d.recv, d.id, n)
	}
}
