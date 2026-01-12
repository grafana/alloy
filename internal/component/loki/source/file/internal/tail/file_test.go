package tail

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/unicode"
)

func TestFile(t *testing.T) {
	verify := func(t *testing.T, f *File, expectedLine *Line, expectedErr error) {
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

		verify(t, file, &Line{Text: "test", Offset: 5}, nil)
		verify(t, file, &Line{Text: testString, Offset: 4104}, nil)
		verify(t, file, &Line{Text: "hello", Offset: 4110}, nil)
		verify(t, file, &Line{Text: "world", Offset: 4116}, nil)
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

			verify(t, file, &Line{Text: "hello", Offset: first}, nil)
			verify(t, file, &Line{Text: "world", Offset: middle}, nil)
			verify(t, file, &Line{Text: "test", Offset: end}, nil)
		})

		t.Run("skip first", func(t *testing.T) {
			file, err := NewFile(log.NewNopLogger(), &Config{
				Filename: name,
				Offset:   first,
			})
			require.NoError(t, err)
			defer file.Stop()

			verify(t, file, &Line{Text: "world", Offset: middle}, nil)
			verify(t, file, &Line{Text: "test", Offset: end}, nil)
		})

		t.Run("last", func(t *testing.T) {
			file, err := NewFile(log.NewNopLogger(), &Config{
				Filename: name,
				Offset:   middle,
			})
			require.NoError(t, err)
			defer file.Stop()

			verify(t, file, &Line{Text: "test", Offset: end}, nil)
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

		verify(t, file, &Line{Text: "hello", Offset: 6}, nil)
		go func() {
			time.Sleep(50 * time.Millisecond)
			appendToFile(t, name, "rld\n")
		}()
		verify(t, file, &Line{Text: "world", Offset: 12}, nil)
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

		verify(t, file, &Line{Text: "a really long string goes here", Offset: 31}, nil)
		verify(t, file, &Line{Text: "hello", Offset: 37}, nil)
		verify(t, file, &Line{Text: "world", Offset: 43}, nil)

		go func() {
			// truncate now
			<-time.After(100 * time.Millisecond)
			truncateFile(t, name, "h311o\nw0r1d\nendofworld\n")
		}()

		verify(t, file, &Line{Text: "h311o", Offset: 6}, nil)
		verify(t, file, &Line{Text: "w0r1d", Offset: 12}, nil)
		verify(t, file, &Line{Text: "endofworld", Offset: 23}, nil)

	})

	t.Run("stopped during wait", func(t *testing.T) {
		name := createFile(t, "stopped", "hello\n")
		defer removeFile(t, name)

		file, err := NewFile(log.NewNopLogger(), &Config{
			Offset:   0,
			Filename: name,
		})
		require.NoError(t, err)

		verify(t, file, &Line{Text: "hello", Offset: 6}, nil)

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

		verify(t, file, &Line{Text: "hello", Offset: 6}, nil)
		removeFile(t, name)

		go func() {
			time.Sleep(100 * time.Millisecond)
			file.Stop()
		}()
		_, err = file.Next()
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("UTF-16LE", func(t *testing.T) {
		file, err := NewFile(log.NewNopLogger(), &Config{
			Filename: "testdata/mssql.log",
			Decoder:  unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder(),
		})
		require.NoError(t, err)
		defer file.Stop()

		verify(t, file, &Line{Text: "2025-03-11 11:11:02.58 Server      Microsoft SQL Server 2019 (RTM) - 15.0.2000.5 (X64) ", Offset: 528}, nil)
		verify(t, file, &Line{Text: "	Sep 24 2019 13:48:23 ", Offset: 552}, nil)
		verify(t, file, &Line{Text: "	Copyright (C) 2019 Microsoft Corporation", Offset: 595}, nil)
		verify(t, file, &Line{Text: "	Enterprise Edition (64-bit) on Windows Server 2022 Standard 10.0 <X64> (Build 20348: ) (Hypervisor)", Offset: 697}, nil)
		verify(t, file, &Line{Text: "", Offset: 699}, nil)
		verify(t, file, &Line{Text: "2025-03-11 11:11:02.71 Server      UTC adjustment: 1:00", Offset: 756}, nil)
		verify(t, file, &Line{Text: "2025-03-11 11:11:02.71 Server      (c) Microsoft Corporation.", Offset: 819}, nil)
		verify(t, file, &Line{Text: "2025-03-11 11:11:02.72 Server      All rights reserved.", Offset: 876}, nil)
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

		verify(t, file, nil, context.Canceled)
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
		verify(t, file, &Line{Text: "line1", Offset: 6}, nil)
		verify(t, file, &Line{Text: "line2", Offset: 12}, nil)

		// Rotate the file
		rotateFile(t, name, "newline1\nnewline2\n")

		// After rotation is detected, drain() should read all remaining
		// lines from the old file (line3, line4, partial) before reading from the new file.
		// Verify we get the remaining old lines first, then new lines
		verify(t, file, &Line{Text: "line3", Offset: 18}, nil)
		verify(t, file, &Line{Text: "line4", Offset: 24}, nil)
		verify(t, file, &Line{Text: "partial", Offset: 31}, nil)
		verify(t, file, &Line{Text: "newline1", Offset: 9}, nil)
		verify(t, file, &Line{Text: "newline2", Offset: 18}, nil)
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
		verify(t, file, &Line{Text: "line1", Offset: 6}, nil)
		verify(t, file, &Line{Text: "line2", Offset: 12}, nil)
		atomicwrite(t, name, "line1\nline2\nline3\nline4\nnewline1\n")
		verify(t, file, &Line{Text: "line3", Offset: 18}, nil)
		verify(t, file, &Line{Text: "line4", Offset: 24}, nil)
		verify(t, file, &Line{Text: "newline1", Offset: 33}, nil)
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
		verify(t, file, &Line{Text: "line1", Offset: 6}, nil)
		verify(t, file, &Line{Text: "line2", Offset: 12}, nil)
		atomicwrite(t, name, "newline1\n")
		// Because we buffer lines when file is deleted we still get line3 and line3.
		verify(t, file, &Line{Text: "line3", Offset: 18}, nil)
		verify(t, file, &Line{Text: "line4", Offset: 24}, nil)
		verify(t, file, &Line{Text: "newline1", Offset: 9}, nil)
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

func atomicwrite(t *testing.T, name, newContent string) {
	dir := filepath.Dir(name)
	filename := filepath.Base(name)

	tmp, err := os.CreateTemp(dir, filename+".tmp")
	require.NoError(t, err)

	_, err = tmp.Write([]byte(newContent))
	require.NoError(t, err)

	require.NoError(t, tmp.Sync())
	require.NoError(t, tmp.Close())
	require.NoError(t, os.Rename(tmp.Name(), name))
}
