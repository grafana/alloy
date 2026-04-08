package deps

import (
	"context"
	"fmt"
	"slices"
	"sort"
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
	for _, dep := range requirements {
		installer := r.installers[dep]
		if err := installer.Install(ctx, kubeconfig); err != nil {
			return fmt.Errorf("install %q: %w", dep, err)
		}
	}
	return nil
}

func (r Registry) Uninstall(ctx context.Context, kubeconfig string, requirements []string) error {
	reversed := slices.Clone(requirements)
	slices.Reverse(reversed)
	for _, dep := range reversed {
		installer := r.installers[dep]
		if err := installer.Uninstall(ctx, kubeconfig); err != nil {
			return fmt.Errorf("uninstall %q: %w", dep, err)
		}
	}
	return nil
}
