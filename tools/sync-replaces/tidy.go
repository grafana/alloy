package syncreplaces

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

const (
	tidyMaxAttempts = 3
	tidyMinBackoff  = 2 * time.Second
	tidyMaxBackoff  = 15 * time.Second
)

func runGoModTidy(moduleDir string) error {
	var lastErr error

	for attempt := 1; attempt <= tidyMaxAttempts; attempt++ {
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = moduleDir
		// Forward go's output so diagnostics surface in the build log.
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		// Pipe falls through to direct on any proxy error; comma only falls
		// through on 404/410, which doesn't cover transient proxy failures.
		cmd.Env = append(os.Environ(), "GOPROXY=https://proxy.golang.org|direct")

		log.Printf("Running go mod tidy in %s (attempt %d/%d)", moduleDir, attempt, tidyMaxAttempts)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			lastErr = fmt.Errorf("go mod tidy failed: %w", err)
			log.Printf("go mod tidy failed in %s (attempt %d/%d): %v", moduleDir, attempt, tidyMaxAttempts, lastErr)
		}

		if attempt < tidyMaxAttempts {
			time.Sleep(tidyBackoff(attempt))
		}
	}

	return lastErr
}

func tidyBackoff(attempt int) time.Duration {
	delay := tidyMinBackoff << (attempt - 1)
	if delay > tidyMaxBackoff {
		return tidyMaxBackoff
	}
	return delay
}
