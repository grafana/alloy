// Package atomicfile replaces the contents of a file in a single step, so that
// a reader either sees the previous contents or the new ones, never a partial
// write.
//
// It exists because the obvious implementations of that, os.CreateTemp
// followed by a rename, github.com/google/renameio and
// github.com/natefinch/atomic, all name their temporary file after a fresh
// random number. That is the right default for a one off write, but Alloy
// rewrites several small files on a timer, and a name that is never repeated
// leaves the kernel holding a directory entry it can never reuse.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/natefinch/atomic"
)

// tempSuffix is appended to the destination name to build the temporary file
// that Write replaces it with.
//
// The suffix is fixed rather than random, which is the whole point of this
// package. Callers rewrite the same file every few seconds, so a unique name
// per write leaves behind a dentry that nothing will ever look up again. Under
// cgroup v2 the kernel memory backing those dentries is charged to the
// container and counts towards its memory limit, so a collector that ships
// almost nothing still reports steadily climbing memory use, while the Go heap
// and RSS stay flat and point at nothing. Reusing one name keeps a single
// dentry alive instead.
//
// See https://github.com/grafana/alloy/issues/6938.
const tempSuffix = ".tmp"

// TempPath returns the path of the temporary file that Write creates while
// replacing path. Callers do not need it, but tests rely on the path being
// derived from the destination rather than picked at random.
func TempPath(path string) string {
	return filepath.Clean(path) + tempSuffix
}

// Write replaces the contents of path with data.
//
// The data is written to a temporary file in the same directory and flushed to
// disk, and only then moved over path. A failure at any point leaves path with
// its previous contents.
//
// perm applies when path does not exist yet. An existing file keeps the
// permissions it already has, so that replacing it does not quietly widen or
// narrow them.
//
// Write is not safe to call concurrently for the same path, from this process
// or from another one, because the temporary file is named after the
// destination. Every caller in Alloy writes its file from a single goroutine.
func Write(path string, data []byte, perm os.FileMode) error {
	dest := filepath.Clean(path)
	temp := TempPath(dest)

	if err := writeTemp(temp, data, destPerm(dest, perm)); err != nil {
		// A temporary file left behind is harmless, the next write truncates
		// it, but removing it keeps the directory clean when the failure is
		// not a transient one.
		os.Remove(temp)
		return err
	}

	// ReplaceFile is os.Rename on Unix and MoveFileEx on Windows. Both replace
	// an existing destination, which os.Rename on its own does not promise on
	// every platform.
	if err := atomic.ReplaceFile(temp, dest); err != nil {
		os.Remove(temp)
		return fmt.Errorf("replacing %s: %w", dest, err)
	}

	return nil
}

// writeTemp writes data to temp with the given permissions and flushes it to
// disk, ready to be moved over the destination.
func writeTemp(temp string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("creating %s: %w", temp, err)
	}

	// The temporary file name is reused between writes, so this file may be
	// one that an earlier run left behind, still carrying the permissions it
	// was created with: O_CREATE only applies perm to a file that did not
	// exist. Setting them explicitly also sidesteps the umask, which would
	// otherwise make the result depend on the environment Alloy was started
	// in.
	if err := f.Chmod(perm); err != nil {
		f.Close()
		return fmt.Errorf("setting permissions on %s: %w", temp, err)
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("writing %s: %w", temp, err)
	}

	// Flushing before the move matters: rename is atomic with respect to other
	// readers, but not with respect to a crash. Without the sync the
	// destination can come back empty afterwards, which for a positions or
	// bookmark file means silently rereading everything from the start.
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("flushing %s: %w", temp, err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", temp, err)
	}

	return nil
}

// destPerm returns the permissions to give the temporary file: those of an
// existing destination, so replacing it does not change them, and perm when
// there is nothing to replace yet.
func destPerm(dest string, perm os.FileMode) os.FileMode {
	if fi, err := os.Stat(dest); err == nil {
		return fi.Mode().Perm()
	}
	return perm.Perm()
}
