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
	"github.com/grafana/alloy/integration-tests/docker/common"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const alloyImageName = "alloy-integration-tests"
const dockerComposeFile = "docker-compose.yaml"

type TestLog struct {
	TestDir     string
	IsError     bool
	AlloyLog    string //TODO: Rename this to not have "Alloy" in the name
	TestOutput  string
	TetragonLog string
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
	// Build without stripping symbols (RELEASE_BUILD=0 omits -s -w ldflags)
	// so that Tetragon can resolve user stack-frame addresses to function names
	// via the ELF symbol table, and go tool addr2line can resolve offsets too.
	executeCommand("make", []string{"-C", "../../", "ALLOY_IMAGE=" + alloyImageName, "RELEASE_BUILD=0", "alloy-image"}, "Building Alloy")
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
func createContainerRequest(dirName, testDir string, port int, networkName string, containerFiles []testcontainers.ContainerFile, cfg common.TestConfig) testcontainers.ContainerRequest {
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
		Entrypoint: []string{
			"/bin/alloy", "run", "/etc/alloy/config.alloy",
			fmt.Sprintf("--server.http.listen-addr=0.0.0.0:%d", port),
			"--stability.level=experimental",
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.PortBindings = portBindings
			hc.Mounts = append(hc.Mounts, mounts...)
			if cfg.Container.Capabilities != nil {
				// Explicit capability list: drop all defaults then add only the
				// listed ones. An empty slice gives the container no capabilities.
				hc.Privileged = false
				hc.CapDrop = []string{"ALL"}
				hc.CapAdd = *cfg.Container.Capabilities
			} else {
				hc.Privileged = cfg.Container.Privileged
				hc.CapAdd = cfg.Container.CapAdd
			}
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

// runTestWithTestcontainers runs a test using testcontainers to create the Alloy container.
// This is used for tests that don't have their own docker-compose.yaml.
func runTestWithTestcontainers(ctx context.Context, testDir string, port int, stateful bool, testTimeout time.Duration) {
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

	cfg, err := common.LoadTestConfig(filepath.Join(absTestDir, "test.yaml"))
	if err != nil {
		panic(fmt.Sprintf("failed to load test.yaml for %s: %v", dirName, err))
	}

	// Self-managed tests handle all container lifecycle themselves (e.g. tests
	// that need multiple containers or a specific start order). Just run the binary.
	if cfg.Container.SelfManaged {
		testOutput, errTest := setupTestCommand(testDir, testTimeout).CombinedOutput()
		testLogs := TestLog{
			TestDir:    dirName,
			TestOutput: string(testOutput),
		}
		if errTest != nil {
			testLogs.IsError = true
		}
		addLog(testLogs)
		return
	}

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

	// Start Tetragon before Alloy so its eBPF probes are armed before
	// Alloy's first syscall.
	var tetragonContainer testcontainers.Container
	if cfg.Container.TetragonImage != "" {
		c, err := common.StartTetragonContainer(ctx, cfg.Container.TetragonImage)
		if err != nil {
			addLog(TestLog{
				TestDir:  dirName,
				AlloyLog: fmt.Sprintf("failed to start Tetragon container: %v", err),
				IsError:  true,
			})
			return
		}
		tetragonContainer = c
	}

	if tetragonContainer != nil {
		time.Sleep(common.TetragonProbeInitDelay)
	}

	defer func() {
		if tetragonContainer != nil {
			if err := tetragonContainer.Terminate(ctx); err != nil {
				addLog(TestLog{
					TestDir:  dirName,
					AlloyLog: fmt.Sprintf("failed to terminate Tetragon container: %v", err),
					IsError:  true,
				})
			}
		}
	}()

	// Start container
	alloyContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Logger:           log.Default(),
	})
	if err != nil {
		addLog(TestLog{
			TestDir:  dirName,
			AlloyLog: fmt.Sprintf("failed to start container: %v", err),
			IsError:  true,
		})
		return
	}

	defer func() {
		if !stateful {
			if err := alloyContainer.Terminate(ctx); err != nil {
				addLog(TestLog{
					TestDir:  dirName,
					AlloyLog: fmt.Sprintf("failed to terminate container: %v", err),
					IsError:  true,
				})
			}
			if cfg.Container.UseMount {
				mountDir := filepath.Join(absTestDir, "mount")
				if err := os.RemoveAll(mountDir); err != nil {
					panic(fmt.Sprintf("failed to remove mount directory: %v\n", err))
				}
			}
		}
	}()

	// Setup and run test command
	containerStartTime := time.Now()
	testCmd := setupTestCommand(testDir, testTimeout)
	if stateful {
		testCmd.Env = append(testCmd.Env,
			fmt.Sprintf("%s=%d", common.AlloyStartTimeEnv, containerStartTime.Unix()),
			fmt.Sprintf("%s=true", common.TestStatefulEnv))
	}
	if tetragonContainer != nil {
		testCmd.Env = append(testCmd.Env,
			fmt.Sprintf("%s=%s", common.TetragonContainerIDEnv, tetragonContainer.GetContainerID()))
	}
	testOutput, errTest := testCmd.CombinedOutput()

	// Collect and report logs
	var tetragonLog string
	if tetragonContainer != nil {
		saveRawTetragonLogs(ctx, tetragonContainer, dirName)
		var err error
		tetragonLog, err = common.FormatTetragonLogs(ctx, tetragonContainer, alloyContainer)
		if err != nil {
			tetragonLog = fmt.Sprintf("failed to collect tetragon logs: %v", err)
		}
	}

	alloyLogs, _ := alloyContainer.Logs(ctx)
	logBytes, _ := io.ReadAll(alloyLogs)
	testLogs := TestLog{
		TestDir:     dirName,
		AlloyLog:    string(logBytes),
		TestOutput:  string(testOutput),
		TetragonLog: tetragonLog,
	}

	if errTest != nil {
		testLogs.IsError = true
	}
	addLog(testLogs)
}

// hasComposeFile returns true if the test directory contains a docker-compose.yaml file.
func hasComposeFile(testDir string) bool {
	composeFile := filepath.Join(testDir, dockerComposeFile)
	_, err := os.Stat(composeFile)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	panic(fmt.Sprintf("failed to stat compose file %q: %v", composeFile, err))
}

// runTest runs a single test, automatically detecting whether to use docker-compose
// or testcontainers based on the presence of a docker-compose.yaml file.
func runTest(ctx context.Context, testDir string, port int, stateful bool, testTimeout time.Duration) {
	if hasComposeFile(testDir) {
		runComposeTest(ctx, testDir, stateful, testTimeout)
	} else {
		runTestWithTestcontainers(ctx, testDir, port, stateful, testTimeout)
	}
}

func runAllTests(ctx context.Context, testTimeout time.Duration) {
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
			runTest(ctx, td, port+offset, stateful, testTimeout)
		}(testDir, i)
	}
	wg.Wait()
}

// runComposeTest runs a test that has its own docker-compose.yaml defining
// test infrastructure (Alloy, data generators, etc.).
func runComposeTest(ctx context.Context, testDir string, stateful bool, testTimeout time.Duration) {
	dirName := filepath.Base(testDir)
	absTestDir, err := filepath.Abs(testDir)
	if err != nil {
		panic(fmt.Sprintf("failed to get absolute path of testDir: %v", err))
	}

	composeFile := filepath.Join(absTestDir, dockerComposeFile)
	projectName := "test-" + dirName

	// Ensure cleanup happens
	defer func() {
		fmt.Printf("Stopping compose services for %s...\n", dirName)
		downCmd := exec.Command("docker", "compose", "-f", composeFile, "-p", projectName, "down")
		downCmd.Dir = absTestDir
		if output, err := downCmd.CombinedOutput(); err != nil {
			fmt.Printf("Warning: failed to stop compose services for %s: %v\nOutput: %s\n", dirName, err, string(output))
		}
	}()

	// Start test-specific services
	fmt.Printf("Starting compose services for %s...\n", dirName)
	upCmd := exec.Command("docker", "compose", "-f", composeFile, "-p", projectName, "up", "-d", "--build", "--wait")
	upCmd.Dir = absTestDir
	upOutput, err := upCmd.CombinedOutput()
	if err != nil {
		addLog(TestLog{
			TestDir:  dirName,
			AlloyLog: fmt.Sprintf("failed to start compose services: %v\nOutput: %s", err, string(upOutput)),
			IsError:  true,
		})
		return
	}

	// Create a context with timeout to enforce test duration limit
	testCtx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()

	// Setup and run test command with context for timeout enforcement
	testCmd := exec.CommandContext(testCtx, "go", "test", "-tags", "alloyintegrationtests")
	testCmd.Dir = testDir
	testCmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", common.TestTimeout, testTimeout.String()))
	if stateful {
		testCmd.Env = append(testCmd.Env, fmt.Sprintf("%s=true", common.TestStatefulEnv))
	}

	testOutput, errTest := testCmd.CombinedOutput()

	// Collect logs from the alloy container
	logsCmd := exec.Command("docker", "compose", "-f", composeFile, "-p", projectName, "logs", "alloy")
	logsCmd.Dir = absTestDir
	alloyLogOutput, _ := logsCmd.CombinedOutput()

	testLogs := TestLog{
		TestDir:    dirName,
		AlloyLog:   string(alloyLogOutput),
		TestOutput: string(testOutput),
	}

	if errTest != nil {
		testLogs.IsError = true
		// Check if the error was due to context timeout
		if testCtx.Err() == context.DeadlineExceeded {
			testLogs.TestOutput = fmt.Sprintf("TEST TIMEOUT: test exceeded %v limit\n%s", testTimeout, testOutput)
		}
	}
	addLog(testLogs)
}

// saveRawTetragonLogs writes the raw NDJSON output from the Tetragon container
// to a file named "tetragon-<testDir>.log" in the current working directory so
// it can be inspected after the test run (e.g. with "tetra getevents -o compact").
func saveRawTetragonLogs(ctx context.Context, c testcontainers.Container, testDir string) {
	logs, err := c.Logs(ctx)
	if err != nil {
		fmt.Printf("warning: could not read Tetragon logs for %s: %v\n", testDir, err)
		return
	}
	defer logs.Close()

	logBytes, err := io.ReadAll(logs)
	if err != nil {
		fmt.Printf("warning: could not read Tetragon logs for %s: %v\n", testDir, err)
		return
	}

	filename := fmt.Sprintf("tetragon-%s.log", testDir)
	if err := os.WriteFile(filename, logBytes, 0o644); err != nil {
		fmt.Printf("warning: could not write Tetragon log file %s: %v\n", filename, err)
		return
	}
	fmt.Printf("Tetragon raw logs written to %s\n", filename)
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
			fmt.Printf("\n=== Failure detected in %s ===\n", log.TestDir)
			dirsWithFailure[log.TestDir] = struct{}{}
		} else if alwaysPrintLogs {
			fmt.Printf("Tests in %s were successful:\n", log.TestDir)
		} else {
			continue
		}
		fmt.Println("\n--- Test output ---")
		fmt.Println(log.TestOutput)
		fmt.Println("\n--- Container logs ---")
		fmt.Println(log.AlloyLog)
		if log.TetragonLog != "" {
			fmt.Println("\n--- Tetragon capability events ---")
			fmt.Println(log.TetragonLog)
		}
		if log.IsError {
			fmt.Println("\nTip: run with --stateful to leave the Alloy container running and inspect it (docker exec -it <id> sh), or --always-print-logs to always see full logs and BCC output.")
		}
		fmt.Println()
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

	base := filepath.Base(name)
	return base == "test.yaml" || base == dockerComposeFile
}
