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
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gopkg.in/yaml.v3"

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
func createContainerRequest(dirName, testDir string, port int, networkName string, containerFiles []testcontainers.ContainerFile, cfg TestConfig) testcontainers.ContainerRequest {
	natPort, err := nat.NewPort("tcp", strconv.Itoa(port))
	if err != nil {
		panic(fmt.Sprintf("failed to build natPort: %v", err))
	}

	portBindings := make(nat.PortMap)
	defaultPortStr := fmt.Sprintf("%d/tcp", port)
	portBindings[nat.Port(defaultPortStr)] = []nat.PortBinding{
		{HostIP: "0.0.0.0", HostPort: strconv.Itoa(port)},
	}

	exposedPorts := []string{defaultPortStr}
	if len(cfg.Container.Ports) > 0 {
		for _, pm := range cfg.Container.Ports {
			exposedPorts = append(exposedPorts, fmt.Sprintf("%d/%s", pm.Container, pm.Protocol))
			portStr := fmt.Sprintf("%d/%s", pm.Container, pm.Protocol)
			portBindings[nat.Port(portStr)] = []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: strconv.Itoa(pm.Host)},
			}
		}
	}

	var mounts []mount.Mount
	if cfg.Container.UseMount {
		mountDir := filepath.Join(testDir, "mount")
		mountSrc, err := filepath.Abs(mountDir)
		if err != nil {
			panic(err)
		}
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   mountSrc,
			Target:   "/etc/alloy/mount",
			ReadOnly: true,
		})
	}

	req := testcontainers.ContainerRequest{
		Image:        alloyImageName,
		ExposedPorts: exposedPorts,
		WaitingFor:   wait.ForListeningPort(natPort),
		Cmd:          []string{"run", "/etc/alloy/config.alloy", "--server.http.listen-addr", fmt.Sprintf("0.0.0.0:%d", port), "--stability.level", "experimental"},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.PortBindings = portBindings
			hc.Mounts = append(hc.Mounts, mounts...)
			hc.Privileged = cfg.Container.Privileged
			hc.CapAdd = cfg.Container.CapAdd
			hc.SecurityOpt = cfg.Container.SecurityOpt
			hc.PidMode = container.PidMode(cfg.Container.PIDMode)
		},
		Files: containerFiles,
		Networks: []string{
			networkName,
		},
		NetworkAliases: map[string][]string{
			networkName: {"alloy-" + dirName},
		},
		Privileged: true,
	}

	return req
}

// Configure the test command with appropriate environment variables if needed
func setupTestCommand(testDir string, testTimeout time.Duration) *exec.Cmd {
	testCmd := exec.Command("go", "test", "-tags", "alloyintegrationtests")
	testCmd.Dir = testDir

	testCmd.Env = append(testCmd.Environ(), fmt.Sprintf("%s=%s", common.TestTimeout, testTimeout.String()))
	return testCmd
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

	var cfg TestConfig
	if _, err := os.Stat(filepath.Join(absTestDir, "test.yaml")); err == nil {
		file, err := os.Open(filepath.Join(absTestDir, "test.yaml"))
		if err != nil {
			panic(fmt.Sprintf("failed to read test.yaml: %v", err))
		}

		if err := yaml.NewDecoder(file).Decode(&cfg); err != nil {
			fmt.Println(testDir)
			panic(fmt.Sprintf("failed to descode test.yaml: %v", err))
		}
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

	if cfg.Container.UseMount {
		// Ensure mountDir exists
		mountDir := filepath.Join(absTestDir, "mount")
		if _, err := os.Stat(mountDir); os.IsNotExist(err) {
			if err := os.MkdirAll(mountDir, 0755); err != nil {
				panic(fmt.Sprintf("failed to create mount directory: %v\n", err))
			}
		}
	}

	// Create container request
	req := createContainerRequest(dirName, testDir, port, "alloy-integration-tests_integration-tests", containerFiles, cfg)

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

		if cfg.Container.UseMount {
			// Cleanup mount directory
			mountDir := filepath.Join(absTestDir, "mount")
			if err := os.RemoveAll(mountDir); err != nil {
				panic(fmt.Sprintf("failed to remove mount directory: %v\n", err))
			}
		}
	}()

	// Setup and run test command
	testCmd := setupTestCommand(testDir, testTimeout)
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
		if !info.IsDir() && !shouldExcludeFile(info.Name()) {
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

func shouldExcludeFile(name string) bool {
	if strings.HasSuffix(name, ".go") {
		return true
	}

	return filepath.Base(name) == "test.yaml"
}
