//go:build windows

package tail

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// atomicWrite performs an atomic write on Windows using ReplaceFileW.
// This allows replacing a file when it's open with FILE_SHARE_DELETE.
// This function is only used in tests.
func atomicWrite(t *testing.T, name, newContent string) {
	var (
		kernel32     = windows.NewLazySystemDLL("kernel32.dll")
		replaceFileW = kernel32.NewProc("ReplaceFileW")
	)

	dir, filename := filepath.Dir(name), filepath.Base(name)

	// Create temp file in the same directory.
	tmp, err := os.CreateTemp(dir, filename+".tmp")
	require.NoError(t, err)

	tmpName := tmp.Name()

	// Write content to temp file.
	_, err = tmp.Write([]byte(newContent))
	require.NoError(t, err)

	// Sync to ensure data is written.
	require.NoError(t, tmp.Sync())

	// Close the temp file before replacing.
	require.NoError(t, tmp.Close())

	// Convert paths to UTF-16 for ReplaceFileW
	replacedPath, err := syscall.UTF16PtrFromString(name)
	require.NoError(t, err)

	replacementPath, err := syscall.UTF16PtrFromString(tmpName)
	require.NoError(t, err)

	// Use ReplaceFileW to atomically replace the target file with the temp file
	// This works when target file is open with FILE_SHARE_DELETE.
	// https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-replacefilew
	ret, _, _ := replaceFileW.Call(
		uintptr(unsafe.Pointer(replacedPath)),
		uintptr(unsafe.Pointer(replacementPath)),
		0, // NULL backup file
		0, // No flags
		0, // NULL exclude
		0, // NULL reserved
	)

	// ReplaceFileW returns 0 if it failed and 1 if it succeeded.
	require.Equal(t, uintptr(1), ret)
}
