package importsource

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	filedetector "github.com/grafana/alloy/internal/filedetector"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax/vm"
)

// ImportFile imports a module from a file or a folder.
type ImportFile struct {
	managedOpts     component.Options
	eval            *vm.Evaluator
	onContentChange func(map[string]string)
	logger          log.Logger

	reloadCh chan struct{}
	args     FileArguments

	mut      sync.RWMutex
	detector io.Closer

	healthMut sync.RWMutex
	health    component.Health
}

// waitReadPeriod holds the time to wait before reading a file while the
// source is running.
//
// This prevents from updating too frequently and exporting partial writes.
const waitReadPeriod time.Duration = 30 * time.Millisecond

var _ ImportSource = (*ImportFile)(nil)

func NewImportFile(managedOpts component.Options, eval *vm.Evaluator, onContentChange func(map[string]string)) *ImportFile {
	opts := managedOpts
	return &ImportFile{
		reloadCh:        make(chan struct{}, 1),
		managedOpts:     opts,
		eval:            eval,
		onContentChange: onContentChange,
		logger:          managedOpts.Logger,
	}
}

type FileArguments struct {
	// Filename indicates the file to watch.
	Filename string `alloy:"filename,attr"`
	// Type indicates how to detect changes to the file.
	Type filedetector.Detector `alloy:"detector,attr,optional"`
	// PollFrequency determines the frequency to check for changes when Type is Poll.
	PollFrequency time.Duration `alloy:"poll_frequency,attr,optional"`
}

var DefaultFileArguments = FileArguments{
	Type:          filedetector.DetectorFSNotify,
	PollFrequency: time.Minute,
}

// SetToDefault implements syntax.Defaulter.
func (a *FileArguments) SetToDefault() {
	*a = DefaultFileArguments
}

func (im *ImportFile) Evaluate(scope *vm.Scope) error {
	im.mut.Lock()
	defer im.mut.Unlock()

	var arguments FileArguments
	if err := im.eval.Evaluate(scope, &arguments); err != nil {
		return fmt.Errorf("decoding configuration: %w", err)
	}

	if reflect.DeepEqual(im.args, arguments) {
		return nil
	}
	im.args = arguments

	// Force an immediate read of the file to report any potential errors early.
	if err := im.readFile(); err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	reloadFile := func() {
		select {
		case im.reloadCh <- struct{}{}:
		default:
			// no-op: a reload is already queued so we don't need to queue a second
			// one.
		}
	}

	if im.detector != nil {
		if err := im.detector.Close(); err != nil {
			level.Error(im.managedOpts.Logger).Log("msg", "failed to shut down detector during eval", "err", err)
			// We don't return the error here because it's just a memory leak.
		}
	}

	var err error
	switch im.args.Type {
	case filedetector.DetectorPoll:
		im.detector = filedetector.NewPoller(filedetector.PollerOptions{
			Filename:      im.args.Filename,
			ReloadFile:    reloadFile,
			PollFrequency: im.args.PollFrequency,
		})
	case filedetector.DetectorFSNotify:
		im.detector, err = filedetector.NewFSNotify(filedetector.FSNotifyOptions{
			Logger:        im.managedOpts.Logger,
			Filename:      im.args.Filename,
			ReloadFile:    reloadFile,
			PollFrequency: im.args.PollFrequency,
		})
	}

	return err
}

func (im *ImportFile) Run(ctx context.Context) error {
	defer func() {
		im.mut.Lock()
		defer im.mut.Unlock()
		if err := im.detector.Close(); err != nil {
			level.Error(im.managedOpts.Logger).Log("msg", "failed to shut down detector", "err", err)
		}
		im.detector = nil
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-im.reloadCh:
			time.Sleep(waitReadPeriod)

			// We ignore the error here from readFile since readFile will log errors
			// and also report the error and update the health of the source.
			_ = im.readFile()
		}
	}
}

func (im *ImportFile) readFile() error {
	files, dir, err := im.collectFiles()
	if err != nil {
		im.setHealth(component.Health{
			Health:     component.HealthTypeUnhealthy,
			Message:    fmt.Sprintf("failed to collect files: %s", err),
			UpdateTime: time.Now(),
		})
		level.Error(im.managedOpts.Logger).Log("msg", "failed to collect files", "err", err)
		return err
	}
	fileContents := make(map[string]string)
	for _, f := range files {
		fpath := f
		if dir {
			fpath = filepath.Join(im.args.Filename, fpath)
		}
		bb, err := os.ReadFile(fpath)
		if err != nil {
			im.setHealth(component.Health{
				Health:     component.HealthTypeUnhealthy,
				Message:    fmt.Sprintf("failed to read file: %s", err),
				UpdateTime: time.Now(),
			})
			level.Error(im.managedOpts.Logger).Log("msg", "failed to read file", "file", fpath, "err", err)
			return err
		}
		fileContents[f] = string(bb)
	}

	im.setHealth(component.Health{
		Health:     component.HealthTypeHealthy,
		Message:    "read file",
		UpdateTime: time.Now(),
	})
	im.onContentChange(fileContents)
	return nil
}

func (im *ImportFile) CurrentHealth() component.Health {
	im.healthMut.RLock()
	defer im.healthMut.RUnlock()
	return im.health
}

func (im *ImportFile) setHealth(h component.Health) {
	im.healthMut.Lock()
	defer im.healthMut.Unlock()
	im.health = h
}

func (im *ImportFile) collectFiles() (content []string, dir bool, err error) {
	fpath := im.args.Filename
	fi, err := os.Stat(fpath)
	if err != nil {
		return nil, false, err
	}

	files := make([]string, 0)
	dir = fi.IsDir()
	if dir {
		files, err = collectFilesFromDir(fpath)
		if err != nil {
			return nil, true, err
		}
	} else {
		files = append(files, fpath)
	}
	return files, dir, nil
}

func collectFilesFromDir(path string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(path, func(curPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// skip all directories and don't recurse into child dirs that aren't at top-level
		if d.IsDir() {
			if curPath != path {
				return filepath.SkipDir
			}
			return nil
		}
		// ignore files not ending in .alloy extension
		if !strings.HasSuffix(curPath, ".alloy") {
			return nil
		}

		files = append(files, d.Name())
		return err
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// Update the evaluator.
func (im *ImportFile) SetEval(eval *vm.Evaluator) {
	im.eval = eval
}

func (im *ImportFile) ModulePath() string {
	path, err := util.ExtractDirPath(im.args.Filename)

	if err != nil {
		level.Error(im.managedOpts.Logger).Log("msg", "failed to extract module path", "module path", im.args.Filename, "err", err)
	}
	return path
}
