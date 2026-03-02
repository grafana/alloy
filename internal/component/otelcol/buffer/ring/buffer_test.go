package ring

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuffer_InsertEvictsOldestWhenMaxBytesExceeded(t *testing.T) {
	buffer, err := NewBuffer(10, time.Hour)
	require.NoError(t, err)

	require.NoError(t, buffer.Insert([]byte("1234"), SignalTypeTraces))
	require.NoError(t, buffer.Insert([]byte("abc"), SignalTypeLogs))
	require.NoError(t, buffer.Insert([]byte("WXYZ"), SignalTypeMetrics))

	entries := buffer.Snapshot()
	require.Len(t, entries, 2)
	require.Equal(t, []byte("abc"), entries[0].Payload)
	require.Equal(t, SignalTypeLogs, entries[0].SignalType)
	require.Equal(t, []byte("WXYZ"), entries[1].Payload)
	require.Equal(t, SignalTypeMetrics, entries[1].SignalType)

	util := buffer.Utilization()
	require.Equal(t, 7, util.TotalBytes)
	require.Equal(t, 2, util.EntryCount)
}

func TestBuffer_EvictsEntriesOlderThanMaxAge(t *testing.T) {
	clock := newManualClock(time.Unix(1000, 0))

	buffer, err := newBuffer(32, 10*time.Second, clock.Now)
	require.NoError(t, err)

	require.NoError(t, buffer.Insert([]byte("old"), SignalTypeTraces))
	clock.Advance(11 * time.Second)
	require.NoError(t, buffer.Insert([]byte("new"), SignalTypeLogs))

	entries := buffer.Snapshot()
	require.Len(t, entries, 1)
	require.Equal(t, []byte("new"), entries[0].Payload)
	require.Equal(t, SignalTypeLogs, entries[0].SignalType)
}

func TestBuffer_UtilizationMatchesSnapshotAfterEviction(t *testing.T) {
	buffer, err := NewBuffer(8, time.Hour)
	require.NoError(t, err)

	require.NoError(t, buffer.Insert([]byte("AAAA"), SignalTypeTraces))
	require.NoError(t, buffer.Insert([]byte("BBB"), SignalTypeLogs))
	require.NoError(t, buffer.Insert([]byte("CC"), SignalTypeMetrics))

	entries := buffer.Snapshot()
	util := buffer.Utilization()

	sum := 0
	for _, entry := range entries {
		sum += entry.Size
	}

	require.Equal(t, sum, util.TotalBytes)
	require.Equal(t, len(entries), util.EntryCount)
	require.Equal(t, 5, util.TotalBytes)
	require.Equal(t, 2, util.EntryCount)
}

func TestBuffer_RejectsOversizedEntry(t *testing.T) {
	buffer, err := NewBuffer(4, time.Hour)
	require.NoError(t, err)

	err = buffer.Insert([]byte("12345"), SignalTypeTraces)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrEntryTooLarge))

	util := buffer.Utilization()
	require.Equal(t, 0, util.TotalBytes)
	require.Equal(t, 0, util.EntryCount)
}

func TestBuffer_ConcurrentInsertAndEvict_NoRaceAndAccurateAccounting(t *testing.T) {
	clock := newManualClock(time.Unix(2000, 0))

	buffer, err := newBuffer(128, 2*time.Second, clock.Now)
	require.NoError(t, err)

	const (
		inserters      = 6
		insertsPerGo   = 250
		evictionPasses = 900
	)

	var wg sync.WaitGroup
	errCh := make(chan error, inserters)

	for i := 0; i < inserters; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < insertsPerGo; j++ {
				size := 1 + ((worker + j) % 11)
				payload := make([]byte, size)
				for k := range payload {
					payload[k] = byte('a' + (worker+j+k)%26)
				}
				if insertErr := buffer.Insert(payload, SignalTypeTraces); insertErr != nil {
					errCh <- insertErr
					return
				}
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < evictionPasses; i++ {
			clock.Advance(10 * time.Millisecond)
			buffer.EvictExpired()
		}
	}()

	wg.Wait()
	close(errCh)
	for insertErr := range errCh {
		require.NoError(t, insertErr)
	}

	buffer.EvictExpired()

	entries := buffer.Snapshot()
	util := buffer.Utilization()

	total := 0
	for _, entry := range entries {
		total += entry.Size
	}

	require.Equal(t, total, util.TotalBytes)
	require.Equal(t, len(entries), util.EntryCount)
	require.LessOrEqual(t, util.TotalBytes, 128)
}

type manualClock struct {
	mut sync.Mutex
	now time.Time
}

func newManualClock(start time.Time) *manualClock {
	return &manualClock{now: start}
}

func (c *manualClock) Now() time.Time {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.now
}

func (c *manualClock) Advance(d time.Duration) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.now = c.now.Add(d)
}

func TestNewBuffer_ValidateArguments(t *testing.T) {
	_, err := NewBuffer(0, time.Second)
	require.EqualError(t, err, "max_bytes must be greater than zero")

	_, err = NewBuffer(1, 0)
	require.EqualError(t, err, "max_age must be greater than zero")

	_, err = newBuffer(1, time.Second, nil)
	require.EqualError(t, err, "now function must not be nil")
}

func TestBuffer_SnapshotReturnsCopy(t *testing.T) {
	buffer, err := NewBuffer(16, time.Hour)
	require.NoError(t, err)
	require.NoError(t, buffer.Insert([]byte("payload"), SignalTypeTraces))

	entries := buffer.Snapshot()
	require.Len(t, entries, 1)

	entries[0].Payload[0] = 'X'
	next := buffer.Snapshot()
	require.Equal(t, "payload", string(next[0].Payload))
}
