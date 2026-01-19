package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

const alloyImageName = "alloy-integration-tests"

type TestLog struct {
	TestDir    string
	IsError    bool
	AlloyLog   string
	TestOutput string
}

type fileInfo struct {
	path    string
	relPath string
}

func executeCommand(command string, args []string, taskDescription string) {
	fmt.Printf("%s...\n", taskDescription)

	cmd := exec.Command(command, args...)

	// Assign os.Stdout and os.Stderr to the command's output streams
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("error executing %s: %v", taskDescription, err)
	}
}

func buildAlloy() {
	executeCommand("make", []string{"-C", "../../", "ALLOY_IMAGE=" + alloyImageName, "alloy-image"}, "Building Alloy")
}

// Setup container files for mounting into the test container
func prepareContainerFiles(absTestDir string) ([]testcontainers.ContainerFile, []*os.File, error) {
	files, err := collectFiles(absTestDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to collect config files: %v", err)
	}

	var containerFiles []testcontainers.ContainerFile
	var openFiles []*os.File

	for _, fileToAdd := range files {
		f, err := os.Open(fileToAdd.path)
		if err != nil {
			// Close files opened so far
			for _, openFile := range openFiles {
				openFile.Close()
			}
			return nil, nil, fmt.Errorf("failed to open file %s: %v", fileToAdd.path, err)
		}
		openFiles = append(openFiles, f)

		containerFiles = append(containerFiles, testcontainers.ContainerFile{
			Reader:            f,
			ContainerFilePath: filepath.Join("/etc/alloy", fileToAdd.relPath),
			FileMode:          0o700,
		})
	}

	return containerFiles, openFiles, nil
}

// Create a container request based on the test directory
func createContainerRequest(dirName string, port int, networkName string, containerFiles []testcontainers.ContainerFile) testcontainers.ContainerRequest {
	natPort, err := nat.NewPort("tcp", strconv.Itoa(port))
	if err != nil {
		panic(fmt.Sprintf("failed to build natPort: %v", err))
	}

	req := testcontainers.ContainerRequest{
		Image:        alloyImageName,
		ExposedPorts: []string{fmt.Sprintf("%d/tcp", port)},
		WaitingFor:   wait.ForListeningPort(natPort),
		Cmd:          []string{"run", "/etc/alloy/config.alloy", "--server.http.listen-addr", fmt.Sprintf("0.0.0.0:%d", port), "--stability.level", "experimental"},
		Files:        containerFiles,
		Networks: []string{
			networkName,
		},
		NetworkAliases: map[string][]string{
			networkName: {"alloy-" + dirName},
		},
		Privileged: true,
	}

	// Apply special configurations for specific tests
	if dirName == "beyla" {
		req.HostConfigModifier = func(hostConfig *container.HostConfig) {
			hostConfig.Privileged = true
			hostConfig.CapAdd = []string{"SYS_ADMIN", "SYS_PTRACE", "SYS_RESOURCE"}
			hostConfig.SecurityOpt = []string{"apparmor:unconfined"}
			hostConfig.PidMode = container.PidMode("host")
		}
	}

	if dirName == "loki-enrich" {
		req.ExposedPorts = append(req.ExposedPorts, "1514/tcp")
	}

	return req
}

// Configure the test command with appropriate environment variables if needed
func setupTestCommand(ctx context.Context, dirName string, testDir string, alloyContainer testcontainers.Container, testTimeout time.Duration) (*exec.Cmd, error) {
	testCmd := exec.Command("go", "test", "-tags", "integration")
	testCmd.Dir = testDir

	testCmd.Env = append(testCmd.Environ(), fmt.Sprintf("%s=%s", common.TestTimeout, testTimeout.String()))

	if dirName == "loki-enrich" {
		mappedPort, err := alloyContainer.MappedPort(ctx, "1514/tcp")
		if err != nil {
			return nil, fmt.Errorf("failed to get mapped port: %v", err)
		}

		host, err := alloyContainer.Host(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get container host: %v", err)
		}

		testCmd.Env = append(testCmd.Environ(),
			fmt.Sprintf("%s=%s", common.AlloyHostEnv, host),
			fmt.Sprintf("%s=%s", common.AlloyPortEnv, mappedPort.Port()))
	}

	return testCmd, nil
}

var logMux sync.Mutex
var logs []TestLog

func runSingleTest(ctx context.Context, testDir string, port int, stateful bool, testTimeout time.Duration) {
	info, err := os.Stat(testDir)
	if err != nil {
		panic(err)
	}
	if !info.IsDir() {
		return
	}

	dirName := filepath.Base(testDir)

	absTestDir, err := filepath.Abs(testDir)
	if err != nil {
		panic(fmt.Sprintf("failed to get absolute path of testDir: %v", err))
	}

	// Prepare container files
	containerFiles, openFiles, err := prepareContainerFiles(absTestDir)
	if err != nil {
		panic(err)
	}
	defer func() {
		for _, f := range openFiles {
			f.Close()
		}
	}()

	// Create container request
	req := createContainerRequest(dirName, port, "alloy-integration-tests_integration-tests", containerFiles)

	// Start container
	containerStartTime := time.Now()
	alloyContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Logger:           log.Default(),
	})
	if err != nil {
		addLog(TestLog{
			TestDir:  dirName,
			AlloyLog: fmt.Sprintf("failed to start Alloy container: %v", err),
			IsError:  true,
		})
		return
	}

	defer func() {
		if err := alloyContainer.Terminate(ctx); err != nil {
			addLog(TestLog{
				TestDir:  dirName,
				AlloyLog: fmt.Sprintf("failed to terminate Alloy container: %v", err),
				IsError:  true,
			})
		}
	}()

	// Setup and run test command
	testCmd, err := setupTestCommand(ctx, dirName, testDir, alloyContainer, testTimeout)
	if err != nil {
		addLog(TestLog{
			TestDir:  dirName,
			AlloyLog: fmt.Sprintf("failed to setup test command: %v", err),
			IsError:  true,
		})
		return
	}
	if stateful {
		testCmd.Env = append(testCmd.Environ(),
			fmt.Sprintf("%s=%d", common.AlloyStartTimeEnv, containerStartTime.Unix()),
			fmt.Sprintf("%s=true", common.TestStatefulEnv))
	}

	testOutput, errTest := testCmd.CombinedOutput()

	// Collect and report logs
	alloyLogs, _ := alloyContainer.Logs(ctx)
	alloyLog, _ := io.ReadAll(alloyLogs)
	testLogs := TestLog{
		TestDir:    dirName,
		AlloyLog:   string(alloyLog),
		TestOutput: string(testOutput),
	}

	if errTest != nil {
		testLogs.IsError = true
	}
	addLog(testLogs)
}

func runAllTests(ctx context.Context) {
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
			runSingleTest(ctx, td, port+offset, stateful, testTimeout)
		}(testDir, i)
	}
	wg.Wait()
}

func addLog(testLog TestLog) {
	logMux.Lock()
	defer logMux.Unlock()
	logs = append(logs, testLog)
}

// reportResults prints the results of the tests and returns the number of failed tests
func reportResults(alwaysPrintLogs bool) int {
	dirsWithFailure := map[string]struct{}{}
	for _, log := range logs {
		if strings.Contains(log.TestOutput, "build constraints exclude all Go files") {
			fmt.Printf("Test %q is not applicable for this OS, ignoring\n", log.TestDir)
			continue
		}
		if log.IsError {
			fmt.Printf("Failure detected in %s:\n", log.TestDir)
			dirsWithFailure[log.TestDir] = struct{}{}
		} else if alwaysPrintLogs {
			fmt.Printf("Tests in %s were successful:\n", log.TestDir)
		} else {
			continue
		}
		fmt.Println("Test output:", log.TestOutput)
		fmt.Println("Alloy logs:", log.AlloyLog)
	}

	return len(dirsWithFailure)
}

func collectFiles(root string) ([]fileInfo, error) {
	var filesToAdd []fileInfo

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.HasSuffix(info.Name(), ".go") {
			relPath, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			filesToAdd = append(filesToAdd, fileInfo{
				path:    path,
				relPath: relPath,
			})
		}
		return nil
	})
	return filesToAdd, err
}
