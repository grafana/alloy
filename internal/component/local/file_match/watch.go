package file_match

import (
	"os"
	"path/filepath"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
)

// watch handles a single discovery.target for file watching.
type watch struct {
	target          discovery.Target
	log             log.Logger
	ignoreOlderThan time.Duration
}

func (w *watch) getPaths() ([]discovery.Target, error) {
	allMatchingPaths := make([]discovery.Target, 0)

	matches, err := doublestar.FilepathGlob(w.getPath())
	if err != nil {
		return nil, err
	}
	exclude := w.getExcludePath()
	for _, m := range matches {
		if exclude != "" {
			if match, _ := doublestar.PathMatch(filepath.FromSlash(exclude), m); match {
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
			// On some filesystems we can get errors accessing the discovered paths. Don't log these as errors.
			// local.file_match will retry on the next sync period if the access is blocked temporarily only.
			if util.IsEphemeralOrFileClosed(err) {
				level.Debug(w.log).Log("msg", "I/O error when getting os stat", "path", abs, "err", err)
			} else {
				level.Error(w.log).Log("msg", "error getting os stat", "path", abs, "err", err)
			}
			continue
		}

		if fi.IsDir() {
			continue
		}

		if w.ignoreOlderThan != 0 && fi.ModTime().Before(time.Now().Add(-w.ignoreOlderThan)) {
			continue
		}

		tb := discovery.NewTargetBuilderFrom(w.target)
		tb.Set("__path__", abs)
		allMatchingPaths = append(allMatchingPaths, tb.Target())
	}

	return allMatchingPaths, nil
}

func (w *watch) getPath() string {
	path, _ := w.target.Get("__path__")
	return path
}

func (w *watch) getExcludePath() string {
	excludePath, _ := w.target.Get("__path_exclude__")
	return excludePath
}
