package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	tc "github.com/testcontainers/testcontainers-go/modules/compose"
)

var specificTest string
var skipBuild bool

func main() {
	rootCmd := &cobra.Command{
		Use:   "integration-tests",
		Short: "Run integration tests",
		Run:   runIntegrationTests,
	}

	rootCmd.PersistentFlags().StringVar(&specificTest, "test", "", "Specific test directory to run")
	rootCmd.PersistentFlags().BoolVar(&skipBuild, "skip-build", false, "Skip building Alloy")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runIntegrationTests(cmd *cobra.Command, args []string) {
	if !skipBuild {
		buildAlloy()
	}

	err := os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	if err != nil {
		fmt.Println("error setting environment variable:", err)
		return
	}

	compose, err := tc.NewDockerCompose("./docker-compose.yaml")
	if err != nil {
		panic(fmt.Errorf("failed to parse the docker compose file: %v", err))
	}

	ctx := context.Background()
	fmt.Println("Start test containers with docker compose config")
	err = compose.Up(ctx)
	if err != nil {
		panic(fmt.Errorf("could not start the docker compose: %v", err))
	}
	defer compose.Down(context.Background(), tc.RemoveImagesAll)

	fmt.Println("Sleep for 20 seconds to ensure that the env has time to initialize...")
	time.Sleep(20 * time.Second)

	if specificTest != "" {
		fmt.Println("Running", specificTest)
		if !filepath.IsAbs(specificTest) && !strings.HasPrefix(specificTest, "./tests/") {
			specificTest = "./tests/" + specificTest
		}
		logChan = make(chan TestLog, 1)
		runSingleTest(ctx, specificTest, 12345)
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
