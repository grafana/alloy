package main

import (
	"bytes"
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

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	alloyBinaryPath = "../../../../../build/alloy"
	alloyImageName  = "alloy-integration-tests"
)

type TestLog struct {
	TestDir    string
	AlloyLog   string
	TestOutput string
}

type fileInfo struct {
	path    string
	relPath string
}

var logChan chan TestLog

func executeCommand(command string, args []string, taskDescription string) {
	fmt.Printf("%s...\n", taskDescription)

	cmd := exec.Command(command, args...)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		log.Fatalf("error executing %s: %v\nstdout: %s\nstderr: %s", taskDescription, err, outBuf.String(), errBuf.String())
	}
}

func buildAlloy() {
	executeCommand("make", []string{"-C", "../../..", "ALLOY_IMAGE=" + alloyImageName, "alloy-image"}, "Building Alloy")
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
func setupTestCommand(ctx context.Context, dirName string, testDir string, alloyContainer testcontainers.Container) (*exec.Cmd, error) {
	testCmd := exec.Command("go", "test")
	testCmd.Dir = testDir

	if dirName == "loki-enrich" {
		mappedPort, err := alloyContainer.MappedPort(ctx, "1514/tcp")
		if err != nil {
			return nil, fmt.Errorf("failed to get mapped port: %v", err)
		}

		host, err := alloyContainer.Host(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get container host: %v", err)
		}

		fmt.Printf("Loki API should be available at http://%s:%s/loki/api/v1/push\n",
			host, mappedPort.Port())

		// TODO: we shouldn't have this logic here, this is needed for the loki-enrich test
		// to work, but we should find a better way to pass the host and port
		testCmd.Env = append(os.Environ(),
			fmt.Sprintf("ALLOY_HOST=%s", host),
			fmt.Sprintf("ALLOY_PORT=%s", mappedPort.Port()))
	}

	return testCmd, nil
}

func runSingleTest(ctx context.Context, testDir string, port int) {
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
	alloyContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Logger:           log.Default(),
	})
	if err != nil {
		logChan <- TestLog{
			TestDir:  dirName,
			AlloyLog: fmt.Sprintf("failed to start Alloy container: %v", err),
		}
		return
	}
	defer alloyContainer.Terminate(ctx)

	// Setup and run test command
	testCmd, err := setupTestCommand(ctx, dirName, testDir, alloyContainer)
	if err != nil {
		logChan <- TestLog{
			TestDir:  dirName,
			AlloyLog: fmt.Sprintf("%v", err),
		}
		return
	}

	testOutput, errTest := testCmd.CombinedOutput()

	// Collect and report logs if test failed
	alloyLogs, _ := alloyContainer.Logs(ctx)
	alloyLog, _ := io.ReadAll(alloyLogs)

	if errTest != nil {
		logChan <- TestLog{
			TestDir:    dirName,
			AlloyLog:   string(alloyLog),
			TestOutput: string(testOutput),
		}
	}
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
			runSingleTest(ctx, td, port+offset)
		}(testDir, i)
	}
	wg.Wait()
}

func reportResults() {
	testsFailed := 0
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
	} else {
		fmt.Println("All integration tests passed!")
	}
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
