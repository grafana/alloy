package tailv2

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/backoff"

	"github.com/grafana/alloy/internal/component/loki/source/file/internal/tailv2/fileext"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func NewTailer(logger log.Logger, cfg *Config) (*Tailer, error) {
	f, err := fileext.OpenFile(cfg.Filename)
	if err != nil {
		return nil, err
	}

	if cfg.Offset != 0 {
		// Seek to provided offset
		if _, err := f.Seek(cfg.Offset, io.SeekStart); err != nil {
			return nil, err
		}
	}

	watcher, err := newWatcher(cfg.Filename, cfg.WatcherConfig)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Tailer{
		cfg:     cfg,
		logger:  logger,
		file:    f,
		reader:  newReader(f, cfg),
		watcher: watcher,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

type Tailer struct {
	cfg    *Config
	logger log.Logger

	mu     sync.Mutex
	file   *os.File
	reader *bufio.Reader

	lastOffset int64

	watcher *watcher

	ctx    context.Context
	cancel context.CancelFunc
}

// FIXME: need clear exit signal
func (t *Tailer) Next() (*Line, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
read:
	text, err := t.readLine()

	if err != nil {
		if errors.Is(err, io.EOF) {
			if err := t.wait(text != ""); err != nil {
				return nil, err
			}
			goto read
		}
		return nil, err
	}

	offset, err := t.offset()
	if err != nil {
		return nil, err
	}

	t.lastOffset = offset

	return &Line{
		Text:   text,
		Offset: offset,
		Time:   time.Now(),
	}, nil
}

func (t *Tailer) Stop() error {
	t.cancel()
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.file.Close()
}

func (t *Tailer) wait(partial bool) error {
	offset, err := t.offset()
	if err != nil {
		return err
	}

	event, err := t.watcher.blockUntilEvent(t.ctx, t.file, offset)
	switch event {
	case eventModified:
		if partial {
			// We need to reset to last succeful offset because we could have consumed a partial line.
			t.file.Seek(t.lastOffset, io.SeekStart)
			t.reader.Reset(t.file)
		}
		return nil
	case eventTruncated:
		// We need to reopen the file when it was truncated.
		return t.reopen(true)
	case eventDeleted:
		// In polling mode we could miss events when a file is deleted, so before we give up
		// we try to reopen the file.
		return t.reopen(false)
	default:
		return err
	}

}

func (t *Tailer) readLine() (string, error) {
	line, err := t.reader.ReadString('\n')
	if err != nil {
		return line, err
	}

	line = strings.TrimRight(line, "\n")
	// Trim Windows line endings
	line = strings.TrimSuffix(line, "\r")
	return line, err
}

func (t *Tailer) offset() (int64, error) {
	offset, err := t.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	return offset - int64(t.reader.Buffered()), nil
}

func (t *Tailer) reopen(truncated bool) error {
	// There are cases where the file is reopened so quickly it's still the same file
	// which causes the poller to hang on an open file handle to a file no longer being written to
	// and which eventually gets deleted.  Save the current file handle info to make sure we only
	// start tailing a different file.
	cf, err := t.file.Stat()
	if !truncated && err != nil {
		// We don't action on this error but are logging it, not expecting to see it happen and not sure if we
		// need to action on it, cf is checked for nil later on to accommodate this
		level.Debug(t.logger).Log("msg", "stat of old file returned, this is not expected and may result in unexpected behavior")
	}

	t.file.Close()

	backoff := backoff.New(t.ctx, backoff.Config{
		MinBackoff: DefaultWatcherConfig.MaxPollFrequency,
		MaxBackoff: DefaultWatcherConfig.MaxPollFrequency,
		MaxRetries: 20,
	})

	for backoff.Ongoing() {
		file, err := fileext.OpenFile(t.cfg.Filename)
		if err != nil {
			if os.IsNotExist(err) {
				level.Debug(t.logger).Log("msg", fmt.Sprintf("waiting for %s to appear...", t.cfg.Filename))
				if err := t.watcher.blockUntilExists(t.ctx); err != nil {
					return fmt.Errorf("failed to detect creation of %s: %w", t.cfg.Filename, err)
				}
				backoff.Wait()
				continue
			}
			return fmt.Errorf("Unable to open file %s: %s", t.cfg.Filename, err)
		}

		// File exists and is opened, get information about it.
		nf, err := file.Stat()
		if err != nil {
			level.Debug(t.logger).Log("msg", "failed to stat new file to be tailed, will try to open it again")
			file.Close()
			backoff.Wait()
			continue
		}

		// Check to see if we are trying to reopen and tail the exact same file (and it was not truncated).
		if !truncated && cf != nil && os.SameFile(cf, nf) {
			file.Close()
			backoff.Wait()
			continue
		}

		t.file = file
		t.reader = newReader(t.file, t.cfg)
		break
	}

	return backoff.Err()
}

func newReader(f *os.File, cfg *Config) *bufio.Reader {
	if cfg.Decoder != nil {
		return bufio.NewReader(cfg.Decoder.Reader(f))
	}
	return bufio.NewReader(f)
}
