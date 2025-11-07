// Copyright (c) 2015 HPE Software Inc. All rights reserved.
// Copyright (c) 2013 ActiveState Software Inc. All rights reserved.

// TODO:
//  * repeat all the tests with Poll:true

package tail

import (
	"fmt"
	_ "fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/loki/source/file/internal/tail/watch"
	"github.com/stretchr/testify/assert"
)

var testPollingOptions = watch.PollingFileWatcherOptions{
	// Use a smaller poll duration for faster test runs. Keep it below
	// 100ms (which value is used as common delays for tests)
	MinPollFrequency: 5 * time.Millisecond,
	MaxPollFrequency: 5 * time.Millisecond,
}

func TestMustExist(t *testing.T) {
	fmt.Println("Start")
	_, err := TailFile("/no/such/file", Config{})
	assert.Error(t, err)
	fmt.Println("first")

	tail, err := TailFile("README.md", Config{})
	fmt.Println("tail second")
	assert.NoError(t, err)
	fmt.Println("NoError")
	assert.NoError(t, tail.Stop())
	fmt.Println("Stopped")
}

func TestWaitsForFileToExist(t *testing.T) {
	tailTest := NewTailTest("waits-for-file-to-exist", t)
	tail := tailTest.StartTail("test.txt", Config{})

	go tailTest.VerifyTailOutput(tail, []string{"hello", "world"}, false)

	<-time.After(100 * time.Millisecond)
	tailTest.CreateFile("test.txt", "hello\nworld\n")
}

func TestWaitsForFileToExistRelativePath(t *testing.T) {
	tailTest := NewTailTest("waits-for-file-to-exist-relative", t)

	oldWD, err := os.Getwd()
	if err != nil {
		tailTest.Fatal(err)
	}
	os.Chdir(tailTest.path)
	defer os.Chdir(oldWD)

	tail := tailTest.StartTail("test.txt", Config{})
	if err != nil {
		tailTest.Fatal(err)
	}

	go tailTest.VerifyTailOutput(tail, []string{"hello", "world"}, false)

	<-time.After(100 * time.Millisecond)
	if err := os.WriteFile("test.txt", []byte("hello\nworld\n"), 0600); err != nil {
		tailTest.Fatal(err)
	}
}

func TestStop(t *testing.T) {
	tail, err := TailFile("_no_such_file", Config{})
	if err != nil {
		t.Error("MustExist:false is violated")
	}
	if tail.Stop() != nil {
		t.Error("Should be stoped successfully")
	}
}

func TestStopAtEOF(t *testing.T) {
	tailTest := NewTailTest("maxlinesize", t)
	tailTest.CreateFile("test.txt", "hello\nthere\nworld\n")
	tail := tailTest.StartTail("test.txt", Config{Location: nil})

	// read "hello"
	line := <-tail.Lines
	if line.Text != "hello" {
		t.Errorf("Expected to get 'hello', got '%s' instead", line.Text)
	}

	tailTest.VerifyTailOutput(tail, []string{"there", "world"}, false)
	tail.StopAtEOF()
}

func TestOver4096ByteLine(t *testing.T) {
	tailTest := NewTailTest("Over4096ByteLine", t)
	testString := strings.Repeat("a", 4097)
	tailTest.CreateFile("test.txt", "test\n"+testString+"\nhello\nworld\n")
	tail := tailTest.StartTail("test.txt", Config{Location: nil})
	go tailTest.VerifyTailOutput(tail, []string{"test", testString, "hello", "world"}, false)

	// Delete after a reasonable delay, to give tail sufficient time
	// to read all lines.
	<-time.After(100 * time.Millisecond)
	tailTest.RemoveFile("test.txt")
}

func TestLocationFull(t *testing.T) {
	tailTest := NewTailTest("location-full", t)
	tailTest.CreateFile("test.txt", "hello\nworld\n")
	tail := tailTest.StartTail("test.txt", Config{Location: nil})
	go tailTest.VerifyTailOutput(tail, []string{"hello", "world"}, false)

	// Delete after a reasonable delay, to give tail sufficient time
	// to read all lines.
	<-time.After(100 * time.Millisecond)
	tailTest.RemoveFile("test.txt")
}

func TestLocationEnd(t *testing.T) {
	tailTest := NewTailTest("location-end", t)
	tailTest.CreateFile("test.txt", "hello\nworld\n")
	tail := tailTest.StartTail("test.txt", Config{Location: &SeekInfo{0, io.SeekEnd}})
	go tailTest.VerifyTailOutput(tail, []string{"more", "data"}, false)

	<-time.After(100 * time.Millisecond)
	tailTest.AppendFile("test.txt", "more\ndata\n")

	// Delete after a reasonable delay, to give tail sufficient time
	// to read all lines.
	<-time.After(100 * time.Millisecond)
	tailTest.RemoveFile("test.txt")
}

func TestLocationMiddle(t *testing.T) {
	// Test reading from middle.
	tailTest := NewTailTest("location-middle", t)
	tailTest.CreateFile("test.txt", "hello\nworld\n")
	tail := tailTest.StartTail("test.txt", Config{Location: &SeekInfo{-6, io.SeekEnd}})
	go tailTest.VerifyTailOutput(tail, []string{"world", "more", "data"}, false)

	<-time.After(100 * time.Millisecond)
	tailTest.AppendFile("test.txt", "more\ndata\n")

	// Delete after a reasonable delay, to give tail sufficient time
	// to read all lines.
	<-time.After(100 * time.Millisecond)
	tailTest.RemoveFile("test.txt")
}

func TestReOpenPolling(t *testing.T) {
	reOpen(t)
}

func TestReSeekPolling(t *testing.T) {
	reSeek(t)
}

func TestTell(t *testing.T) {
	tailTest := NewTailTest("tell-position", t)
	tailTest.CreateFile("test.txt", "hello\nworld\nagain\nmore\n")
	config := Config{
		Location: &SeekInfo{0, io.SeekStart}}
	tail := tailTest.StartTail("test.txt", config)
	// read noe line
	<-tail.Lines
	offset, err := tail.Tell()
	if err != nil {
		tailTest.Errorf("Tell return error: %s", err.Error())
	}
	tail.Done()
	// tail.close()

	config = Config{Location: &SeekInfo{offset, io.SeekStart}}
	tail = tailTest.StartTail("test.txt", config)
	l := <-tail.Lines

	if l.Text != "world" && l.Text != "again" {
		tailTest.Fatalf("mismatch; expected world or again, but got %s", l.Text)
	}

	tailTest.RemoveFile("test.txt")
	tail.Done()
}

func reOpen(t *testing.T) {
	delay := 1000 * time.Millisecond

	tailTest := NewTailTest("reopen-polling", t)
	tailTest.CreateFile("test.txt", "hello\nworld\n")
	tail := tailTest.StartTail(
		"test.txt",
		Config{PollOptions: testPollingOptions},
	)
	content := []string{"hello", "world", "more", "data", "endofworld"}
	go tailTest.VerifyTailOutput(tail, content, false)

	// deletion must trigger reopen
	<-time.After(delay)
	tailTest.RemoveFile("test.txt")
	<-time.After(delay)
	tailTest.CreateFile("test.txt", "more\ndata\n")

	// rename must trigger reopen
	<-time.After(delay)
	tailTest.RenameFile("test.txt", "test.txt.rotated")
	<-time.After(delay)
	tailTest.CreateFile("test.txt", "endofworld\n")

	// Delete after a reasonable delay, to give tail sufficient time
	// to read all lines.
	<-time.After(delay)
	tailTest.RemoveFile("test.txt")
	<-time.After(delay)
}

func reSeek(t *testing.T) {
	tailTest := NewTailTest("reseek-polling", t)
	tailTest.CreateFile("test.txt", "a really long string goes here\nhello\nworld\n")
	tail := tailTest.StartTail(
		"test.txt",
		Config{PollOptions: testPollingOptions})

	go tailTest.VerifyTailOutput(tail, []string{"a really long string goes here", "hello", "world", "h311o", "w0r1d", "endofworld"}, false)

	// truncate now
	<-time.After(100 * time.Millisecond)
	tailTest.TruncateFile("test.txt", "h311o\nw0r1d\nendofworld\n")

	// Delete after a reasonable delay, to give tail sufficient time
	// to read all lines.
	<-time.After(100 * time.Millisecond)
	tailTest.RemoveFile("test.txt")

	// Do not bother with stopping as it could kill the tomb during
	// the reading of data written above. Timings can vary based on
	// test environment.
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

	close(tailTest.done)
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

	close(tailTest.done)
}

// Test library

type TailTest struct {
	Name string
	path string
	done chan struct{}
	*testing.T
}

func NewTailTest(name string, t *testing.T) TailTest {
	tt := TailTest{name, t.TempDir() + "/" + name, make(chan struct{}), t}
	err := os.MkdirAll(tt.path, os.ModeTemporary|0700)
	if err != nil {
		tt.Fatal(err)
	}

	return tt
}

func (t TailTest) CreateFile(name string, contents string) {
	err := os.WriteFile(t.path+"/"+name, []byte(contents), 0600)
	if err != nil {
		t.Fatal(err)
	}
}

func (t TailTest) AppendToFile(name string, contents string) {
	err := os.WriteFile(t.path+"/"+name, []byte(contents), 0600|os.ModeAppend)
	if err != nil {
		t.Fatal(err)
	}
}

func (t TailTest) RemoveFile(name string) {
	err := os.Remove(t.path + "/" + name)
	if err != nil {
		t.Fatal(err)
	}
}

func (t TailTest) RenameFile(oldname string, newname string) {
	oldname = t.path + "/" + oldname
	newname = t.path + "/" + newname
	err := os.Rename(oldname, newname)
	if err != nil {
		t.Fatal(err)
	}
}

func (t TailTest) AppendFile(name string, contents string) {
	f, err := os.OpenFile(t.path+"/"+name, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, err = f.WriteString(contents)
	if err != nil {
		t.Fatal(err)
	}
}

func (t TailTest) TruncateFile(name string, contents string) {
	f, err := os.OpenFile(t.path+"/"+name, os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, err = f.WriteString(contents)
	if err != nil {
		t.Fatal(err)
	}
}

func (t TailTest) StartTail(name string, config Config) *Tail {
	tail, err := TailFile(t.path+"/"+name, config)
	if err != nil {
		t.Fatal(err)
	}
	return tail
}

func (t TailTest) VerifyTailOutput(tail *Tail, lines []string, expectEOF bool) {
	defer close(t.done)
	t.ReadLines(tail, lines)
	// It is important to do this if only EOF is expected
	// otherwise we could block on <-tail.Lines
	if expectEOF {
		line, ok := <-tail.Lines
		if ok {
			t.Fatalf("more content from tail: %+v", line)
		}
	}
}

func (t TailTest) ReadLines(tail *Tail, lines []string) {
	for idx, line := range lines {
		tailedLine, ok := <-tail.Lines
		if !ok {
			// tail.Lines is closed and empty.
			err := tail.Err()
			if err != nil {
				t.Fatalf("tail ended with error: %v", err)
			}
			t.Fatalf("tail ended early; expecting more: %v", lines[idx:])
		}
		if tailedLine == nil {
			t.Fatalf("tail.Lines returned nil; not possible")
		}
		// Note: not checking .Err as the `lines` argument is designed
		// to match error strings as well.
		if tailedLine.Text != line {
			t.Fatalf(
				"unexpected line/err from tail: "+
					"expecting <<%s>>>, but got <<<%s>>>",
				line, tailedLine.Text)
		}
	}
}
