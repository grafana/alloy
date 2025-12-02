package tailv2

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/unicode"
)

func TestTailTailer(t *testing.T) {
	verify := func(t *testing.T, tailer *Tailer, expectedLine *Line, expectedErr error) {
		t.Helper()
		line, err := tailer.Next()
		require.ErrorIs(t, err, expectedErr)
		if expectedLine == nil {
			require.Nil(t, line)
		} else {
			require.Equal(t, expectedLine.Text, line.Text)
			require.Equal(t, expectedLine.Offset, line.Offset)
		}
	}

	t.Run("file must exist", func(t *testing.T) {
		_, err := NewTailer(log.NewNopLogger(), &Config{
			Filename: "/no/such/file",
		})
		require.ErrorIs(t, err, os.ErrNotExist)

		name := createFile(t, "exists", "")
		defer removeFile(t, name)

		_, err = NewTailer(log.NewNopLogger(), &Config{
			Filename: name,
		})
		require.NoError(t, err)
	})

	t.Run("over 4096 byte line", func(t *testing.T) {
		testString := strings.Repeat("a", 4098)

		name := createFile(t, "over4096", "test\n"+testString+"\nhello\nworld\n")
		defer removeFile(t, name)

		tailer, err := NewTailer(log.NewNopLogger(), &Config{
			Filename: name,
		})
		require.NoError(t, err)

		verify(t, tailer, &Line{Text: "test", Offset: 5}, nil)
		verify(t, tailer, &Line{Text: testString, Offset: 4104}, nil)
		verify(t, tailer, &Line{Text: "hello", Offset: 4110}, nil)
		verify(t, tailer, &Line{Text: "world", Offset: 4116}, nil)
		verify(t, tailer, nil, io.EOF)
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
			tailer, err := NewTailer(log.NewNopLogger(), &Config{
				Filename: name,
				Offset:   0,
			})
			require.NoError(t, err)

			verify(t, tailer, &Line{Text: "hello", Offset: first}, nil)
			verify(t, tailer, &Line{Text: "world", Offset: middle}, nil)
			verify(t, tailer, &Line{Text: "test", Offset: end}, nil)
			verify(t, tailer, nil, io.EOF)
		})

		t.Run("skip first", func(t *testing.T) {
			tailer, err := NewTailer(log.NewNopLogger(), &Config{
				Filename: name,
				Offset:   first,
			})
			require.NoError(t, err)

			verify(t, tailer, &Line{Text: "world", Offset: middle}, nil)
			verify(t, tailer, &Line{Text: "test", Offset: end}, nil)
			verify(t, tailer, nil, io.EOF)
		})

		t.Run("end", func(t *testing.T) {
			tailer, err := NewTailer(log.NewNopLogger(), &Config{
				Filename: name,
				Offset:   end,
			})
			require.NoError(t, err)
			verify(t, tailer, nil, io.EOF)
		})
	})

	t.Run("partail line", func(t *testing.T) {
		name := createFile(t, "partial", "hello\nwo")
		defer removeFile(t, name)

		tailer, err := NewTailer(log.NewNopLogger(), &Config{
			Offset:   0,
			Filename: name,
		})
		require.NoError(t, err)

		verify(t, tailer, &Line{Text: "hello", Offset: 6}, nil)
		verify(t, tailer, nil, io.EOF)

		go appendToFile(t, name, "rld\n")

		require.NoError(t, tailer.Wait())
		verify(t, tailer, &Line{Text: "world", Offset: 12}, nil)

		verify(t, tailer, nil, io.EOF)
	})

	t.Run("wait", func(t *testing.T) {
		name := createFile(t, "wait", "hello\nwo")
		defer removeFile(t, name)

		tailer, err := NewTailer(log.NewNopLogger(), &Config{
			Offset:   0,
			Filename: name,
		})
		require.NoError(t, err)

		verify(t, tailer, &Line{Text: "hello", Offset: 6}, nil)
		verify(t, tailer, nil, io.EOF)

		go func() {
			<-time.After(200 * time.Millisecond)
			appendToFile(t, name, "rld\n")
		}()
		require.NoError(t, tailer.Wait())
		verify(t, tailer, &Line{Text: "world", Offset: 12}, nil)
		verify(t, tailer, nil, io.EOF)
	})

	t.Run("truncate", func(t *testing.T) {
		name := createFile(t, "truncate", "a really long string goes here\nhello\nworld\n")
		defer removeFile(t, name)

		tailer, err := NewTailer(log.NewNopLogger(), &Config{
			Filename: name,
			WatcherConfig: WatcherConfig{
				MinPollFrequency: 5 * time.Millisecond,
				MaxPollFrequency: 5 * time.Millisecond,
			},
		})
		require.NoError(t, err)

		verify(t, tailer, &Line{Text: "a really long string goes here", Offset: 31}, nil)
		verify(t, tailer, &Line{Text: "hello", Offset: 37}, nil)
		verify(t, tailer, &Line{Text: "world", Offset: 43}, nil)
		verify(t, tailer, nil, io.EOF)

		go func() {
			// truncate now
			<-time.After(100 * time.Millisecond)
			truncateFile(t, name, "h311o\nw0r1d\nendofworld\n")
		}()

		tailer.Wait()
		verify(t, tailer, &Line{Text: "h311o", Offset: 6}, nil)
		verify(t, tailer, &Line{Text: "w0r1d", Offset: 12}, nil)
		verify(t, tailer, &Line{Text: "endofworld", Offset: 23}, nil)
		verify(t, tailer, nil, io.EOF)
	})

	t.Run("stopped during wait", func(t *testing.T) {
		name := createFile(t, "stopped", "hello\n")
		defer removeFile(t, name)

		tailer, err := NewTailer(log.NewNopLogger(), &Config{
			Offset:   0,
			Filename: name,
		})
		require.NoError(t, err)

		verify(t, tailer, &Line{Text: "hello", Offset: 6}, nil)
		verify(t, tailer, nil, io.EOF)

		go func() {
			time.Sleep(100 * time.Millisecond)
			require.NoError(t, tailer.Stop())
		}()

		require.ErrorIs(t, tailer.Wait(), context.Canceled)
	})

	t.Run("removed and created during wait", func(t *testing.T) {
		name := createFile(t, "removed", "hello\n")
		defer removeFile(t, name)

		tailer, err := NewTailer(log.NewNopLogger(), &Config{
			Offset:   0,
			Filename: name,
			WatcherConfig: WatcherConfig{
				MinPollFrequency: 5 * time.Millisecond,
				MaxPollFrequency: 5 * time.Millisecond,
			},
		})
		require.NoError(t, err)

		verify(t, tailer, &Line{Text: "hello", Offset: 6}, nil)
		verify(t, tailer, nil, io.EOF)

		go func() {
			time.Sleep(100 * time.Millisecond)
			removeFile(t, name)
			time.Sleep(100 * time.Millisecond)
			recreateFile(t, name, "new\n")
		}()

		require.NoError(t, tailer.Wait())

		verify(t, tailer, &Line{Text: "new", Offset: 4}, nil)
		verify(t, tailer, nil, io.EOF)
	})

	t.Run("stopped while waiting for file to be created", func(t *testing.T) {
		name := createFile(t, "removed", "hello\n")

		tailer, err := NewTailer(log.NewNopLogger(), &Config{
			Offset:   0,
			Filename: name,
			WatcherConfig: WatcherConfig{
				MinPollFrequency: 5 * time.Millisecond,
				MaxPollFrequency: 5 * time.Millisecond,
			},
		})
		require.NoError(t, err)

		verify(t, tailer, &Line{Text: "hello", Offset: 6}, nil)
		verify(t, tailer, nil, io.EOF)

		removeFile(t, name)

		go func() {
			time.Sleep(100 * time.Millisecond)
			tailer.Stop()
		}()

		require.ErrorIs(t, tailer.Wait(), context.Canceled)
	})

	t.Run("UTF-16LE", func(t *testing.T) {
		tailer, err := NewTailer(log.NewNopLogger(), &Config{
			Filename: "testdata/mssql.log",
			Decoder:  unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder(),
		})
		require.NoError(t, err)

		verify(t, tailer, &Line{Text: "2025-03-11 11:11:02.58 Server      Microsoft SQL Server 2019 (RTM) - 15.0.2000.5 (X64) ", Offset: 528}, nil)
		verify(t, tailer, &Line{Text: "	Sep 24 2019 13:48:23 ", Offset: 552}, nil)
		verify(t, tailer, &Line{Text: "	Copyright (C) 2019 Microsoft Corporation", Offset: 595}, nil)
		verify(t, tailer, &Line{Text: "	Enterprise Edition (64-bit) on Windows Server 2022 Standard 10.0 <X64> (Build 20348: ) (Hypervisor)", Offset: 697}, nil)
		verify(t, tailer, &Line{Text: "", Offset: 699}, nil)
		verify(t, tailer, &Line{Text: "2025-03-11 11:11:02.71 Server      UTC adjustment: 1:00", Offset: 756}, nil)
		verify(t, tailer, &Line{Text: "2025-03-11 11:11:02.71 Server      (c) Microsoft Corporation.", Offset: 819}, nil)
		verify(t, tailer, &Line{Text: "2025-03-11 11:11:02.72 Server      All rights reserved.", Offset: 876}, nil)
		verify(t, tailer, nil, io.EOF)
	})

}

func createFile(t *testing.T, name, content string) string {
	path := t.TempDir() + "/" + name
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func recreateFile(t *testing.T, path, content string) {
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
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

/*
func renameFile(t *testing.T, oldname, newname string) {
	oldname = t.TempDir() + "/" + oldname
	newname = t.TempDir() + "/" + newname
	require.NoError(t, os.Rename(oldname, newname))
}
*/

func removeFile(t *testing.T, name string) {
	require.NoError(t, os.Remove(name))
}
