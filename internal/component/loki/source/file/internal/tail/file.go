package tail

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/grafana/dskit/backoff"

	"github.com/grafana/alloy/internal/component/loki/source/file/internal/tail/fileext"
)

// NewFile creates a new File for the specified file path.
// It opens the file and seeks to the provided offset if one is specified.
// The returned File can be used to read lines from the file as they are appended.
// The caller is responsible for calling Close() when done to clean up resources.
func NewFile(logger *slog.Logger, cfg *Config) (*File, error) {
	f, err := fileext.OpenFile(cfg.Filename)
	if err != nil {
		return nil, err
	}

	encoding, err := getEncoding(cfg.Encoding)
	if err != nil {
		f.Close()
		return nil, err
	}

	if cfg.WatcherConfig == (WatcherConfig{}) {
		cfg.WatcherConfig = defaultWatcherConfig
	}

	sig, err := newSignatureFromFile(f)
	if err != nil {
		f.Close()
		return nil, err
	}

	reader, err := newReader(logger, f, cfg.Offset, encoding, cfg.Compression, cfg.StartFromEnd)
	if err != nil {
		f.Close()
		return nil, err
	}

	cfg.WatcherConfig.MinPollFrequency = min(cfg.WatcherConfig.MinPollFrequency, cfg.WatcherConfig.MaxPollFrequency)
	return &File{
		cfg:       cfg,
		logger:    logger,
		file:      f,
		reader:    reader,
		signature: sig,
	}, nil
}

// File represents a file being tailed. It provides methods to read lines
// from the file as they are appended, handling file events such as truncation,
// deletion, and modification.
//
// File is not safe for concurrent use: Next, Flush, Wait, and Close must all be
// called from a single goroutine. Cancellation is driven through the context
// passed to Wait rather than by closing the file from another goroutine.
type File struct {
	cfg    *Config
	logger *slog.Logger

	file      *os.File
	reader    *reader
	signature *signature

	// bufferedLines stores lines that were read from an old file handle before
	// it was closed during file rotation.
	bufferedLines []Line
}

// Next returns the next available line from the file.
// When no complete line is available it returns io.EOF. On io.EOF the caller
// should call Wait to block until more data is available and then call Next
// again, or call Flush to read any remaining partial line. Any other error
// indicates an unrecoverable read or decode failure.
func (f *File) Next() (*Line, error) {
	// If we have buffered lines from a previous file rotation, return them first.
	// These are lines that were read from the old file handle before it was closed,
	// ensuring we don't lose any data during file rotation.
	if len(f.bufferedLines) > 0 {
		line := f.bufferedLines[0]
		f.bufferedLines = f.bufferedLines[1:]
		return &line, nil
	}

	text, err := f.reader.next()
	if err != nil {
		return nil, err
	}

	return f.makeLine(text)
}

// Flush returns any remaining data at the end of the file not
// terminated by a newline. It returns io.EOF if there is nothing left.
func (f *File) Flush() (*Line, error) {
	text, err := f.reader.flush()
	if err != nil {
		return nil, err
	}
	return f.makeLine(text)
}

func (f *File) makeLine(text string) (*Line, error) {
	offset := f.reader.position()

	// Recompute signature if we've crossed a threshold and haven't reached it yet.
	// This progressively builds a more complete signature as the file grows.
	if f.signature.shouldRecompute(offset) {
		sig, err := newSignatureFromFile(f.file)
		if err != nil {
			return nil, err
		}

		f.signature = sig
	}

	return &Line{
		Text:   text,
		Offset: offset,
		Time:   time.Now(),
	}, nil
}

// Size returns the current size of the file in bytes.
func (f *File) Size() (int64, error) {
	fi, err := f.file.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// Close closes the underlying file and releases its resources.
func (f *File) Close() error {
	return f.file.Close()
}

// Wait blocks until a file event is detected (modification, truncation, or deletion).
// It returns once the caller should resume calling Next, or an error if ctx is canceled
// or the wait fails.
func (f *File) Wait(ctx context.Context) error {
	offset, err := f.offset()
	if err != nil {
		return err
	}

	event, err := blockUntilEvent(ctx, f.file, offset, f.cfg)
	switch event {
	case eventModified:
		f.logger.Debug("file modified")
		return nil
	case eventTruncated:
		f.logger.Debug("file truncated")
		return f.handleTruncation()
	case eventDeleted:
		f.logger.Debug("file deleted")
		// If a file is deleted we want to make sure we drain what's remaining in the open file.
		f.drain()
		// If we have any buffered lines after drain we can return here to make sure they are consumed and
		// we are not blocking on reopening the new file.
		if len(f.bufferedLines) > 0 {
			f.logger.Debug("finish reading deleted file before reopen")
			return nil
		}
		// In polling mode we could miss events when a file is deleted, so before we give up
		// we try to reopen the file.
		return f.reopen(ctx)
	default:
		return err
	}
}

// handleTruncation resets tailing to the start of the file after an in-place
// truncate.
func (f *File) handleTruncation() error {
	sig, err := newSignatureFromFile(f.file)
	if err != nil {
		return err
	}

	f.signature = sig
	return f.reader.reset(f.file, 0)
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
	for {
		text, err := f.reader.next()
		if err != nil {
			if text != "" {
				f.bufferedLines = append(f.bufferedLines, Line{
					Text:   text,
					Offset: f.reader.position(),
					Time:   time.Now(),
				})
			}

			// flush any remaining data in buffer
			text, _ = f.reader.flush()
			if text != "" {
				f.bufferedLines = append(f.bufferedLines, Line{
					Text:   text,
					Offset: f.reader.position(),
					Time:   time.Now(),
				})
			}

			return
		}
		f.bufferedLines = append(f.bufferedLines, Line{
			Text:   text,
			Offset: f.reader.position(),
			Time:   time.Now(),
		})
	}
}

// reopen closes the current file handle and opens a new one at the same path after
// a delete, rename, or pending-delete event. It waits until the path exists again.
func (f *File) reopen(ctx context.Context) error {
	f.file.Close()

	backoff := backoff.New(ctx, backoff.Config{
		MinBackoff: f.cfg.WatcherConfig.MinPollFrequency,
		MaxBackoff: f.cfg.WatcherConfig.MaxPollFrequency,
		MaxRetries: 20,
	})

	for backoff.Ongoing() {
		file, err := fileext.OpenFile(f.cfg.Filename)
		if err != nil {
			if os.IsNotExist(err) {
				f.logger.Debug("waiting for file to appear", "filename", f.cfg.Filename)
				if err := blockUntilExists(ctx, f.cfg); err != nil {
					return fmt.Errorf("failed to detect creation of %s: %w", f.cfg.Filename, err)
				}
				continue
			}
			return fmt.Errorf("unable to open file %s: %s", f.cfg.Filename, err)
		}

		// File exists and is opened, get information about it.
		nf, err := file.Stat()
		if err != nil {
			f.logger.Debug("failed to stat new file to be tailed, will try to open it again")
			file.Close()
			backoff.Wait()
			continue
		}

		sig, err := newSignatureFromFile(file)
		if err != nil {
			file.Close()
			return err
		}

		// A file recreated at this path starts from the beginning, unless its signature
		// matches the previous file's. Atomic rename-over (for example mv tempfile) is
		// reported as a delete, so a matching signature is treated as the same file and
		// resumes at the previous offset.
		offset := int64(0)
		if f.signature.equal(sig) {
			offset = min(f.reader.position(), nf.Size())
		}

		f.file = file
		f.signature = sig
		if err := f.reader.reset(f.file, offset); err != nil {
			file.Close()
			return err
		}

		break
	}

	return backoff.Err()
}
