package util

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
)

type CleanupFunc func()

// Running the test in a stateful way means that k8s resources won't be deleted at the end.
// It's useful for debugging. You can run the test like this:
// ALLOY_STATEFUL_K8S_TEST=true make integration-test-k8s
//
// After you're done with the test, run a command like this to clean up:
// minikube delete -p prometheus-operator
func isStateful() bool {
	stateful, _ := strconv.ParseBool(os.Getenv(envVarStateful))
	return stateful
}

// clusterName will be prefixed with "alloy-int-test-" before the cluster is created.
func BootstrapTest(testDir, clusterName string) CleanupFunc {
	image := "grafana/alloy:latest"

	clusterName = "alloy-int-test-" + clusterName

	// Create a new cluster
	// Minikube will raise an error if the cluster already exists.
	// We don't need to check it explicitly.
	ExecuteCommand(
		"minikube", []string{"start", "-p", clusterName},
		fmt.Sprintf("Creating the `%s` cluster", clusterName))

	var cleanupFunc CleanupFunc
	if isStateful() {
		// In stateful mode, don't delete the cluster to allow for fast iteration
		cleanupFunc = func() {
			fmt.Printf("Stateful mode enabled: Skipping cluster deletion for `%s`\n", clusterName)
		}
	} else {
		cleanupFunc = func() {
			ExecuteCommand(
				"minikube", []string{"delete", "-p", clusterName},
				fmt.Sprintf("Deleting the `%s` cluster", clusterName))
		}
	}

	// Load the Alloy image
	ExecuteCommand(
		"minikube", []string{"image", "load", image, "-p", clusterName},
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
	fmt.Printf("Executing: %s %v\n", command, args)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Printf("Command finished with error: %v", err)
	}
}

func ExecuteBackgroundCommand(command string, args []string, taskDescription string) CleanupFunc {
	fmt.Printf("----- %s...\n", taskDescription)
	cmd := exec.Command(command, args...)
	fmt.Printf("Executing: %s %v\n", command, args)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Fatalf("Error: %s\n", err)
	}

	return func() {
		if err := cmd.Process.Kill(); err != nil {
			log.Fatalf("Error: %s\n", err)
		}
	}
}
