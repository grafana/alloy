package discover

import (
	"io/fs"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

type config struct {
	exclude  []string
	skipDirs []string
}

// Option configures Files.
type Option func(*config)

// WithExclude skips files whose path relative to root equals, or is nested
// under, any of the given paths (e.g. "tools/generate/testdata").
func WithExclude(paths ...string) Option {
	return func(c *config) { c.exclude = append(c.exclude, paths...) }
}

// WithSkipDirs skips any directory with one of the given names (e.g. "vendor", ".git", "node_modules").
func WithSkipDirs(names ...string) Option {
	return func(c *config) { c.skipDirs = append(c.skipDirs, names...) }
}

// Result holds the files discovered by Files.
type Result struct {
	files []string
}

// Files returns the paths of all discovered files, sorted.
func (r Result) Files() []string {
	return r.files
}

// Dirs returns the unique directories containing the discovered files, sorted.
func (r Result) Dirs() []string {
	seen := make(map[string]struct{}, len(r.files))
	var dirs []string
	for _, f := range r.files {
		dir := filepath.Dir(f)
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

// Files returns every file under root whose base name matches the glob
// pattern (see filepath.Match), joined with root and sorted.
func Files(root, pattern string, opts ...Option) (Result, error) {
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}

	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if slices.Contains(cfg.skipDirs, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		ok, err := filepath.Match(pattern, d.Name())
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for _, ex := range cfg.exclude {
			if rel == ex || strings.HasPrefix(rel, ex+string(filepath.Separator)) {
				return nil
			}
		}

		found = append(found, path)
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	sort.Strings(found)
	return Result{files: found}, nil
}
