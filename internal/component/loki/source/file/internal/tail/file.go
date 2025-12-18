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

// NewFile creates a new File tailer for the specified file path.
// It opens the file and seeks to the provided offset if one is specified.
// The returned File can be used to read lines from the file as they are appended.
// The caller is responsible for calling Stop() when done to close the file and clean up resources.
func NewFile(logger log.Logger, cfg *Config) (*File, error) {
	f, err := fileext.OpenFile(cfg.Filename)
	if err != nil {
		return nil, err
	}

	sig, err := newSignatureFromFile(f)
	if err != nil {
		return nil, err
	}

	// NOTE: we always need to seek because newSignatureFromFile will read from file.
	if _, err := f.Seek(cfg.Offset, io.SeekStart); err != nil {
		return nil, err
	}

	if cfg.WatcherConfig == (WatcherConfig{}) {
		cfg.WatcherConfig = defaultWatcherConfig
	}

	cfg.WatcherConfig.MinPollFrequency = min(cfg.WatcherConfig.MinPollFrequency, cfg.WatcherConfig.MaxPollFrequency)

	ctx, cancel := context.WithCancel(context.Background())

	return &File{
		cfg:        cfg,
		logger:     logger,
		file:       f,
		reader:     newReader(f, cfg),
		ctx:        ctx,
		cancel:     cancel,
		signature:  sig,
		lastOffset: cfg.Offset,
	}, nil
}

// File represents a file being tailed. It provides methods to read lines
// from the file as they are appended, handling file events such as truncation,
// deletion, and modification. File is safe for concurrent use.
type File struct {
	cfg    *Config
	logger log.Logger

	// protects file, reader, and lastOffset.
	mu     sync.Mutex
	file   *os.File
	reader *bufio.Reader

	lastOffset int64
	signature  *signature

	// bufferedLines stores lines that were read from an old file handle before
	// it was closed during file rotation.
	bufferedLines []Line

	ctx    context.Context
	cancel context.CancelFunc
}

// Next reads and returns the next line from the file.
// It blocks until a line is available, file is closed or unrecoverable error occurs.
// If file was closed context.Canceled is returned.
func (f *File) Next() (*Line, error) {
	select {
	case <-f.ctx.Done():
		return nil, f.ctx.Err()
	default:
	}

	f.mu.Lock()
	defer f.mu.Unlock()

read:
	// If we have buffered lines from a previous file rotation, return them first.
	// These are lines that were read from the old file handle before it was closed,
	// ensuring we don't lose any data during file rotation.
	if len(f.bufferedLines) > 0 {
		line := f.bufferedLines[0]
		f.bufferedLines = f.bufferedLines[1:]
		return &line, nil
	}

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

	// We only recompute signature once we have read past the target size and it's not already
	// complete.
	if f.lastOffset >= signatureSize && !f.signature.completed() {
		if _, err := f.file.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}

		sig, err := newSignatureFromFile(f.file)
		if err != nil {
			return nil, err
		}

		f.signature = sig
		if _, err := f.file.Seek(f.lastOffset, io.SeekStart); err != nil {
			return nil, err
		}
		f.reader.Reset(f.file)
	}

	return &Line{
		Text:   text,
		Offset: offset,
		Time:   time.Now(),
	}, nil
}

// Size returns the current size of the file in bytes.
// It is safe to call concurrently with other File methods.
func (f *File) Size() (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	fi, err := f.file.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// Stop closes the file and cancels any ongoing wait operations.
// After Stop is called, Next() will return errors for any subsequent calls.
// It is safe to call Stop multiple times.
func (f *File) Stop() error {
	f.cancel()
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Close()
}

// wait blocks until a file event is detected (modification, truncation, or deletion).
// Returns an error if the context is canceled or an unrecoverable error occurs.
func (f *File) wait(partial bool) error {
	offset, err := f.offset()
	if err != nil {
		return err
	}

	event, err := blockUntilEvent(f.ctx, f.file, offset, f.cfg)
	switch event {
	case eventModified:
		level.Debug(f.logger).Log("msg", "file modified")
		if partial {
			// We need to reset to last successful offset because we consumed a partial line.
			f.file.Seek(f.lastOffset, io.SeekStart)
			f.reader.Reset(f.file)
		}
		return nil
	case eventTruncated:
		level.Debug(f.logger).Log("msg", "file truncated")
		// We need to reopen the file when it was truncated.
		return f.reopen(true)
	case eventDeleted:
		level.Debug(f.logger).Log("msg", "file deleted")
		// if a file is deleted we want to make sure we drain what's remaining in the open file.
		f.drain()
		// In polling mode we could miss events when a file is deleted, so before we give up
		// we try to reopen the file.
		return f.reopen(false)
	default:
		return err
	}
}

// readLine reads a single line from the file, including the newline character.
// The newline and any trailing carriage return (for Windows line endings) are stripped.
func (f *File) readLine() (string, error) {
	line, err := f.reader.ReadString('\n')
	if err != nil {
		return line, err
	}
	return strings.TrimRight(line, "\r\n"), err
}

// drain reads all remaining complete lines from the current file handle and stores
// them in bufferedLines. This is called when a file deletion/rotation is detected
// to ensure we don't lose any data from the old file before switching to the new one.
// drain is best effort and will stop if it encounters any errors.
func (f *File) drain() {
	if _, err := f.file.Seek(f.lastOffset, io.SeekStart); err != nil {
		return
	}
	f.reader.Reset(f.file)

	for {
		text, err := f.readLine()
		if err != nil {
			if text != "" {
				offset, err := f.offset()
				if err != nil {
					return
				}

				f.lastOffset = offset
				f.bufferedLines = append(f.bufferedLines, Line{
					Text:   text,
					Offset: offset,
					Time:   time.Now(),
				})
			}
			return
		}

		offset, err := f.offset()
		if err != nil {
			return
		}

		f.lastOffset = offset
		f.bufferedLines = append(f.bufferedLines, Line{
			Text:   text,
			Offset: offset,
			Time:   time.Now(),
		})
	}
}

// offset returns the current byte offset in the file where the next read will occur.
// It accounts for buffered data in the reader.
func (f *File) offset() (int64, error) {
	offset, err := f.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	return offset - int64(f.reader.Buffered()), nil
}

// reopen closes the current file handle and opens a new one for the same file path.
// If truncated is true, it indicates the file was truncated and we should reopen immediately.
// If truncated is false, it indicates the file was deleted or moved, and we should wait
// for it to be recreated before reopening.
//
// reopen handles the case where a file is reopened so quickly it's still the same file,
// which could cause the poller to hang on an open file handle to a file no longer being
// written to. It saves the current file handle info to ensure we only start tailing a
// different file instance.
func (f *File) reopen(truncated bool) error {
	cf, err := f.file.Stat()
	if !truncated && err != nil {
		// We don't action on this error but are logging it, not expecting to see it happen and not sure if we
		// need to action on it, cf is checked for nil later on to accommodate this
		level.Debug(f.logger).Log("msg", "stat of old file returned, this is not expected and may result in unexpected behavior")
	}

	offset, _ := f.offset()
	f.file.Close()

	backoff := backoff.New(f.ctx, backoff.Config{
		MinBackoff: f.cfg.WatcherConfig.MinPollFrequency,
		MaxBackoff: f.cfg.WatcherConfig.MaxPollFrequency,
		MaxRetries: 20,
	})

	for backoff.Ongoing() {
		file, err := fileext.OpenFile(f.cfg.Filename)
		if err != nil {
			if os.IsNotExist(err) {
				level.Debug(f.logger).Log("msg", fmt.Sprintf("waiting for %s to appear...", f.cfg.Filename))
				if err := blockUntilExists(f.ctx, f.cfg); err != nil {
					return fmt.Errorf("failed to detect creation of %s: %w", f.cfg.Filename, err)
				}
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

		// Compute a new signature and compare it with the previous one to detect atomic writes.
		// When a file is replaced atomically, the file handle changes but the
		// initial content may be the same. If signatures match, it's the same file content,
		// so we continue from the previous offset. If they differ, it's a different
		// file, so we start from the beginning.
		sig, err := newSignatureFromFile(file)
		if err != nil {
			return err
		}

		if f.signature.equal(sig) {
			f.lastOffset = offset
		} else {
			f.lastOffset = 0
		}

		f.signature = sig
		f.file = file
		f.file.Seek(f.lastOffset, io.SeekStart)
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
