package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
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

	start := time.Now()
	executeCommand("docker", []string{"compose", "up", "-d"}, "Starting dependent services with docker compose")
	waitArgs := []string{"compose", "up", "-d", "--wait", "mimir", "tempo", "kafka", "loki", "redis"}
	executeCommand("docker", waitArgs, "Waiting for dependent services to be healthy")
	waitForHTTPReady("http://localhost:9009/ready", 3*time.Minute)
	fmt.Printf("Environment setup completed in %s\n", time.Since(start))
	if !stateful {
		defer executeCommand("docker", []string{"compose", "down"}, "Stopping dependent services")
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

func waitForHTTPReady(url string, timeout time.Duration) {
	fmt.Printf("Waiting for %s to be ready...\n", url)
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 5 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return
			}
		}
		time.Sleep(2 * time.Second)
	}

	log.Fatalf("Timed out waiting for %s to be ready", url)
}
