package tail

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

	"github.com/grafana/alloy/internal/component/loki/source/file/internal/tail/fileext"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func NewFile(logger log.Logger, cfg *Config) (*File, error) {
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

	return &File{
		cfg:     cfg,
		logger:  logger,
		file:    f,
		reader:  newReader(f, cfg),
		watcher: watcher,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

type File struct {
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
func (f *File) Next() (*Line, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
read:
	text, err := f.readLine()

	if err != nil {
		if errors.Is(err, io.EOF) {
			if err := f.wait(text != ""); err != nil {
				return nil, err
			}
			goto read
		}
		return nil, err
	}

	offset, err := f.offset()
	if err != nil {
		return nil, err
	}

	f.lastOffset = offset

	return &Line{
		Text:   text,
		Offset: offset,
		Time:   time.Now(),
	}, nil
}

func (f *File) Size() (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	fi, err := f.file.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func (f *File) Stop() error {
	f.cancel()
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Close()
}

func (f *File) wait(partial bool) error {
	offset, err := f.offset()
	if err != nil {
		return err
	}

	event, err := f.watcher.blockUntilEvent(f.ctx, f.file, offset)
	switch event {
	case eventModified:
		if partial {
			// We need to reset to last succeful offset because we could have consumed a partial line.
			f.file.Seek(f.lastOffset, io.SeekStart)
			f.reader.Reset(f.file)
		}
		return nil
	case eventTruncated:
		// We need to reopen the file when it was truncated.
		return f.reopen(true)
	case eventDeleted:
		// In polling mode we could miss events when a file is deleted, so before we give up
		// we try to reopen the file.
		return f.reopen(false)
	default:
		return err
	}

}

func (f *File) readLine() (string, error) {
	line, err := f.reader.ReadString('\n')
	if err != nil {
		return line, err
	}

	line = strings.TrimRight(line, "\n")
	// Trim Windows line endings
	line = strings.TrimSuffix(line, "\r")
	return line, err
}

func (f *File) offset() (int64, error) {
	offset, err := f.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	return offset - int64(f.reader.Buffered()), nil
}

func (f *File) reopen(truncated bool) error {
	// There are cases where the file is reopened so quickly it's still the same file
	// which causes the poller to hang on an open file handle to a file no longer being written to
	// and which eventually gets deleted.  Save the current file handle info to make sure we only
	// start tailing a different file.
	cf, err := f.file.Stat()
	if !truncated && err != nil {
		// We don't action on this error but are logging it, not expecting to see it happen and not sure if we
		// need to action on it, cf is checked for nil later on to accommodate this
		level.Debug(f.logger).Log("msg", "stat of old file returned, this is not expected and may result in unexpected behavior")
	}

	f.file.Close()

	backoff := backoff.New(f.ctx, backoff.Config{
		MinBackoff: DefaultWatcherConfig.MaxPollFrequency,
		MaxBackoff: DefaultWatcherConfig.MaxPollFrequency,
		MaxRetries: 20,
	})

	for backoff.Ongoing() {
		file, err := fileext.OpenFile(f.cfg.Filename)
		if err != nil {
			if os.IsNotExist(err) {
				level.Debug(f.logger).Log("msg", fmt.Sprintf("waiting for %s to appear...", f.cfg.Filename))
				if err := f.watcher.blockUntilExists(f.ctx); err != nil {
					return fmt.Errorf("failed to detect creation of %s: %w", f.cfg.Filename, err)
				}
				backoff.Wait()
				continue
			}
			return fmt.Errorf("Unable to open file %s: %s", f.cfg.Filename, err)
		}

		// File exists and is opened, get information about it.
		nf, err := file.Stat()
		if err != nil {
			level.Debug(f.logger).Log("msg", "failed to stat new file to be tailed, will try to open it again")
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

		f.file = file
		f.reader.Reset(f.file)
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
