package harness

import "fmt"

// Kubectl runs kubectl with the managed test kubeconfig.
func Kubectl(args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("kubectl requires at least one argument")
	}
	return runCommand("kubectl", args...)
}
