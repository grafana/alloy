//go:build windows

package windowsevent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/loki/source/windowsevent/win_eventlog"
)

func TestNewBookMarkKeepsSavedPosition(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "bookmark.xml")
	// As UpdateBookmark renders it: the UTF-16 terminator ends up in the file, and stringPointer drops it again.
	saved := "<BookmarkList>\r\n  <Bookmark Channel='Application' RecordId='1' IsCurrent='true'/>\r\n</BookmarkList>\x00"
	require.NoError(t, os.WriteFile(filePath, []byte(saved), 0600))

	bm, err := newBookMark(filePath)
	require.NoError(t, err)
	defer win_eventlog.Close([]win_eventlog.EvtHandle{bm.handle})

	require.False(t, bm.isNew)
	require.Equal(t, saved, bm.bookmarkStr.Load())

	// Saving before a new event has been read keeps the loaded position.
	require.NoError(t, bm.save())
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, saved, string(content))
}

func TestNewBookMarkInvalidContent(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "bookmark.xml")
	require.NoError(t, os.WriteFile(filePath, []byte("not a bookmark"), 0600))

	bm, err := newBookMark(filePath)
	require.NoError(t, err)
	defer win_eventlog.Close([]win_eventlog.EvtHandle{bm.handle})

	require.True(t, bm.isNew)
	require.Equal(t, "", bm.bookmarkStr.Load())
}
