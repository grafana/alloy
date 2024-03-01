//go:build windows
// +build windows

// This code is copied from Promtail v1.6.2-0.20231004111112-07cbef92268a with minor changes.

package windowsevent

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/natefinch/atomic"

	"github.com/grafana/loki/clients/pkg/promtail/targets/windows/win_eventlog"
)

type bookMark struct {
	handle win_eventlog.EvtHandle
	isNew  bool
	path   string
	buf    []byte
}

// newBookMark creates a new windows event bookmark.
// The bookmark will be saved at the given path. Use save to save the current position for a given event.
func newBookMark(path string) (*bookMark, error) {
	// 16kb buffer for rendering bookmark
	buf := make([]byte, 16<<10)

	_, err := os.Stat(path)
	// creates a new bookmark file if none exists.
	if errors.Is(err, fs.ErrNotExist) {
		_, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		bm, err := win_eventlog.CreateBookmark("")
		if err != nil {
			return nil, err
		}
		return &bookMark{
			handle: bm,
			path:   path,
			isNew:  true,
			buf:    buf,
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
		handle: bm,
		path:   path,
		isNew:  fileString == "",
		buf:    buf,
	}, nil
}

// save Saves the bookmark at the current event position.
func (b *bookMark) save(event win_eventlog.EvtHandle) error {
	newBookmark, err := win_eventlog.UpdateBookmark(b.handle, event, b.buf)
	if err != nil {
		return err
	}
	return atomic.WriteFile(b.path, bytes.NewReader([]byte(newBookmark)))
}
