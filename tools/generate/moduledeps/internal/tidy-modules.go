package internal

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/grafana/alloy/tools/generate/moduledeps/internal/helpers"
	"github.com/grafana/alloy/tools/generate/moduledeps/internal/types"
)

func TidyModules(fileHelper *helpers.FileHelper, projectReplaces *types.ProjectReplaces) {
	for _, module := range projectReplaces.Modules {
		if module.FileType != types.FileTypeMod {
			continue
		}

		if err := runGoModTidy(fileHelper, module); err != nil {
			log.Fatalf("Failed to run go mod tidy for module %q: %v", module.Name, err)
		}
	}
}

func runGoModTidy(dirs *helpers.FileHelper, module types.Module) error {
	moduleDir, err := dirs.ModuleDir(module.Path)

	if err != nil {
		return err
	}

	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = moduleDir
	// Forward go's output so its diagnostics surface in the build log; without
	// this a failure only ever shows up as a bare "exit status 1".
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("Running go mod tidy in %s (module: %s)", moduleDir, module.Name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	return nil
}
