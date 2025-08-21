package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	specificTest string
	skipBuild    bool
	stateful     bool
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
	}
	executeCommand("docker", []string{"compose", "up", "kafka", "loki", "--wait"}, "Waiting for necessary compose services to be healthy")

	if specificTest != "" {
		fmt.Println("Running", specificTest)
		if !filepath.IsAbs(specificTest) && !strings.HasPrefix(specificTest, "./tests/") {
			specificTest = "./tests/" + specificTest
		}
		logChan = make(chan TestLog, 1)
		runSingleTest(ctx, specificTest, 12345, stateful)
	} else {
		testDirs, err := filepath.Glob("./tests/*")
		if err != nil {
			panic(err)
		}
		logChan = make(chan TestLog, len(testDirs))
		runAllTests(ctx)
	}
	reportResults()
}
