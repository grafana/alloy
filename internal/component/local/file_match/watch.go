package file_match

import (
	"os"
	"path/filepath"

	"github.com/go-kit/log"

	"github.com/bmatcuk/doublestar"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/flow/logging/level"
)

// watch handles a single discovery.target for file watching.
type watch struct {
	target discovery.Target
	log    log.Logger
}

func (w *watch) getPaths() ([]discovery.Target, error) {
	allMatchingPaths := make([]discovery.Target, 0)

	matches, err := doublestar.Glob(w.getPath())
	if err != nil {
		return nil, err
	}
	exclude := w.getExcludePath()
	for _, m := range matches {
		if exclude != "" {
			if match, _ := doublestar.PathMatch(exclude, m); match {
				continue
			}
		}
		abs, err := filepath.Abs(m)
		if err != nil {
			level.Error(w.log).Log("msg", "error getting absolute path", "path", m, "err", err)
			continue
		}
		fi, err := os.Stat(abs)
		if err != nil {
			level.Error(w.log).Log("msg", "error getting os stat", "path", abs, "err", err)
			continue
		}
		if fi.IsDir() {
			continue
		}
		dt := discovery.Target{}
		for dk, v := range w.target {
			dt[dk] = v
		}
		dt["__path__"] = abs
		allMatchingPaths = append(allMatchingPaths, dt)
	}

	return allMatchingPaths, nil
}

func (w *watch) getPath() string {
	return w.target["__path__"]
}

func (w *watch) getExcludePath() string {
	return w.target["__path_exclude__"]
}
