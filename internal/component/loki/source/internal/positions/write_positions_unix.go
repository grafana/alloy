//go:build !windows

package positions

// This code is copied from Promtail. The positions package allows logging
// components to keep track of read file offsets on disk and continue from the
// same place in case of a restart.

import (
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
)

const positionFileMode = 0600

// atomicWriteFile writes buf to filename using a deterministic temp file
// (<target>.tmp), fsyncs, renames over the target, then fsyncs the directory.
// This reuses the same dentry on every write instead of allocating a new one
// (as renameio.WriteFile does), which prevents unbounded kernel dentry slab
// growth under cgroup v2.
func atomicWriteFile(filename string, buf []byte) error {
	target := filepath.Clean(filename)
	tmp := target + ".tmp"

	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(positionFileMode))
	if err != nil {
		return err
	}

	if _, err := f.Write(buf); err != nil {
		f.Close()
		return err
	}

	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmp, target); err != nil {
		return err
	}

	dir, err := os.Open(filepath.Dir(target))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func writePositionFile(filename string, positions map[Entry]string) error {
	buf, err := yaml.Marshal(File{
		Positions: positions,
	})
	if err != nil {
		return err
	}

	return atomicWriteFile(filename, buf)
}
