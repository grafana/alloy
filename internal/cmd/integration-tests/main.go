package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
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
	defer reportResults()

	testFolder := "./tests/"
	alloyBinaryPath := "../../../../../build/alloy"
	alloyBinary := "build/alloy"

	if runtime.GOOS == "windows" {
		testFolder = "./tests-windows/"
		alloyBinaryPath = "..\\..\\..\\..\\..\\build\\alloy.exe"
		alloyBinary = "build/alloy.exe"
	} else {
		setupEnvironment()
		defer cleanUpEnvironment()
	}

	if !skipBuild {
		buildAlloy(alloyBinary)
	}

	if specificTest != "" {
		fmt.Println("Running", specificTest)
		if !filepath.IsAbs(specificTest) && !strings.HasPrefix(specificTest, testFolder) {
			specificTest = testFolder + specificTest
		}
		logChan = make(chan TestLog, 1)
		runSingleTest(alloyBinaryPath, specificTest, 12345)
	} else {
		testDirs, err := filepath.Glob(testFolder + "*")
		if err != nil {
			panic(err)
		}
		logChan = make(chan TestLog, len(testDirs))
		runAllTests(alloyBinaryPath, testFolder)
	}
}
