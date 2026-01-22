//go:build linux || darwin || freebsd || netbsd || openbsd

package fileext

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/natefinch/atomic"
)

func OpenFile(name string) (file *os.File, err error) {
	filename := name
	// Check if the path requested is a symbolic link
	fi, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		filename, err = filepath.EvalSymlinks(name)
		if err != nil {
			return nil, err
		}
	}
	return os.Open(filename)
}

func IsDeletePending(_ *os.File) (bool, error) {
	return false, nil
}

// AtomicWrite performs an atomic write on POSIX systems using the atomic package.
func AtomicWrite(name string, content []byte) error {
	return atomic.WriteFile(name, bytes.NewReader(content))
}
