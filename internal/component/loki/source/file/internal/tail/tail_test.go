// Copyright (c) 2015 HPE Software Inc. All rights reserved.
// Copyright (c) 2013 ActiveState Software Inc. All rights reserved.

// TODO:
//  * repeat all the tests with Poll:true

package tail

import (
	_ "fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/loki/source/file/internal/tail/watch"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/encoding/unicode"
)

var testPollingOptions = watch.PollingFileWatcherOptions{
	// Use a smaller poll duration for faster test runs. Keep it below
	// 100ms (which value is used as common delays for tests)
	MinPollFrequency: 5 * time.Millisecond,
	MaxPollFrequency: 5 * time.Millisecond,
}

func TestTail(t *testing.T) {
	verify := func(t *testing.T, tail *Tail, lines []string) {
		got := make([]string, 0, len(lines))

		var wg sync.WaitGroup
		wg.Go(func() {
			for {
				line := <-tail.Lines
				got = append(got, line.Text)
				if len(got) == len(lines) {
					return
				}
			}
		})
		wg.Wait()
		assert.Equal(t, lines, got)
	}

	t.Run("file must exist", func(t *testing.T) {
		_, err := TailFile("/no/such/file", Config{})
		assert.Error(t, err)
	})

	t.Run("should be able to stop", func(t *testing.T) {
		tail, err := TailFile("README.md", Config{})
		assert.NoError(t, err)

		go consume(tail)
		assert.NoError(t, tail.Stop())
	})

	t.Run("over 4096 byte line", func(t *testing.T) {
		tailTest := NewTailTest("Over4096ByteLine", t)
		testString := strings.Repeat("a", 4097)
		tailTest.CreateFile("test.txt", "test\n"+testString+"\nhello\nworld\n")
		defer tailTest.RemoveFile("test.txt")
		tail := tailTest.StartTail("test.txt", Config{})
		defer tail.Stop()

		verify(t, tail, []string{"test", testString, "hello", "world"})
	})

	t.Run("read full", func(t *testing.T) {
		tailTest := NewTailTest("location-full", t)
		tailTest.CreateFile("test.txt", "hello\nworld\n")
		defer tailTest.RemoveFile("test.txt")
		tail := tailTest.StartTail("test.txt", Config{Location: nil})
		defer tail.Stop()

		verify(t, tail, []string{"hello", "world"})
	})

	t.Run("read end", func(t *testing.T) {
		tailTest := NewTailTest("location-end", t)
		tailTest.CreateFile("test.txt", "hello\nworld\n")
		defer tailTest.RemoveFile("test.txt")
		tail := tailTest.StartTail("test.txt", Config{Location: &SeekInfo{0, io.SeekEnd}})
		defer tail.Stop()

		go func() {
			<-time.After(100 * time.Millisecond)
			tailTest.AppendFile("test.txt", "more\ndata\n")

			<-time.After(100 * time.Millisecond)
			tailTest.AppendFile("test.txt", "more\ndata\n")
		}()

		verify(t, tail, []string{"more", "data", "more", "data"})
	})

	t.Run("read middle", func(t *testing.T) {
		tailTest := NewTailTest("location-middle", t)
		tailTest.CreateFile("test.txt", "hello\nworld\n")
		defer tailTest.RemoveFile("test.txt")
		tail := tailTest.StartTail("test.txt", Config{Location: &SeekInfo{-6, io.SeekEnd}})
		defer tail.Stop()

		go func() {
			<-time.After(100 * time.Millisecond)
			tailTest.AppendFile("test.txt", "more\ndata\n")

			<-time.After(100 * time.Millisecond)
			tailTest.AppendFile("test.txt", "more\ndata\n")
		}()

		verify(t, tail, []string{"world", "more", "data", "more", "data"})
	})

	t.Run("reseek", func(t *testing.T) {
		tailTest := NewTailTest("reseek-polling", t)
		tailTest.CreateFile("test.txt", "a really long string goes here\nhello\nworld\n")
		defer tailTest.RemoveFile("test.txt")
		tail := tailTest.StartTail("test.txt", Config{PollOptions: testPollingOptions})
		defer tail.Stop()

		go func() {
			// truncate now
			<-time.After(100 * time.Millisecond)
			tailTest.TruncateFile("test.txt", "h311o\nw0r1d\nendofworld\n")
		}()

		verify(t, tail, []string{"a really long string goes here", "hello", "world", "h311o", "w0r1d", "endofworld"})
	})

	t.Run("tell", func(t *testing.T) {
		tailTest := NewTailTest("tell-position", t)
		tailTest.CreateFile("test.txt", "hello\nworld\nagain\nmore\n")
		defer tailTest.RemoveFile("test.txt")

		tail := tailTest.StartTail("test.txt", Config{Location: &SeekInfo{0, io.SeekStart}})

		// read one line
		<-tail.Lines
		offset, err := tail.Tell()
		assert.NoError(t, err)

		// consume rest so we can close
		go consume(tail)
		tail.Stop()

		tail = tailTest.StartTail("test.txt", Config{Location: &SeekInfo{offset, io.SeekStart}})
		l := <-tail.Lines
		assert.Equal(t, "again", l.Text)

		// consume rest so we can stop
		go consume(tail)
		tail.Stop()
	})

	t.Run("UTF-16LE", func(t *testing.T) {
		tail, err := TailFile("testdata/mssql.log", Config{Decoder: unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder()})
		assert.NoError(t, err)
		defer tail.Stop()

		expectedLines := []string{
			"2025-03-11 11:11:02.58 Server      Microsoft SQL Server 2019 (RTM) - 15.0.2000.5 (X64) ",
			"	Sep 24 2019 13:48:23 ",
			"	Copyright (C) 2019 Microsoft Corporation",
			"	Enterprise Edition (64-bit) on Windows Server 2022 Standard 10.0 <X64> (Build 20348: ) (Hypervisor)",
			"",
			"2025-03-11 11:11:02.71 Server      UTC adjustment: 1:00",
			"2025-03-11 11:11:02.71 Server      (c) Microsoft Corporation.",
			"2025-03-11 11:11:02.72 Server      All rights reserved.",
		}

		verify(t, tail, expectedLines)
	})
}

func TestTellRace(t *testing.T) {
	tailTest := NewTailTest("tell-race", t)
	tailTest.CreateFile("test.txt", "hello\nworld\n")

	tail := tailTest.StartTail("test.txt", Config{PollOptions: testPollingOptions})

	<-tail.Lines
	<-tail.Lines

	_, err := tail.Tell()
	if err != nil {
		t.Fatal("unexpected error", err)
	}

	tailTest.TruncateFile("test.txt", "yay\nyay2\n")

	// wait for reopen to happen
	time.Sleep(100 * time.Millisecond)

	_, err = tail.Tell()
	if err != nil {
		t.Fatal("unexpected error", err)
	}

}

func TestSizeRace(t *testing.T) {
	tailTest := NewTailTest("tell-race", t)
	tailTest.CreateFile("test.txt", "hello\nworld\n")
	tail := tailTest.StartTail("test.txt", Config{PollOptions: testPollingOptions})

	<-tail.Lines
	<-tail.Lines

	s1, err := tail.Size()
	if err != nil {
		t.Fatal("unexpected error", err)
	}

	tailTest.TruncateFile("test.txt", "yay\nyay2\n") // smaller than before

	// wait for reopen to happen
	time.Sleep(100 * time.Millisecond)

	s2, err := tail.Size()
	if err != nil {
		t.Fatal("unexpected error", err)
	}

	if s2 == 0 || s2 > s1 {
		t.Fatal("expected 0 < s2 < s1! s1:", s1, "s2:", s2)
	}
}

// Test library
type TailTest struct {
	Name string
	path string
	t    *testing.T
}

func NewTailTest(name string, t *testing.T) TailTest {
	tt := TailTest{name, t.TempDir() + "/" + name, t}
	err := os.MkdirAll(tt.path, os.ModeTemporary|0700)
	if err != nil {
		tt.t.Fatal(err)
	}

	return tt
}

func (t TailTest) CreateFile(name string, contents string) {
	assert.NoError(t.t, os.WriteFile(t.path+"/"+name, []byte(contents), 0600))
}

func (t TailTest) AppendToFile(name string, contents string) {
	assert.NoError(t.t, os.WriteFile(t.path+"/"+name, []byte(contents), 0600|os.ModeAppend))

}

func (t TailTest) RemoveFile(name string) {
	err := os.Remove(t.path + "/" + name)
	assert.NoError(t.t, err)

}

func (t TailTest) RenameFile(oldname string, newname string) {
	oldname = t.path + "/" + oldname
	newname = t.path + "/" + newname
	assert.NoError(t.t, os.Rename(oldname, newname))
}

func (t TailTest) AppendFile(name string, contents string) {
	f, err := os.OpenFile(t.path+"/"+name, os.O_APPEND|os.O_WRONLY, 0600)
	assert.NoError(t.t, err)
	defer f.Close()
	_, err = f.WriteString(contents)
	assert.NoError(t.t, err)
}

func (t TailTest) TruncateFile(name string, contents string) {
	f, err := os.OpenFile(t.path+"/"+name, os.O_TRUNC|os.O_WRONLY, 0600)
	assert.NoError(t.t, err)
	defer f.Close()
	_, err = f.WriteString(contents)
	assert.NoError(t.t, err)
}

func (t TailTest) StartTail(name string, config Config) *Tail {
	tail, err := TailFile(t.path+"/"+name, config)
	assert.NoError(t.t, err)
	return tail
}

// consume consumes lines from tail
func consume(tail *Tail) {
	for range tail.Lines {
	}
}
