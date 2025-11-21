//go:build windows
// +build windows

// This code is adapted from loki/promtail. Last revision used to port changes to Alloy was v1.6.2-0.20231004111112-07cbef92268a.

package windowsevent

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/natefinch/atomic"
	uberAtomic "go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/loki/source/windowsevent/win_eventlog"
)

type bookMark struct {
	handle win_eventlog.EvtHandle
	isNew  bool
	path   string
	buf    []byte

	bookmarkStr *uberAtomic.String
}

// newBookMark creates a new windows event bookmark.
// The bookmark will be saved at the given path. Use save to save the current position for a given event.
func newBookMark(path string) (*bookMark, error) {
	// 16kb buffer for rendering bookmark
	buf := make([]byte, 16<<10)

	_, err := os.Stat(path)
	// creates a new bookmark file if none exists.
	if errors.Is(err, fs.ErrNotExist) {
		f, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		bm, err := win_eventlog.CreateBookmark("")
		if err != nil {
			return nil, err
		}
		return &bookMark{
			handle:      bm,
			path:        path,
			isNew:       true,
			buf:         buf,
			bookmarkStr: uberAtomic.NewString(""),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	// otherwise open the current one.
	file, err := os.OpenFile(path, os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	fileString := string(fileContent)
	// load the current bookmark.
	bm, err := win_eventlog.CreateBookmark(fileString)
	if err != nil {
		// If we errored likely due to incorrect data then create a blank one
		bm, err = win_eventlog.CreateBookmark("")
		fileString = ""
		// This should never fail but just in case.
		if err != nil {
			return nil, err
		}
	}
	return &bookMark{
		handle:      bm,
		path:        path,
		isNew:       fileString == "",
		buf:         buf,
		bookmarkStr: uberAtomic.NewString(""),
	}, nil
}

func (b *bookMark) update(event win_eventlog.EvtHandle) error {
	newBookmark, err := win_eventlog.UpdateBookmark(b.handle, event, b.buf)
	if err != nil {
		return err
	}
	b.bookmarkStr.Store(newBookmark)
	return nil
}

// save Saves the bookmark at the current event position.
func (b *bookMark) save() error {
	return atomic.WriteFile(b.path, bytes.NewReader([]byte(b.bookmarkStr.Load())))
}
