package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	alloyBinaryPath = "../../../../../build/alloy"
)

type TestLog struct {
	TestDir    string
	AlloyLog   string
	TestOutput string
}

var logChan chan TestLog

func executeCommand(command string, args []string, taskDescription string) {
	fmt.Printf("%s...\n", taskDescription)
	cmd := exec.Command(command, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Error: %s\n", stderr.String())
	}
}

func buildAlloy() {
	executeCommand("make", []string{"-C", "../../..", "alloy"}, "Building Alloy")
}

func setupEnvironment() {
	executeCommand("docker-compose", []string{"up", "-d"}, "Setting up environment with Docker Compose")
	fmt.Println("Sleep for 45 seconds to ensure that the env has time to initialize...")
	time.Sleep(45 * time.Second)
}

func runSingleTest(testDir string, port int) {
	info, err := os.Stat(testDir)
	if err != nil {
		panic(err)
	}
	if !info.IsDir() {
		return
	}

	dirName := filepath.Base(testDir)

	var alloyLogBuffer bytes.Buffer
	cmd := exec.Command(alloyBinaryPath, "run", "config.alloy", "--server.http.listen-addr", fmt.Sprintf("0.0.0.0:%d", port), "--stability.level", "experimental")
	cmd.Dir = testDir
	cmd.Stdout = &alloyLogBuffer
	cmd.Stderr = &alloyLogBuffer

	if err := cmd.Start(); err != nil {
		logChan <- TestLog{
			TestDir:  dirName,
			AlloyLog: fmt.Sprintf("Failed to start Alloy: %v", err),
		}
		return
	}

	testCmd := exec.Command("go", "test")
	testCmd.Dir = testDir
	testOutput, errTest := testCmd.CombinedOutput()

	err = cmd.Process.Kill()
	if err != nil {
		panic(err)
	}

	alloyLog := alloyLogBuffer.String()

	if errTest != nil {
		logChan <- TestLog{
			TestDir:    dirName,
			AlloyLog:   alloyLog,
			TestOutput: string(testOutput),
		}
	}

	err = os.RemoveAll(filepath.Join(testDir, "data-alloy"))
	if err != nil {
		panic(err)
	}
}

func runAllTests() {
	testDirs, err := filepath.Glob("./tests/*")
	if err != nil {
		panic(err)
	}
	var wg sync.WaitGroup
	port := 12345
	for i, testDir := range testDirs {
		fmt.Println("Running", testDir)
		wg.Add(1)
		go func(td string, offset int) {
			defer wg.Done()
			runSingleTest(td, port+offset)
		}(testDir, i)
	}
	wg.Wait()
}

func cleanUpEnvironment() {
	fmt.Println("Cleaning up Docker environment...")
	err := exec.Command("docker-compose", "down", "--volumes", "--rmi", "all").Run()
	if err != nil {
		panic(err)
	}
}

func reportResults() {
	testsFailed := 0
	// It's ok to close the channel here because all tests are finished.
	// If the channel would not be closed, the for loop would wait forever.
	close(logChan)
	for log := range logChan {
		if strings.Contains(log.TestOutput, "build constraints exclude all Go files") {
			fmt.Printf("Test %q is not applicable for this OS, ignoring\n", log.TestDir)
			continue
		}
		fmt.Printf("Failure detected in %s:\n", log.TestDir)
		fmt.Println("Test output:", log.TestOutput)
		fmt.Println("Alloy logs:", log.AlloyLog)
		testsFailed++
	}

	if testsFailed > 0 {
		fmt.Printf("%d tests failed!\n", testsFailed)
		os.Exit(1)
	} else {
		fmt.Println("All integration tests passed!")
	}
}
