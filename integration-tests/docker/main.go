package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

var (
	specificTest    string
	skipBuild       bool
	stateful        bool
	testTimeout     time.Duration
	alwaysPrintLogs bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "integration-tests",
		Short: "Run integration tests",
		Run:   runIntegrationTests,
	}

	rootCmd.PersistentFlags().StringVar(&specificTest, "test", "", "Specific test directory to run")
	rootCmd.PersistentFlags().BoolVar(&skipBuild, "skip-build", false, "Skip building Alloy")
	statefulUsageString := "Run the tests in a stateful manner. " +
		"The docker compose setup will not be torn down after the tests complete. " +
		"Any queries will be run with a start time set to the alloy container start time. " +
		"This is useful for a fast iteration loop locally but should not be used in CI." +
		"You must run 'docker compose down' manually if you want to switch from stateful to stateless mode."
	rootCmd.PersistentFlags().BoolVar(&stateful, "stateful", false, statefulUsageString)
	rootCmd.PersistentFlags().DurationVar(&testTimeout, "test-timeout", common.DefaultTimeout, "Timeout for each individual test")
	rootCmd.PersistentFlags().BoolVar(&alwaysPrintLogs, "always-print-logs", false, "Always print the test and alloy logs, even if the test passed")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runIntegrationTests(cmd *cobra.Command, args []string) {
	fmt.Printf("Running integration tests (stateful=%v, skip-build=%v, specific-test=%s)\n", stateful, skipBuild, specificTest)

	ctx := cmd.Context()
	if !skipBuild {
		buildAlloy()
	}

	executeCommand("docker", []string{"compose", "up", "-d"}, "Starting dependent services with docker compose")
	if !stateful {
		defer executeCommand("docker", []string{"compose", "down"}, "Stopping dependent services")
		fmt.Println("Sleep for 10 seconds to ensure that the env has time to initialize...")
		time.Sleep(10 * time.Second)
	} else {
		// This has been the observed set of services that are required to be healthy for the tests to run. We cannot
		// wait for all services as we have an init container that is expected to exit.
		// After all services get a healthcheck we can use this 100% of the time instead of the hardcoded "wait 10 seconds".
		executeCommand("docker", []string{"compose", "up", "kafka", "loki", "--wait"}, "Waiting for necessary compose services to be healthy")
	}

	if specificTest != "" {
		fmt.Println("Running", specificTest)
		if !filepath.IsAbs(specificTest) && !strings.HasPrefix(specificTest, "./tests/") {
			specificTest = "./tests/" + specificTest
		}
		runSingleTest(ctx, specificTest, 12345, stateful, testTimeout)
	} else {
		runAllTests(ctx)
	}
	failedTests := reportResults(alwaysPrintLogs)
	if failedTests > 0 {
		log.Fatalf("%d tests failed. See logs for failure", failedTests)
	}
	fmt.Println("All integration tests passed!")
}
