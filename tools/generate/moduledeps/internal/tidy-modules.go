package internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/grafana/dskit/backoff"

	"github.com/grafana/alloy/tools/generate/moduledeps/internal/helpers"
	"github.com/grafana/alloy/tools/generate/moduledeps/internal/types"
)

const tidyMaxRetries = 3

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

	var lastErr error
	// go mod tidy re-derives the whole module graph, so it hits the proxy far more
	// than a normal build and one transient failure aborts it. Retry with backoff.
	boff := backoff.New(context.Background(), backoff.Config{
		MinBackoff: 2 * time.Second,
		MaxBackoff: 15 * time.Second,
		MaxRetries: tidyMaxRetries,
	})
	for boff.Ongoing() {
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = moduleDir
		// Forward go's output so its diagnostics surface in the build log; without
		// this a failure only ever shows up as a bare "exit status 1".
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		// Pipe falls through to the next proxy on any error; the default comma
		// only falls through on 404/410. So a transient failure routes to direct.
		cmd.Env = append(os.Environ(), "GOPROXY=https://proxy.golang.org|direct")

		attempt := boff.NumRetries() + 1
		log.Printf("Running go mod tidy in %s (module: %s, attempt %d/%d)", moduleDir, module.Name, attempt, tidyMaxRetries)
		runErr := cmd.Run()
		if runErr == nil {
			return nil
		}

		// tidy is idempotent, so re-running after a transient failure is safe.
		lastErr = fmt.Errorf("go mod tidy failed: %w", runErr)
		log.Printf("go mod tidy failed for module %s (attempt %d/%d): %v", module.Name, attempt, tidyMaxRetries, lastErr)
		boff.Wait()
	}

	return lastErr
}
