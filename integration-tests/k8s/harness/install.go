package harness

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = commandEnv()
	return cmd.Run()
}

func runCommandOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = commandEnv()
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%s %v failed: %w: %s", name, args, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}
