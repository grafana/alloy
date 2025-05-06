package util

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/stretchr/testify/assert"
)

// TODO: Delete this?
func GetExecutaleDirectory() string {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return filepath.Dir(ex)
}

type CleanupFunc func()

func BootstrapTest(testDir, clusterName string) CleanupFunc {
	image := "grafana/alloy:latest"

	// Create a new cluster
	// K3d will raise an error if the cluster already exists.
	// We don't need to check it explicitly.
	ExecuteCommand(
		"k3d", []string{"cluster", "create", clusterName},
		fmt.Sprintf("Creating the `%s` cluster", clusterName))
	cleanupFunc := func() {
		ExecuteCommand(
			"k3d", []string{"cluster", "delete", clusterName},
			fmt.Sprintf("Deleting the `%s` cluster", clusterName))
	}

	// Load the Alloy image
	ExecuteCommand(
		"k3d", []string{"image", "import", "-c", clusterName, image},
		fmt.Sprintf("Loading the `%s` image in the `%s` cluster", image, clusterName))

	// Run a setup script. Install the operator
	setupScript := testDir + "setup.sh"
	ExecuteCommand(
		setupScript, []string{},
		fmt.Sprintf("Running the `%s`", setupScript))

	// Run the kuberenetes.yaml file from each folder in each cluster
	ExecuteCommand(
		"kubectl", []string{"apply", "-f", testDir + "kubernetes.yml"},
		"Applying the yaml manifest")

	return cleanupFunc
}

// TODO: Reuse the function from the other int tests
func ExecuteCommand(command string, args []string, taskDescription string) {
	fmt.Printf("----- %s...\n", taskDescription)
	cmd := exec.Command(command, args...)

	var stderr, stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		log.Printf("Output: %s\n", stdout.String())
		log.Fatalf("Error: %s\n", stderr.String())
	} else {
		log.Printf("Output: \n%s\n", stdout.String())
	}
}

func ExecuteBackgroundCommand(command string, args []string, taskDescription string) CleanupFunc {
	fmt.Printf("----- %s...\n", taskDescription)
	cmd := exec.Command(command, args...)

	// TODO: Get the stdout and stderr
	if err := cmd.Start(); err != nil {
		log.Fatalf("Error: %s\n", err)
	}

	return func() {
		if err := cmd.Process.Kill(); err != nil {
			log.Fatalf("Error: %s\n", err)
		}
	}
}

func Curl(c *assert.CollectT, url string) string {
	resp, err := http.Get(url)
	if err != nil {
		c.Errorf("Failed to make HTTP request: %v", err)
		c.FailNow()
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response body: %v", err)
	}

	return string(body)
}
