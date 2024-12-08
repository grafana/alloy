package file_match

import (
	"os"
	"path/filepath"
	"time"

	"github.com/bmatcuk/doublestar"
	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/component/common/loki/utils"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// watch handles a single discovery.target for file watching.
type watch struct {
	target      discovery.Target
	log         log.Logger
	ignoreOlder time.Duration
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
			// On some filesystems we can get errors accessing the discovered paths. Don't log these as errors.
			// local.file_match will retry on the next sync period if the access is blocked temporarily only.
			if utils.IsEphemeralOrFileClosed(err) {
				level.Debug(w.log).Log("msg", "I/O error when getting os stat", "path", abs, "err", err)
			} else {
				level.Error(w.log).Log("msg", "error getting os stat", "path", abs, "err", err)
			}
			continue
		}

		if w.ignoreOlder != 0 && fi.ModTime().Before(time.Now().Add(-w.ignoreOlder)) {
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
