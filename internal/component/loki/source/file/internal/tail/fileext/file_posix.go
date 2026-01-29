//go:build linux || darwin || freebsd || netbsd || openbsd

package fileext

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
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

// FileID uniquely identifies a file on the filesystem using device and inode.
// Two different paths pointing to the same file (via symlinks or hard links)
// will have the same FileID.
type FileID struct {
	dev uint64
	ino uint64
}

// String returns a string representation of the FileID for use in metrics and logs.
func (f FileID) String() string {
	return fmt.Sprintf("%d:%d", f.dev, f.ino)
}

// NewFileID extracts a unique file identifier from os.FileInfo.
// On POSIX systems, this uses the device and inode numbers.
// Returns the FileID and true if successful, or an empty FileID and false
// if the file identity could not be determined.
func NewFileID(fi os.FileInfo) (FileID, bool) {
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return FileID{}, false
	}
	return FileID{
		dev: uint64(stat.Dev),
		ino: stat.Ino,
	}, true
}
