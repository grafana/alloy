package tail

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/backoff"
	"golang.org/x/text/encoding"

	"github.com/grafana/alloy/internal/component/loki/source/file/internal/tail/fileext"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// detectBOM reads the first few bytes of the file to detect a Byte Order Mark (BOM).
// Returns the number of bytes the BOM occupies (0 if no BOM is found).
// Common BOM patterns:
//   - UTF-8: EF BB BF (3 bytes)
//   - UTF-16 LE: FF FE (2 bytes)
//   - UTF-16 BE: FE FF (2 bytes)
//   - UTF-32 LE: FF FE 00 00 (4 bytes)
//   - UTF-32 BE: 00 00 FE FF (4 bytes)
func detectBOM(f *os.File) (int64, error) {
	// Save current position
	currentPos, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	defer f.Seek(currentPos, io.SeekStart) // Restore position

	// Seek to start
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	// Read first 4 bytes to cover all BOM types
	bomBytes := make([]byte, 4)
	n, err := f.Read(bomBytes)
	if err != nil && err != io.EOF {
		return 0, err
	}

	if n < 2 {
		return 0, nil // Not enough bytes for any BOM
	}

	// Check for UTF-16 LE/BE (2 bytes)
	if bomBytes[0] == 0xFF && bomBytes[1] == 0xFE {
		if n >= 4 && bomBytes[2] == 0x00 && bomBytes[3] == 0x00 {
			return 4, nil // UTF-32 LE
		}
		return 2, nil // UTF-16 LE
	}

	// Check for UTF-16 BE (2 bytes)
	if bomBytes[0] == 0xFE && bomBytes[1] == 0xFF {
		return 2, nil // UTF-16 BE
	}

	// Check for UTF-32 BE (4 bytes)
	if n >= 4 && bomBytes[0] == 0x00 && bomBytes[1] == 0x00 && bomBytes[2] == 0xFE && bomBytes[3] == 0xFF {
		return 4, nil // UTF-32 BE
	}

	// Check for UTF-8 BOM (3 bytes)
	if n >= 3 && bomBytes[0] == 0xEF && bomBytes[1] == 0xBB && bomBytes[2] == 0xBF {
		return 3, nil // UTF-8
	}

	return 0, nil // No BOM found
}

// NewFile creates a new File tailer for the specified file path.
// It opens the file and seeks to the provided offset if one is specified.
// The returned File can be used to read lines from the file as they are appended.
// The caller is responsible for calling Stop() when done to close the file and clean up resources.
func NewFile(logger log.Logger, cfg *Config) (*File, error) {
	f, err := fileext.OpenFile(cfg.Filename)
	if err != nil {
		return nil, err
	}

	if cfg.Encoding == nil {
		cfg.Encoding = encoding.Nop
	}

	if cfg.WatcherConfig == (WatcherConfig{}) {
		cfg.WatcherConfig = defaultWatcherConfig
	}

	// Detect and skip BOM if starting from the beginning of the file
	actualOffset := cfg.Offset
	if cfg.Offset == 0 {
		bomSize, err := detectBOM(f)
		if err != nil {
			return nil, fmt.Errorf("failed to detect BOM: %w", err)
		}
		if bomSize > 0 {
			actualOffset = bomSize
			if _, err := f.Seek(bomSize, io.SeekStart); err != nil {
				return nil, err
			}
		}
	} else {
		// Seek to provided offset
		if _, err := f.Seek(cfg.Offset, io.SeekStart); err != nil {
			return nil, err
		}
	}

	scanner, err := newScanner(f, actualOffset, cfg.Encoding)
	if err != nil {
		return nil, err
	}

	cfg.WatcherConfig.MinPollFrequency = min(cfg.WatcherConfig.MinPollFrequency, cfg.WatcherConfig.MaxPollFrequency)
	ctx, cancel := context.WithCancel(context.Background())

	return &File{
		cfg:     cfg,
		logger:  logger,
		file:    f,
		scanner: scanner,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

// File represents a file being tailed. It provides methods to read lines
// from the file as they are appended, handling file events such as truncation,
// deletion, and modification. File is safe for concurrent use.
type File struct {
	cfg    *Config
	logger log.Logger

	// protects file, reader, and lastOffset.
	mu      sync.Mutex
	file    *os.File
	scanner *scanner

	lastOffset int64

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

	text, err := f.scanner.next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			if err := f.wait(); err != nil {
				return nil, err
			}
			goto read
		}
		return nil, err
	}

	offset, err := f.scanner.position()
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
func (f *File) wait() error {
	offset, err := f.offset()
	if err != nil {
		return err
	}

	event, err := blockUntilEvent(f.ctx, f.file, offset, f.cfg)
	switch event {
	case eventModified:
		level.Debug(f.logger).Log("msg", "file modified")
		f.file.Seek(f.lastOffset, io.SeekStart)
		f.scanner.reset(f.file, f.lastOffset)
		return nil
	case eventTruncated:
		level.Debug(f.logger).Log("msg", "file truncated")
		// We need to reopen the file when it was truncated.
		f.lastOffset = 0
		return f.reopen(true)
	case eventDeleted:
		level.Debug(f.logger).Log("msg", "file deleted")
		// if a file is deleted we want to make sure we drain what's remaining in the open file.
		f.drain()

		f.lastOffset = 0
		// In polling mode we could miss events when a file is deleted, so before we give up
		// we try to reopen the file.
		return f.reopen(false)
	default:
		return err
	}
}

// offset returns the current byte offset in the file where the next read will occur.
func (f *File) offset() (int64, error) {
	offset, err := f.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	return offset, nil
}

// drain reads all remaining complete lines from the current file handle and stores
// them in bufferedLines. This is called when a file deletion/rotation is detected
// to ensure we don't lose any data from the old file before switching to the new one.
// drain is best effort and will stop if it encounters any errors.
func (f *File) drain() {
	if _, err := f.file.Seek(f.lastOffset, io.SeekStart); err != nil {
		return
	}
	f.scanner.reset(f.file, f.lastOffset)

	for {
		text, err := f.scanner.next()
		if err != nil {
			if text != "" {
				offset, err := f.scanner.position()
				if err != nil {
					return
				}
				f.bufferedLines = append(f.bufferedLines, Line{
					Text:   text,
					Offset: offset,
					Time:   time.Now(),
				})
			}
			return
		}

		offset, err := f.scanner.position()
		if err != nil {
			return
		}

		f.bufferedLines = append(f.bufferedLines, Line{
			Text:   text,
			Offset: offset,
			Time:   time.Now(),
		})
	}
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

		f.file = file
		f.scanner.reset(f.file, f.lastOffset)
		break
	}

	return backoff.Err()
}
