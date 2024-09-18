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
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
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

	files, err := collectFiles(absTestDir)
	if err != nil {
		panic(fmt.Sprintf("failed to collect config files: %v", err))
	}

	natPort, err := nat.NewPort("tcp", strconv.Itoa(port))
	if err != nil {
		panic(fmt.Sprintf("failed to build natPort: %v", err))
	}

	// The network is created via the docker-compose file but a randomly generated prefix is added.
	// We retrieve the name to add the Alloy test containers in the same network.
	networkName, err := getTestcontainersNetworkName(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to get Testcontainers network name: %v", err))
	}

	var containerFiles []testcontainers.ContainerFile
	var openFiles []*os.File

	for _, fileToAdd := range files {
		f, err := os.Open(fileToAdd.path)
		if err != nil {
			panic(fmt.Sprintf("failed to open file %s: %v", fileToAdd.path, err))
		}
		openFiles = append(openFiles, f)

		containerFiles = append(containerFiles, testcontainers.ContainerFile{
			Reader:            f,
			ContainerFilePath: filepath.Join("/etc/alloy", fileToAdd.relPath),
			FileMode:          0o700,
		})
	}

	defer func() {
		for _, f := range openFiles {
			f.Close()
		}
	}()

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

	if dirName == "beyla" {
		containerID, err := findContainerID(ctx, "otel-metrics-gen-1")
		if err != nil {
			panic(err)
		}
		req.HostConfigModifier = func(hostConfig *container.HostConfig) {
			hostConfig.PidMode = container.PidMode("container:" + containerID)
		}
	}

	alloyContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		logChan <- TestLog{
			TestDir:  dirName,
			AlloyLog: fmt.Sprintf("failed to start Alloy container: %v", err),
		}
		return
	}
	defer alloyContainer.Terminate(ctx)

	testCmd := exec.Command("go", "test")
	testCmd.Dir = testDir
	testOutput, errTest := testCmd.CombinedOutput()

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
		os.Exit(1)
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

func findContainerID(ctx context.Context, targetName string) (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}

	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return "", err
	}

	for _, c := range containers {
		for _, name := range c.Names {
			if strings.HasSuffix(name, targetName) {
				return c.ID, nil
			}
		}
	}

	return "", fmt.Errorf("container with name suffix %s not found", targetName)
}

func getTestcontainersNetworkName(ctx context.Context) (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", fmt.Errorf("failed to create Docker client: %v", err)
	}
	defer cli.Close()

	networks, err := cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list networks: %v", err)
	}

	for _, network := range networks {
		if strings.HasSuffix(network.Name, "_integration-tests") {
			return network.Name, nil
		}
	}

	return "", fmt.Errorf("could not find Testcontainers network")
}
