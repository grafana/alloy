package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "build-image" {
		fmt.Fprintf(os.Stderr, "usage: build-image -version <version>\n")
		os.Exit(1)
	}

	var version string
	fs := flag.NewFlagSet("go-version", flag.ExitOnError)
	fs.StringVar(&version, "version", "", "Go version for build images (e.g. 1.25.7)")
	fs.Parse(os.Args[2:])

	if version == "" {
		fmt.Fprintf(os.Stderr, "usage: build-image -version <version>\n")
		os.Exit(1)
	}

	root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	if err := updateBuildImage(root, version); err != nil {
		log.Fatalf("failed to update build image version: %s", err)
	}
}

func updateBuildImage(root string, version string) error {
	err := updateBuildImageVersion(filepath.Join(root, ".github/workflows/create_build_image.yml"), version, replaceWorkflowBuildImageVersion)
	if err != nil {
		return err
	}
	err = updateBuildImageVersion(filepath.Join(root, ".github/workflows/check-linux-build-image.yml"), version, replaceWorkflowBuildImageVersion)
	if err != nil {
		log.Fatalf("failed to update build image version: %s", err)
	}
	err = updateBuildImageVersion(filepath.Join(root, "tools/build-image/windows/Dockerfile"), version, replaceWindowsBuildImageVersion)
	if err != nil {
		log.Fatalf("failed to update build image version: %s", err)
	}
	return nil
}

func updateBuildImageVersion(path, version string, replace func(content []byte, version string) []byte) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if err := os.WriteFile(path, replace(content, version), 0644); err != nil {
		return fmt.Errorf("failed to update file: %w", err)
	}

	return nil
}

func replaceWorkflowBuildImageVersion(content []byte, version string) []byte {
	out := regexp.MustCompile(`golang:1\.\d+\.\d+-alpine3\.\d+`).ReplaceAllLiteral(content, []byte("golang:"+version+"-alpine3.23"))
	out = regexp.MustCompile(`golang:1\.\d+\.\d+-bookworm`).ReplaceAllLiteral(out, []byte("golang:"+version+"-bookworm"))
	return out
}

func replaceWindowsBuildImageVersion(content []byte, version string) []byte {
	re := regexp.MustCompile(`library/golang:1\.\d+\.\d+-windowsservercore-ltsc2022`)
	return re.ReplaceAllLiteral(content, []byte("library/golang:"+version+"-windowsservercore-ltsc2022"))
}
