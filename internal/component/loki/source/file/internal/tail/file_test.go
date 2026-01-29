package tail

import (
	"compress/gzip"
	"compress/zlib"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

func TestFile(t *testing.T) {
	t.Run("file must exist", func(t *testing.T) {
		_, err := NewFile(log.NewNopLogger(), &Config{
			Filename: "/no/such/file",
		})
		require.ErrorIs(t, err, os.ErrNotExist)

		name := createFile(t, "exists", "")
		defer removeFile(t, name)

		_, err = NewFile(log.NewNopLogger(), &Config{
			Filename: name,
		})
		require.NoError(t, err)
	})

	t.Run("over 4096 byte line", func(t *testing.T) {
		testString := strings.Repeat("a", 4098)

		name := createFile(t, "over4096", "test\n"+testString+"\nhello\nworld\n")
		defer removeFile(t, name)

		file, err := NewFile(log.NewNopLogger(), &Config{
			Filename: name,
		})
		require.NoError(t, err)
		defer file.Stop()

		verifyResult(t, file, &Line{Text: "test", Offset: 5}, nil)
		verifyResult(t, file, &Line{Text: testString, Offset: 4104}, nil)
		verifyResult(t, file, &Line{Text: "hello", Offset: 4110}, nil)
		verifyResult(t, file, &Line{Text: "world", Offset: 4116}, nil)
	})

	t.Run("read", func(t *testing.T) {
		name := createFile(t, "read", "hello\nworld\ntest\n")
		defer removeFile(t, name)

		const (
			first  = 6
			middle = 12
			end    = 17
		)

		t.Run("start", func(t *testing.T) {
			file, err := NewFile(log.NewNopLogger(), &Config{
				Filename: name,
				Offset:   0,
			})
			require.NoError(t, err)
			defer file.Stop()

			verifyResult(t, file, &Line{Text: "hello", Offset: first}, nil)
			verifyResult(t, file, &Line{Text: "world", Offset: middle}, nil)
			verifyResult(t, file, &Line{Text: "test", Offset: end}, nil)
		})

		t.Run("skip first", func(t *testing.T) {
			file, err := NewFile(log.NewNopLogger(), &Config{
				Filename: name,
				Offset:   first,
			})
			require.NoError(t, err)
			defer file.Stop()

			verifyResult(t, file, &Line{Text: "world", Offset: middle}, nil)
			verifyResult(t, file, &Line{Text: "test", Offset: end}, nil)
		})

		t.Run("last", func(t *testing.T) {
			file, err := NewFile(log.NewNopLogger(), &Config{
				Filename: name,
				Offset:   middle,
			})
			require.NoError(t, err)
			defer file.Stop()

			verifyResult(t, file, &Line{Text: "test", Offset: end}, nil)
		})
	})

	t.Run("partial line", func(t *testing.T) {
		name := createFile(t, "partial", "hello\nwo")
		defer removeFile(t, name)

		file, err := NewFile(log.NewNopLogger(), &Config{
			Offset:   0,
			Filename: name,
		})
		require.NoError(t, err)
		defer file.Stop()

		verifyResult(t, file, &Line{Text: "hello", Offset: 6}, nil)
		go func() {
			time.Sleep(50 * time.Millisecond)
			appendToFile(t, name, "rld\n")
		}()
		verifyResult(t, file, &Line{Text: "world", Offset: 12}, nil)
	})

	t.Run("truncate", func(t *testing.T) {
		name := createFile(t, "truncate", "a really long string goes here\nhello\nworld\n")
		defer removeFile(t, name)

		file, err := NewFile(log.NewNopLogger(), &Config{
			Filename: name,
			WatcherConfig: WatcherConfig{
				MinPollFrequency: 50 * time.Millisecond,
				MaxPollFrequency: 50 * time.Millisecond,
			},
		})
		require.NoError(t, err)
		defer file.Stop()

		verifyResult(t, file, &Line{Text: "a really long string goes here", Offset: 31}, nil)
		verifyResult(t, file, &Line{Text: "hello", Offset: 37}, nil)
		verifyResult(t, file, &Line{Text: "world", Offset: 43}, nil)

		go func() {
			// truncate now
			<-time.After(100 * time.Millisecond)
			truncateFile(t, name, "h311o\nw0r1d\nendofworld\n")
		}()

		verifyResult(t, file, &Line{Text: "h311o", Offset: 6}, nil)
		verifyResult(t, file, &Line{Text: "w0r1d", Offset: 12}, nil)
		verifyResult(t, file, &Line{Text: "endofworld", Offset: 23}, nil)

	})

	t.Run("stopped during wait", func(t *testing.T) {
		name := createFile(t, "stopped", "hello\n")
		defer removeFile(t, name)

		file, err := NewFile(log.NewNopLogger(), &Config{
			Offset:   0,
			Filename: name,
		})
		require.NoError(t, err)

		verifyResult(t, file, &Line{Text: "hello", Offset: 6}, nil)

		go func() {
			time.Sleep(100 * time.Millisecond)
			require.NoError(t, file.Stop())
		}()

		_, err = file.Next()
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("stopped while waiting for file to be created", func(t *testing.T) {
		name := createFile(t, "removed", "hello\n")

		file, err := NewFile(log.NewNopLogger(), &Config{
			Offset:   0,
			Filename: name,
			WatcherConfig: WatcherConfig{
				MinPollFrequency: 50 * time.Millisecond,
				MaxPollFrequency: 50 * time.Millisecond,
			},
		})
		require.NoError(t, err)

		verifyResult(t, file, &Line{Text: "hello", Offset: 6}, nil)
		removeFile(t, name)

		go func() {
			time.Sleep(100 * time.Millisecond)
			file.Stop()
		}()
		_, err = file.Next()
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("calls to next after stop", func(t *testing.T) {
		name := createFile(t, "stopped", "hello\n")
		defer removeFile(t, name)

		file, err := NewFile(log.NewNopLogger(), &Config{
			Offset:   0,
			Filename: name,
		})
		require.NoError(t, err)
		file.Stop()

		verifyResult(t, file, nil, context.Canceled)
	})

	t.Run("file rotation drains remaining lines from old file", func(t *testing.T) {
		name := createFile(t, "rotation", "line1\nline2\nline3\nline4\npartial")
		defer removeFile(t, name)

		file, err := NewFile(log.NewNopLogger(), &Config{
			Filename: name,
			WatcherConfig: WatcherConfig{
				MinPollFrequency: 50 * time.Millisecond,
				MaxPollFrequency: 50 * time.Millisecond,
			},
		})
		require.NoError(t, err)
		defer file.Stop()

		// Read first two lines
		verifyResult(t, file, &Line{Text: "line1", Offset: 6}, nil)
		verifyResult(t, file, &Line{Text: "line2", Offset: 12}, nil)

		// Rotate the file
		rotateFile(t, name, "newline1\nnewline2\n")

		// After rotation is detected, drain() should read all remaining
		// lines from the old file (line3, line4, partial) before reading from the new file.
		// Verify we get the remaining old lines first, then new lines
		verifyResult(t, file, &Line{Text: "line3", Offset: 18}, nil)
		verifyResult(t, file, &Line{Text: "line4", Offset: 24}, nil)
		verifyResult(t, file, &Line{Text: "partial", Offset: 31}, nil)
		verifyResult(t, file, &Line{Text: "newline1", Offset: 9}, nil)
		verifyResult(t, file, &Line{Text: "newline2", Offset: 18}, nil)
	})

	t.Run("deleted while reading", func(t *testing.T) {
		name := createFile(t, "removed", "1\n2\n")
		file, err := NewFile(log.NewNopLogger(), &Config{
			Offset:   0,
			Filename: name,
			WatcherConfig: WatcherConfig{
				MinPollFrequency: 50 * time.Millisecond,
				MaxPollFrequency: 50 * time.Millisecond,
			},
		})
		require.NoError(t, err)

		go func() {
			time.Sleep(50 * time.Millisecond)
			appendToFile(t, name, "3\n4\n")
			removeFile(t, name)
		}()

		verifyResult(t, file, &Line{Text: "1", Offset: 2}, nil)
		verifyResult(t, file, &Line{Text: "2", Offset: 4}, nil)
		verifyResult(t, file, &Line{Text: "3", Offset: 6}, nil)
		verifyResult(t, file, &Line{Text: "4", Offset: 8}, nil)
	})

	t.Run("should handle atomic writes", func(t *testing.T) {
		name := createFile(t, "atomicwrite", "line1\nline2\nline3\nline4\n")
		defer removeFile(t, name)

		file, err := NewFile(log.NewNopLogger(), &Config{
			Filename: name,
			WatcherConfig: WatcherConfig{
				MinPollFrequency: 50 * time.Millisecond,
				MaxPollFrequency: 50 * time.Millisecond,
			},
		})
		require.NoError(t, err)
		defer file.Stop()

		// Read first two lines
		verifyResult(t, file, &Line{Text: "line1", Offset: 6}, nil)
		verifyResult(t, file, &Line{Text: "line2", Offset: 12}, nil)
		atomicWrite(t, name, "line1\nline2\nline3\nline4\nnewline1\n")
		verifyResult(t, file, &Line{Text: "line3", Offset: 18}, nil)
		verifyResult(t, file, &Line{Text: "line4", Offset: 24}, nil)
		verifyResult(t, file, &Line{Text: "newline1", Offset: 33}, nil)
	})

	t.Run("should handle atomic writes with new content", func(t *testing.T) {
		name := createFile(t, "atomicwrite", "line1\nline2\nline3\nline4\n")
		defer removeFile(t, name)

		file, err := NewFile(log.NewNopLogger(), &Config{
			Filename: name,
			WatcherConfig: WatcherConfig{
				MinPollFrequency: 50 * time.Millisecond,
				MaxPollFrequency: 50 * time.Millisecond,
			},
		})
		require.NoError(t, err)
		defer file.Stop()

		// Read first two lines
		verifyResult(t, file, &Line{Text: "line1", Offset: 6}, nil)
		verifyResult(t, file, &Line{Text: "line2", Offset: 12}, nil)
		atomicWrite(t, name, "newline1\n")
		// Because we buffer lines when file is deleted we still get line3 and line4.
		verifyResult(t, file, &Line{Text: "line3", Offset: 18}, nil)
		verifyResult(t, file, &Line{Text: "line4", Offset: 24}, nil)
		verifyResult(t, file, &Line{Text: "newline1", Offset: 9}, nil)
	})

	t.Run("UTF-16LE", func(t *testing.T) {
		file, err := NewFile(log.NewNopLogger(), &Config{
			Filename: "testdata/mssql.log",
			Encoding: "UTF-16LE",
		})
		require.NoError(t, err)
		defer file.Stop()

		verifyResult(t, file, &Line{Text: "2025-03-11 11:11:02.58 Server      Microsoft SQL Server 2019 (RTM) - 15.0.2000.5 (X64) ", Offset: 180}, nil)
		verifyResult(t, file, &Line{Text: "	Sep 24 2019 13:48:23 ", Offset: 228}, nil)
		verifyResult(t, file, &Line{Text: "	Copyright (C) 2019 Microsoft Corporation", Offset: 314}, nil)
		verifyResult(t, file, &Line{Text: "	Enterprise Edition (64-bit) on Windows Server 2022 Standard 10.0 <X64> (Build 20348: ) (Hypervisor)", Offset: 518}, nil)
		verifyResult(t, file, &Line{Text: "", Offset: 522}, nil)
		verifyResult(t, file, &Line{Text: "2025-03-11 11:11:02.71 Server      UTC adjustment: 1:00", Offset: 636}, nil)
		verifyResult(t, file, &Line{Text: "2025-03-11 11:11:02.71 Server      (c) Microsoft Corporation.", Offset: 762}, nil)
		verifyResult(t, file, &Line{Text: "2025-03-11 11:11:02.72 Server      All rights reserved.", Offset: 876}, nil)
	})

	t.Run("should detect UTF-16LE encoding from BOM", func(t *testing.T) {
		enc := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder()
		encoded, err := enc.String("Hello, 世界\r\n")
		require.NoError(t, err)
		name := createFile(t, "utf-16LE", encoded)
		defer removeFile(t, name)

		file, err := NewFile(log.NewNopLogger(), &Config{
			Filename: name,
			// We are setting UTF8 here but still expect to decode the file using UTF-16LE
			Encoding: "UTF-8",
		})
		require.NoError(t, err)

		verifyResult(t, file, &Line{Text: "Hello, 世界", Offset: 24}, nil)
		file.Stop()

		enc = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
		encoded, err = enc.String("newline\r\n")
		require.NoError(t, err)
		appendToFile(t, name, encoded)

		// Reopen file from last offset to make sure it handles that.
		file, err = NewFile(log.NewNopLogger(), &Config{
			Filename: name,
			// We are setting UTF-8 here but still expect to decode the file using UTF-16LE
			Encoding: "UTF-8",
			Offset:   24,
		})
		require.NoError(t, err)
		defer file.Stop()

		verifyResult(t, file, &Line{Text: "newline", Offset: 42}, nil)
	})

	t.Run("should detect UTF-16BE encoding from BOM", func(t *testing.T) {
		enc := unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewEncoder()
		encoded, err := enc.String("Hello, 世界\r\n")
		require.NoError(t, err)
		name := createFile(t, "utf-16LE", encoded)
		defer removeFile(t, name)

		file, err := NewFile(log.NewNopLogger(), &Config{
			Filename: name,
			// We are setting UTF-8 here but still expect to decode the file using UTF-16LE
			Encoding: "UTF-8",
		})
		require.NoError(t, err)

		verifyResult(t, file, &Line{Text: "Hello, 世界", Offset: 24}, nil)
		file.Stop()

		enc = unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewEncoder()
		encoded, err = enc.String("newline\r\n")
		require.NoError(t, err)
		appendToFile(t, name, encoded)

		// Reopen file from last offset.
		file, err = NewFile(log.NewNopLogger(), &Config{
			Filename: name,
			// We are setting UTF-8 here but still expect to decode the file using UTF-16LE
			Encoding: "UTF-8",
			Offset:   24,
		})
		require.NoError(t, err)
		defer file.Stop()

		verifyResult(t, file, &Line{Text: "newline", Offset: 42}, nil)
	})

	t.Run("should detect UTF-8 encoding from BOM", func(t *testing.T) {
		bytes := []byte("Hello, 世界\r\n")

		name := createFile(t, "utf-8", string(append(bomUTF8Bytes, bytes...)))
		defer removeFile(t, name)

		file, err := NewFile(log.NewNopLogger(), &Config{
			Filename: name,
			Encoding: "UTF-16",
		})
		require.NoError(t, err)
		defer file.Stop()

		verifyResult(t, file, &Line{Text: "Hello, 世界", Offset: 18}, nil)
	})

	var (
		utf8offsets  = [3]int64{6, 12, 18}
		utf16offsets = [3]int64{14, 26, 38}
	)

	t.Run("read gzip", func(t *testing.T) {
		compressionTest(t, "plain", "gz", encoding.Nop.NewEncoder(), utf8offsets)
		compressionTest(t, "utf-16be", "gz", unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewEncoder(), utf16offsets)
		compressionTest(t, "utf-16le", "gz", unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder(), utf16offsets)
	})

	t.Run("read zlib", func(t *testing.T) {
		compressionTest(t, "plain", "z", encoding.Nop.NewEncoder(), utf8offsets)
		compressionTest(t, "utf-16be", "z", unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewEncoder(), utf16offsets)
		compressionTest(t, "utf-16le", "z", unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder(), utf16offsets)
	})
}

func compressionTest(t *testing.T, name, compression string, enc *encoding.Encoder, offsets [3]int64) {
	t.Run(name, func(t *testing.T) {
		content, err := enc.String("line1\nline2\nline3\n")
		require.NoError(t, err)

		fileName := createCompressedFile(t, name, compression, strings.NewReader(content))
		defer removeFile(t, fileName)

		file, err := NewFile(log.NewNopLogger(), &Config{
			Filename: fileName,
			WatcherConfig: WatcherConfig{
				MinPollFrequency: 50 * time.Millisecond,
				MaxPollFrequency: 50 * time.Millisecond,
			},
			Compression: compression,
		})
		require.NoError(t, err)

		verifyResult(t, file, &Line{Text: "line1", Offset: offsets[0]}, nil)
		verifyResult(t, file, &Line{Text: "line2", Offset: offsets[1]}, nil)
		verifyResult(t, file, &Line{Text: "line3", Offset: offsets[2]}, nil)
		verifyResult(t, file, nil, io.EOF)
		require.NoError(t, file.Stop())

		file, err = NewFile(log.NewNopLogger(), &Config{
			Filename: fileName,
			WatcherConfig: WatcherConfig{
				MinPollFrequency: 50 * time.Millisecond,
				MaxPollFrequency: 50 * time.Millisecond,
			},
			Compression: compression,
			Offset:      offsets[0],
		})
		require.NoError(t, err)
		defer file.Stop()

		verifyResult(t, file, &Line{Text: "line2", Offset: offsets[1]}, nil)
		verifyResult(t, file, &Line{Text: "line3", Offset: offsets[2]}, nil)
		verifyResult(t, file, nil, io.EOF)
	})
}

func createFile(t *testing.T, name, content string) string {
	path := t.TempDir() + "/" + name
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func appendToFile(t *testing.T, name, content string) {
	f, err := os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0600)
	require.NoError(t, err)
	defer f.Close()
	_, err = f.WriteString(content)
	require.NoError(t, err)
}

func truncateFile(t *testing.T, name, content string) {
	f, err := os.OpenFile(name, os.O_TRUNC|os.O_WRONLY, 0600)
	require.NoError(t, err)
	defer f.Close()
	_, err = f.WriteString(content)
	require.NoError(t, err)
}

func removeFile(t *testing.T, name string) {
	require.NoError(t, os.Remove(name))
}

func rotateFile(t *testing.T, name, newContent string) {
	removeFile(t, name)
	// Create new file with same name
	require.NoError(t, os.WriteFile(name, []byte(newContent), 0600))
}

func createCompressedFile(t *testing.T, name, compression string, reader io.Reader) string {
	path := t.TempDir() + "/" + name
	f, err := os.Create(path)
	require.NoError(t, err)

	var (
		writer io.WriteCloser
	)

	switch compression {
	case "gz":
		writer = gzip.NewWriter(f)
	case "z":
		writer = zlib.NewWriter(f)
	case "bz2":
		// go std lib to not provide writer for bzip2.
		t.Fatalf("bz2 unimplemented")
	default:
		writer = f
	}

	_, err = io.Copy(writer, reader)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	// NOTE: if we did not provide a compression file is used as
	// writer and we already closed it on the line above.
	if err := f.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		require.NoError(t, err)
	}
	return path
}

func verifyResult(t *testing.T, f *File, expectedLine *Line, expectedErr error) {
	t.Helper()
	line, err := f.Next()

	require.ErrorIs(t, err, expectedErr)
	if expectedLine == nil {
		require.Nil(t, line)
	} else {
		require.Equal(t, expectedLine.Text, line.Text)
		require.Equal(t, expectedLine.Offset, line.Offset)
	}
}
