package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// Define flags
	message := flag.String("message", "", "The message to print (required)")
	count := flag.Int("count", 1, "Number of times to print the message (default: 1)")

	// Parse command-line flags
	flag.Parse()

	// Validate required flags
	if *message == "" {
		fmt.Fprintln(os.Stderr, "Error: --message is required")
		fmt.Fprintln(os.Stderr, "\nUsage: go run main.go --message \"Your message here\" [--count N]")
		fmt.Fprintln(os.Stderr, "  --message (required): The message to print")
		fmt.Fprintln(os.Stderr, "  --count (optional):   Number of times to print (default: 1)")
		os.Exit(1)
	}

	// Validate count is positive
	if *count < 1 {
		fmt.Fprintln(os.Stderr, "Error: --count must be at least 1")
		os.Exit(1)
	}

	// Print the message the specified number of times
	for i := 0; i < *count; i++ {
		fmt.Println(*message)
	}
}
