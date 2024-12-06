package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
)

func main() {
	if len(os.Args) < 5 {
		fmt.Println("Usage: go run main.go <script-path> <name-prefix> <port-start> <N>")
		os.Exit(1)
	}

	scriptPath := os.Args[1]
	namePrefix := os.Args[2]
	portStart, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Println("Invalid port start")
		os.Exit(1)
	}
	numInstances, err := strconv.Atoi(os.Args[4])
	if err != nil {
		fmt.Println("Error converting num instances to integer")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	// Trap SIGINT (CTRL+C) and SIGTERM to gracefully shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Run the script N times in parallel
	for i := 0; i < numInstances; i++ {
		wg.Add(1)
		go func(instanceID int) {
			defer wg.Done()
			runScript(ctx, scriptPath, namePrefix, portStart, instanceID)
		}(i)
	}

	// Wait for CTRL+C or termination signal
	<-signalChan
	fmt.Println("\nShutting down...")
	cancel() // Signal all scripts to stop

	wg.Wait() // Wait for all goroutines to finish
	fmt.Println("All scripts stopped.")
	os.Exit(0)
}

// runScript runs a given script in a separate goroutine and pipes its output with a prefix
func runScript(ctx context.Context, scriptPath string, namePrefix string, portStart int, instanceID int) {
	cmd := exec.CommandContext(ctx, "bash", scriptPath, namePrefix+"-"+strconv.Itoa(instanceID), strconv.Itoa(portStart+instanceID))

	// Get the stdout and stderr pipes
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("[Instance %d] Error creating stdout pipe: %v\n", instanceID, err)
		return
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("[Instance %d] Error creating stderr pipe: %v\n", instanceID, err)
		return
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		fmt.Printf("[Instance %d] Failed to start script: %v\n", instanceID, err)
		return
	}

	// Log stdout and stderr with prefixes
	go logOutput(fmt.Sprintf("[Instance %d] stdout: ", instanceID), stdoutPipe)
	go logOutput(fmt.Sprintf("[Instance %d] stderr: ", instanceID), stderrPipe)

	// Wait for the command to complete
	if err := cmd.Wait(); err != nil {
		fmt.Printf("[Instance %d] Script exited with error: %v\n", instanceID, err)
	}
}

// logOutput reads from a pipe and logs each line with a given prefix
func logOutput(prefix string, pipe io.ReadCloser) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		fmt.Println(prefix + scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("%s Error reading output: %v\n", prefix, err)
	}
}
