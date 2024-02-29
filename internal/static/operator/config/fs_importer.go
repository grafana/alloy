package config

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	jsonnet "github.com/google/go-jsonnet"
)

// FSImporter implements jsonnet.Importer for a fs.FS.
type FSImporter struct {
	fs fs.FS

	cache map[string]jsonnet.Contents
	paths []string
}

// NewFSImporter creates a new jsonnet VM Importer that uses the given fs.
func NewFSImporter(f fs.FS, paths []string) *FSImporter {
	return &FSImporter{
		fs:    f,
		cache: make(map[string]jsonnet.Contents),
		paths: paths,
	}
}

// Import implements jsonnet.Importer.
func (i *FSImporter) Import(importedFrom, importedPath string) (contents jsonnet.Contents, foundAt string, err error) {
	tryPaths := append([]string{importedFrom}, i.paths...)
	for _, p := range tryPaths {
		cleanedPath := path.Clean(
			path.Join(path.Dir(p), importedPath),
		)
		cleanedPath = strings.TrimPrefix(cleanedPath, "./")

		c, fa, err := i.tryImport(cleanedPath)
		if err == nil {
			return c, fa, err
		}
	}

	return jsonnet.Contents{}, "", fmt.Errorf("no such file: %s", importedPath)
}

func (i *FSImporter) tryImport(path string) (contents jsonnet.Contents, foundAt string, err error) {
	// jsonnet expects the same "foundAt" to always return the same instance of
	// contents, so we need to return a cache here.
	if c, ok := i.cache[path]; ok {
		return c, path, nil
	}

	f, err := i.fs.Open(path)
	if err != nil {
		err = jsonnet.RuntimeError{Msg: err.Error()}
		return
	}

	var buf bytes.Buffer
	if _, copyErr := io.Copy(&buf, f); copyErr != nil {
		err = jsonnet.RuntimeError{Msg: copyErr.Error()}
		return
	}

	contents = jsonnet.MakeContents(buf.String())
	i.cache[path] = contents
	return contents, path, nil
}
