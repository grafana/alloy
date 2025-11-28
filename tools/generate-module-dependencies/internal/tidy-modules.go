package internal

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/grafana/replace-generator/internal/helpers"
	"github.com/grafana/replace-generator/internal/types"
)

func TidyModules(fileHelper *helpers.FileHelper, projectReplaces *types.ProjectReplaces) {
	for _, module := range projectReplaces.Modules {
		if err := runGoModTidy(fileHelper, module); err != nil {
			log.Fatalf("Failed to run go mod tidy for module %q: %v", module.Name, err)
		}
	}
}

func runGoModTidy(dirs *helpers.FileHelper, module types.Module) error {
	moduleDir := dirs.ModuleDir(module.Path)

	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = moduleDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("Running go mod tidy in %s (module: %s)", moduleDir, module.Name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	return nil
}
