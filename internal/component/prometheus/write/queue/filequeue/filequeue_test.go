package filequeue

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/vladopajic/go-actor/actor"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/prometheus/write/queue/types"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

func TestFileQueue(t *testing.T) {
	defer goleak.VerifyNone(t)
	dir := t.TempDir()
	log := log.NewNopLogger()
	mbx := actor.NewMailbox[types.DataHandle]()
	mbx.Start()
	defer mbx.Stop()
	q, err := NewQueue(dir, func(ctx context.Context, dh types.DataHandle) {
		_ = mbx.Send(ctx, dh)
	}, log)
	require.NoError(t, err)
	q.Start()
	defer q.Stop()
	err = q.Store(context.Background(), nil, []byte("test"))

	require.NoError(t, err)

	meta, buf, err := getHandle(t, mbx)
	require.NoError(t, err)
	require.True(t, string(buf) == "test")
	require.Len(t, meta, 0)

	// Ensure nothing new comes through.
	timer := time.NewTicker(100 * time.Millisecond)
	select {
	case <-timer.C:
		return
	case <-mbx.ReceiveC():
		require.True(t, false)
	}
}

func TestMetaFileQueue(t *testing.T) {
	defer goleak.VerifyNone(t)

	dir := t.TempDir()
	log := log.NewNopLogger()
	mbx := actor.NewMailbox[types.DataHandle]()
	mbx.Start()
	defer mbx.Stop()
	q, err := NewQueue(dir, func(ctx context.Context, dh types.DataHandle) {
		_ = mbx.Send(ctx, dh)
	}, log)
	q.Start()
	defer q.Stop()
	require.NoError(t, err)
	err = q.Store(context.Background(), map[string]string{"name": "bob"}, []byte("test"))
	require.NoError(t, err)

	meta, buf, err := getHandle(t, mbx)
	require.NoError(t, err)
	require.True(t, string(buf) == "test")
	require.Len(t, meta, 1)
	require.True(t, meta["name"] == "bob")
}

func TestCorruption(t *testing.T) {
	defer goleak.VerifyNone(t)

	dir := t.TempDir()
	log := log.NewNopLogger()
	mbx := actor.NewMailbox[types.DataHandle]()
	mbx.Start()
	defer mbx.Stop()
	q, err := NewQueue(dir, func(ctx context.Context, dh types.DataHandle) {
		_ = mbx.Send(ctx, dh)
	}, log)
	q.Start()
	defer q.Stop()
	require.NoError(t, err)

	err = q.Store(context.Background(), map[string]string{"name": "bob"}, []byte("first"))
	require.NoError(t, err)
	err = q.Store(context.Background(), map[string]string{"name": "bob"}, []byte("second"))

	require.NoError(t, err)

	// Send is async so may need to wait a bit for it happen.
	require.Eventually(t, func() bool {
		// First should be 1.committed
		_, errStat := os.Stat(filepath.Join(dir, "1.committed"))
		return errStat == nil
	}, 2*time.Second, 100*time.Millisecond)

	fi, err := os.Stat(filepath.Join(dir, "1.committed"))

	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, fi.Name()), []byte("bad"), 0644)
	require.NoError(t, err)

	_, _, err = getHandle(t, mbx)
	require.Error(t, err)

	meta, buf, err := getHandle(t, mbx)
	require.NoError(t, err)
	require.True(t, string(buf) == "second")
	require.Len(t, meta, 1)
}

func TestFileDeleted(t *testing.T) {
	defer goleak.VerifyNone(t)

	dir := t.TempDir()
	log := log.NewNopLogger()
	mbx := actor.NewMailbox[types.DataHandle]()
	mbx.Start()
	defer mbx.Stop()
	q, err := NewQueue(dir, func(ctx context.Context, dh types.DataHandle) {
		_ = mbx.Send(ctx, dh)
	}, log)
	q.Start()
	defer q.Stop()
	require.NoError(t, err)

	evenHandles := make([]string, 0)
	for i := 0; i < 10; i++ {
		err = q.Store(context.Background(), map[string]string{"name": "bob"}, []byte(strconv.Itoa(i)))

		require.NoError(t, err)
		if i%2 == 0 {
			evenHandles = append(evenHandles, filepath.Join(dir, strconv.Itoa(i+1)+".committed"))
		}
	}

	// Send is async so may need to wait a bit for it happen, check for the last file written.
	require.Eventually(t, func() bool {
		_, errStat := os.Stat(filepath.Join(dir, "10.committed"))
		return errStat == nil
	}, 2*time.Second, 100*time.Millisecond)

	for _, h := range evenHandles {
		_ = os.Remove(h)
	}
	// Every even file was deleted and should have an error.
	for i := 0; i < 10; i++ {
		_, buf2, err := getHandle(t, mbx)
		if i%2 == 0 {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.True(t, string(buf2) == strconv.Itoa(i))
		}
	}
}

func TestOtherFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		// TODO: Fix this test as we mature the file queue
		t.Skip("This test is very flaky on Windows. Will need to fix it as we mature the filequeue.")
	}
	defer goleak.VerifyNone(t)

	dir := t.TempDir()
	log := log.NewNopLogger()
	mbx := actor.NewMailbox[types.DataHandle]()
	mbx.Start()
	defer mbx.Stop()
	q, err := NewQueue(dir, func(ctx context.Context, dh types.DataHandle) {
		_ = mbx.Send(ctx, dh)
	}, log)
	q.Start()
	defer q.Stop()
	require.NoError(t, err)

	err = q.Store(context.Background(), nil, []byte("first"))
	require.NoError(t, err)
	os.Create(filepath.Join(dir, "otherfile"))
	_, buf, err := getHandle(t, mbx)
	require.NoError(t, err)
	require.True(t, string(buf) == "first")
}

func TestResuming(t *testing.T) {
	defer goleak.VerifyNone(t)

	dir := t.TempDir()
	log := log.NewNopLogger()
	mbx := actor.NewMailbox[types.DataHandle]()
	mbx.Start()
	q, err := NewQueue(dir, func(ctx context.Context, dh types.DataHandle) {
		_ = mbx.Send(ctx, dh)
	}, log)
	q.Start()
	require.NoError(t, err)

	err = q.Store(context.Background(), nil, []byte("first"))

	require.NoError(t, err)

	err = q.Store(context.Background(), nil, []byte("second"))

	require.NoError(t, err)
	time.Sleep(1 * time.Second)
	mbx.Stop()
	q.Stop()

	mbx2 := actor.NewMailbox[types.DataHandle]()
	mbx2.Start()
	defer mbx2.Stop()
	q2, err := NewQueue(dir, func(ctx context.Context, dh types.DataHandle) {
		_ = mbx2.Send(ctx, dh)
	}, log)
	require.NoError(t, err)
	q2.Start()
	defer q2.Stop()
	err = q2.Store(context.Background(), nil, []byte("third"))

	require.NoError(t, err)
	_, buf, err := getHandle(t, mbx2)
	require.NoError(t, err)
	require.True(t, string(buf) == "first")

	_, buf, err = getHandle(t, mbx2)
	require.NoError(t, err)
	require.True(t, string(buf) == "second")

	_, buf, err = getHandle(t, mbx2)
	require.NoError(t, err)
	require.True(t, string(buf) == "third")
}

func getHandle(t *testing.T, mbx actor.MailboxReceiver[types.DataHandle]) (map[string]string, []byte, error) {
	timer := time.NewTicker(5 * time.Second)
	select {
	case <-timer.C:
		require.True(t, false)
		// This is only here to satisfy the linting.
		return nil, nil, nil
	case item, ok := <-mbx.ReceiveC():
		require.True(t, ok)
		return item.Pop()
	}
}
