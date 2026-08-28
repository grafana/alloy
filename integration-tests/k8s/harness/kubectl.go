package harness

import (
	"fmt"
	"time"
)

const (
	readyTimeout        = 5 * time.Minute // overall WaitForReady budget
	readyAttemptTimeout = "15s"           // per kubectl wait call
	readyPollInterval   = 1 * time.Second // gap between retries
)

// ApplyManifest pipes manifest into `kubectl apply -f -`. Empty namespace
// for cluster-scoped manifests or those declaring their own.
func ApplyManifest(namespace, manifest string) error {
	args := []string{"apply"}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	args = append(args, "-f", "-")
	return RunCommandStdin(manifest, "kubectl", args...)
}

// DeleteManifest mirrors ApplyManifest for `kubectl delete`. Always passes
// --ignore-not-found, --wait and --timeout=10m so cleanup is idempotent.
func DeleteManifest(namespace, manifest string) error {
	args := []string{"delete"}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	args = append(args, "-f", "-",
		"--ignore-not-found=true", "--wait=true", "--timeout=10m",
	)
	return RunCommandStdin(manifest, "kubectl", args...)
}

// WaitForReady blocks until every pod matching selector in namespace is
// Ready, or readyTimeout elapses. The retry loop tolerates "no matching
// resources found" so it's safe to call right after kubectl apply.
func WaitForReady(namespace, selector string) error {
	deadline := time.Now().Add(readyTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		err := RunCommand("kubectl",
			"--namespace", namespace,
			"wait", "--for=condition=ready", "pod",
			"-l", selector,
			"--timeout="+readyAttemptTimeout,
		)
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(readyPollInterval)
	}
	return fmt.Errorf("timed out after %s waiting for pods ready namespace=%s selector=%s: %w", readyTimeout, namespace, selector, lastErr)
}
