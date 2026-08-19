package main

import "os"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "plan" {
		os.Exit(runBuildPlan(os.Args[2:], os.Stdin, os.Stdout, os.Stderr, os.Environ(), execDelegate{}))
	}
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Environ(), execDelegate{}))
}
