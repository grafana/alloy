package remotecfg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// filesDirName is the directory under the remotecfg storage path where
// auxiliary files received from the API are stored.
const filesDirName = "files"

// tmpFilePrefix is prepended to file names while they are being written, so
// that readers never observe a partially written file.
const tmpFilePrefix = ".alloy-tmp-"

// validateFileName checks that name is a plain file name which can be safely
// written inside the files directory.
func validateFileName(name string) error {
	switch {
	case name == "":
		return errors.New("file name must not be empty")
	case name == "." || name == "..":
		return fmt.Errorf("invalid file name %q", name)
	case strings.ContainsAny(name, `/\`) || filepath.Base(name) != name || !filepath.IsLocal(name):
		return fmt.Errorf("file name %q must not contain path separators", name)
	case strings.HasPrefix(name, tmpFilePrefix):
		return fmt.Errorf("file name %q uses reserved prefix %q", name, tmpFilePrefix)
	}
	return nil
}

// syncFiles makes the contents of dir match files exactly: files are written
// atomically if their contents changed, and any entry not present in files is
// removed. All file names are validated before anything is written.
func syncFiles(dir string, files map[string][]byte) error {
	for name := range files {
		if err := validateFileName(name); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create files directory: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("failed to open files directory: %w", err)
	}
	defer root.Close()

	for name, content := range files {
		// Skip unchanged files to avoid needlessly triggering file watchers.
		if existing, err := root.ReadFile(name); err == nil && bytes.Equal(existing, content) {
			continue
		}

		tmp := tmpFilePrefix + name
		if err := root.WriteFile(tmp, content, 0640); err != nil {
			_ = root.Remove(tmp)
			return fmt.Errorf("failed to write file %q: %w", name, err)
		}
		if err := root.Rename(tmp, name); err != nil {
			_ = root.Remove(tmp)
			return fmt.Errorf("failed to write file %q: %w", name, err)
		}
	}

	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return fmt.Errorf("failed to list files directory: %w", err)
	}
	var errs []error
	for _, e := range entries {
		if _, ok := files[e.Name()]; ok {
			continue
		}
		if err := root.RemoveAll(e.Name()); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove stale file %q: %w", e.Name(), err))
		}
	}
	return errors.Join(errs...)
}

// readFiles returns the regular files currently stored in dir. A missing
// directory is treated as having no files.
func readFiles(dir string) (map[string][]byte, error) {
	root, err := os.OpenRoot(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string][]byte{}, nil
	} else if err != nil {
		return nil, err
	}
	defer root.Close()

	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, err
	}
	files := make(map[string][]byte, len(entries))
	for _, e := range entries {
		if !e.Type().IsRegular() || validateFileName(e.Name()) != nil {
			continue
		}
		b, err := root.ReadFile(e.Name())
		if err != nil {
			return nil, err
		}
		files[e.Name()] = b
	}
	return files, nil
}

// getConfigHash returns a hash identifying a configuration and its auxiliary
// files. When there are no files it is identical to getHash(content).
func getConfigHash(content []byte, files map[string][]byte) string {
	if len(files) == 0 {
		return getHash(content)
	}

	h := fnv.New32()
	writeField := func(b []byte) {
		_ = binary.Write(h, binary.LittleEndian, uint64(len(b)))
		h.Write(b)
	}
	writeField(content)
	for _, name := range slices.Sorted(maps.Keys(files)) {
		writeField([]byte(name))
		writeField(files[name])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
