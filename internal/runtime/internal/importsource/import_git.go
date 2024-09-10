package importsource

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/vcs"
	"github.com/grafana/alloy/syntax/vm"
)

// ImportGit imports a module from a git repository.
// There are currently no remote.git component, the logic is implemented here.
type ImportGit struct {
	opts            component.Options
	log             log.Logger
	eval            *vm.Evaluator
	mut             sync.RWMutex
	repo            *vcs.GitRepo
	repoOpts        vcs.GitRepoOptions
	args            GitArguments
	onContentChange func(map[string]string)

	argsChanged chan struct{}

	healthMut sync.RWMutex
	health    component.Health
}

var (
	_ ImportSource              = (*ImportGit)(nil)
	_ component.Component       = (*ImportGit)(nil)
	_ component.HealthComponent = (*ImportGit)(nil)
)

type GitArguments struct {
	Repository    string            `alloy:"repository,attr"`
	Revision      string            `alloy:"revision,attr,optional"`
	Path          string            `alloy:"path,attr"`
	PullFrequency time.Duration     `alloy:"pull_frequency,attr,optional"`
	GitAuthConfig vcs.GitAuthConfig `alloy:",squash"`
}

var DefaultGitArguments = GitArguments{
	Revision:      "main",
	PullFrequency: time.Minute,
}

// SetToDefault implements syntax.Defaulter.
func (args *GitArguments) SetToDefault() {
	*args = DefaultGitArguments
}

func NewImportGit(managedOpts component.Options, eval *vm.Evaluator, onContentChange func(map[string]string)) *ImportGit {
	return &ImportGit{
		opts:            managedOpts,
		log:             managedOpts.Logger,
		eval:            eval,
		argsChanged:     make(chan struct{}, 1),
		onContentChange: onContentChange,
	}
}

func (im *ImportGit) Evaluate(scope *vm.Scope) error {
	var arguments GitArguments
	if err := im.eval.Evaluate(scope, &arguments); err != nil {
		return fmt.Errorf("decoding configuration: %w", err)
	}

	if reflect.DeepEqual(im.args, arguments) {
		return nil
	}

	if err := im.Update(arguments); err != nil {
		return fmt.Errorf("updating component: %w", err)
	}
	return nil
}

func (im *ImportGit) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		ticker  *time.Ticker
		tickerC <-chan time.Time
	)

	for {
		select {
		case <-ctx.Done():
			// TODO: should we stope the ticker here?
			return nil

		case <-im.argsChanged:
			im.mut.Lock()
			pullFrequency := im.args.PullFrequency
			im.mut.Unlock()
			ticker, tickerC = im.updateTicker(pullFrequency, ticker, tickerC)

		case <-tickerC:
			level.Info(im.log).Log("msg", "updating repository")
			im.tickPollFile(ctx)
		}
	}
}

func (im *ImportGit) updateTicker(pullFrequency time.Duration, ticker *time.Ticker, tickerC <-chan time.Time) (*time.Ticker, <-chan time.Time) {
	level.Info(im.log).Log("msg", "updating repository pull frequency, next pull attempt will be done according to the pullFrequency", "new_frequency", pullFrequency)

	if pullFrequency > 0 {
		if ticker == nil {
			ticker = time.NewTicker(pullFrequency)
			tickerC = ticker.C
		} else {
			ticker.Reset(pullFrequency)
		}
		return ticker, tickerC
	}

	if ticker != nil {
		ticker.Stop()
	}
	return nil, nil
}

func (im *ImportGit) tickPollFile(ctx context.Context) {
	im.mut.Lock()
	err := im.pollFile(ctx, im.args)
	pullFrequency := im.args.PullFrequency
	im.mut.Unlock()

	im.updateHealth(err)

	if err != nil {
		level.Error(im.log).Log("msg", "failed to update repository", "pullFrequency", pullFrequency, "err", err)
	}
}

func (im *ImportGit) updateHealth(err error) {
	im.healthMut.Lock()
	defer im.healthMut.Unlock()

	if err != nil {
		im.health = component.Health{
			Health:     component.HealthTypeUnhealthy,
			Message:    err.Error(),
			UpdateTime: time.Now(),
		}
	} else {
		im.health = component.Health{
			Health:     component.HealthTypeHealthy,
			Message:    "module updated",
			UpdateTime: time.Now(),
		}
	}
}

// Update implements component.Component.
// Only acknowledge the error from Update if it's not a
// vcs.UpdateFailedError; vcs.UpdateFailedError means that the Git repo
// exists, but we were unable to update it. It makes sense to retry on the next poll and it may succeed.
func (im *ImportGit) Update(args component.Arguments) (err error) {
	defer func() {
		im.updateHealth(err)
	}()
	im.mut.Lock()
	defer im.mut.Unlock()

	newArgs := args.(GitArguments)

	// TODO(rfratto): store in a repo-specific directory so changing repositories
	// doesn't risk break the module loader if there's a SHA collision between
	// the two different repositories.
	repoPath := filepath.Join(im.opts.DataPath, "repo")

	repoOpts := vcs.GitRepoOptions{
		Repository: newArgs.Repository,
		Revision:   newArgs.Revision,
		Auth:       newArgs.GitAuthConfig,
	}

	// Create or update the repo field.
	// Failure to update repository makes the module loader temporarily use cached contents on disk
	if im.repo == nil || !reflect.DeepEqual(repoOpts, im.repoOpts) {
		r, err := vcs.NewGitRepo(context.Background(), repoPath, repoOpts)
		if err != nil {
			if errors.As(err, &vcs.UpdateFailedError{}) {
				level.Error(im.log).Log("msg", "failed to update repository", "err", err)
				im.updateHealth(err)
			} else {
				return err
			}
		}
		im.repo = r
		im.repoOpts = repoOpts
	}

	if err := im.pollFile(context.Background(), newArgs); err != nil {
		if errors.As(err, &vcs.UpdateFailedError{}) {
			level.Error(im.log).Log("msg", "failed to poll file from repository", "err", err)
			// We don't update the health here because it will be updated via the defer call.
			// This is not very good because if we reassign the err before exiting the function it will not update the health correctly.
			// TODO improve the error  health handling.
		} else {
			return err
		}
	}

	// Schedule an update for handling the changed arguments.
	select {
	case im.argsChanged <- struct{}{}:
	default:
	}

	im.args = newArgs
	return nil
}

// pollFile fetches the latest content from the repository and updates the
// controller. pollFile must only be called with im.mut held.
func (im *ImportGit) pollFile(ctx context.Context, args GitArguments) error {
	// Make sure our repo is up-to-date.
	if err := im.repo.Update(ctx); err != nil {
		return err
	}

	info, err := im.repo.Stat(args.Path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return im.handleDirectory(args.Path)
	}

	return im.handleFile(args.Path)
}

func (im *ImportGit) handleDirectory(path string) error {
	filesInfo, err := im.repo.ReadDir(path)
	if err != nil {
		return err
	}

	content := make(map[string]string)
	for _, fi := range filesInfo {
		if fi.IsDir() || !strings.HasSuffix(fi.Name(), ".alloy") {
			continue
		}
		bb, err := im.repo.ReadFile(filepath.Join(path, fi.Name()))
		if err != nil {
			return err
		}
		content[fi.Name()] = string(bb)
	}
	im.onContentChange(content)
	return nil
}

func (im *ImportGit) handleFile(path string) error {
	bb, err := im.repo.ReadFile(path)
	if err != nil {
		return err
	}
	im.onContentChange(map[string]string{path: string(bb)})
	return nil
}

// CurrentHealth implements component.HealthComponent.
func (im *ImportGit) CurrentHealth() component.Health {
	im.healthMut.RLock()
	defer im.healthMut.RUnlock()
	return im.health
}

// Update the evaluator.
func (im *ImportGit) SetEval(eval *vm.Evaluator) {
	im.eval = eval
}
