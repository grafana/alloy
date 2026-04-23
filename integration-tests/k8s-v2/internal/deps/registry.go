package deps

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
)

// Installer installs/uninstalls a single dependency. Namespace returns the
// kubernetes namespace the installer places its resources into; callers that
// need to pre-create or inspect that namespace can obtain it without knowing
// the installer's internals.
type Installer interface {
	Name() string
	Namespace() string
	Install(ctx context.Context, env Env) error
	Uninstall(ctx context.Context, env Env) error
}

// Registry owns the configured Env and the set of known installers. It has
// no package-level state so Registry instances can safely be created per run.
type Registry struct {
	env        Env
	installers map[string]Installer
}

func NewRegistry(env Env, installerList ...Installer) Registry {
	installers := make(map[string]Installer, len(installerList))
	for _, installer := range installerList {
		installers[installer.Name()] = installer
	}
	return Registry{
		env:        env.withDefaults(),
		installers: installers,
	}
}

func NewDefaultRegistry(env Env) Registry {
	return NewRegistry(env, NewMimirInstaller(), NewLokiInstaller())
}

// Namespace returns the namespace for the named dependency or empty string
// when the dependency is unknown.
func (r Registry) Namespace(name string) string {
	if inst, ok := r.installers[name]; ok {
		return inst.Namespace()
	}
	return ""
}

func (r Registry) Validate(requirements []string) error {
	var unknown []string
	for _, dep := range requirements {
		if _, ok := r.installers[dep]; !ok {
			unknown = append(unknown, dep)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("unknown dependencies: %v", unknown)
	}
	return nil
}

func (r Registry) Install(ctx context.Context, kubeconfig string, requirements []string) error {
	env := r.env
	env.Kubeconfig = kubeconfig

	installed := make([]string, 0, len(requirements))
	failures := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, dep := range requirements {
		dep := dep
		installer := r.installers[dep]
		wg.Add(1)
		go func() {
			defer wg.Done()
			env.Logger.Info("installing dependency", "dependency", dep)
			if err := installer.Install(ctx, env); err != nil {
				env.Logger.Warn("dependency install failed", "dependency", dep, "error", err)
				mu.Lock()
				failures[dep] = err
				mu.Unlock()
				return
			}
			env.Logger.Info("dependency ready", "dependency", dep)
			mu.Lock()
			installed = append(installed, dep)
			mu.Unlock()
		}()
	}

	wg.Wait()
	if len(failures) == 0 {
		return nil
	}

	sort.Strings(installed)
	if len(installed) > 0 {
		env.Logger.Info("rolling back partially installed dependencies", "installed", installed)
		if uninstallErr := r.Uninstall(ctx, kubeconfig, installed); uninstallErr != nil {
			return fmt.Errorf("dependency installs failed (%s) and rollback failed: %w", formatFailureSummary(failures), uninstallErr)
		}
	}
	return fmt.Errorf("dependency installs failed: %s", formatFailureSummary(failures))
}

// Uninstall removes every dependency in requirements. A single failure
// does not abort the run: remaining dependencies are still uninstalled so
// cleanup does not leak resources on the cluster. All accumulated errors
// are returned joined together.
//
// Requirements are walked in reverse order for symmetry with a future
// dependency-ordered install path; today installs run concurrently so the
// reverse is cosmetic.
func (r Registry) Uninstall(ctx context.Context, kubeconfig string, requirements []string) error {
	env := r.env
	env.Kubeconfig = kubeconfig

	reversed := slices.Clone(requirements)
	slices.Reverse(reversed)

	var errs []error
	for _, dep := range reversed {
		env.Logger.Info("uninstalling dependency", "dependency", dep)
		installer := r.installers[dep]
		if err := installer.Uninstall(ctx, env); err != nil {
			env.Logger.Warn("dependency uninstall failed", "dependency", dep, "error", err)
			errs = append(errs, fmt.Errorf("uninstall %q: %w", dep, err))
			continue
		}
		env.Logger.Info("dependency removed", "dependency", dep)
	}
	return errors.Join(errs...)
}

func formatFailureSummary(failures map[string]error) string {
	if len(failures) == 0 {
		return ""
	}
	names := make([]string, 0, len(failures))
	for dep := range failures {
		names = append(names, dep)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, dep := range names {
		parts = append(parts, fmt.Sprintf("%s=%v", dep, failures[dep]))
	}
	return strings.Join(parts, ", ")
}
