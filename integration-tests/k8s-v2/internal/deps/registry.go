package deps

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
)

type Installer interface {
	Name() string
	Install(ctx context.Context, kubeconfig string) error
	Uninstall(ctx context.Context, kubeconfig string) error
}

type Registry struct {
	installers map[string]Installer
}

func NewRegistry(installerList ...Installer) Registry {
	installers := make(map[string]Installer, len(installerList))
	for _, installer := range installerList {
		installers[installer.Name()] = installer
	}
	return Registry{installers: installers}
}

func NewDefaultRegistry() Registry {
	return NewRegistry(NewMimirInstaller(), NewLokiInstaller())
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
			cfg.Logger.Info("installing dependency", "dependency", dep)
			if err := installer.Install(ctx, kubeconfig); err != nil {
				cfg.Logger.Warn("dependency install failed", "dependency", dep, "error", err)
				mu.Lock()
				failures[dep] = err
				mu.Unlock()
				return
			}
			cfg.Logger.Info("dependency ready", "dependency", dep)
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
		cfg.Logger.Info("rolling back partially installed dependencies", "installed", installed)
		if uninstallErr := r.Uninstall(ctx, kubeconfig, installed); uninstallErr != nil {
			return fmt.Errorf("dependency installs failed (%s) and rollback failed: %w", formatFailureSummary(failures), uninstallErr)
		}
	}
	return fmt.Errorf("dependency installs failed: %s", formatFailureSummary(failures))
}

func (r Registry) Uninstall(ctx context.Context, kubeconfig string, requirements []string) error {
	reversed := slices.Clone(requirements)
	slices.Reverse(reversed)
	for _, dep := range reversed {
		cfg.Logger.Info("uninstalling dependency", "dependency", dep)
		installer := r.installers[dep]
		if err := installer.Uninstall(ctx, kubeconfig); err != nil {
			return fmt.Errorf("uninstall %q: %w", dep, err)
		}
		cfg.Logger.Info("dependency removed", "dependency", dep)
	}
	return nil
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
