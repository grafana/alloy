package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
)

func main() {
	if len(os.Args) < 3 {
		printUsage()
		os.Exit(1)
	}

	command, version := os.Args[1], os.Args[2]

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get working directory: %s", err)
	}

	switch command {
	case "build-image":
		if err := updateBuildImage(wd, version); err != nil {
			log.Fatalf("failed to update build image version: %s", err)
		}
	case "go-mod":
		if err := updateGoMod(wd, version); err != nil {
			log.Fatalf("failed to update go.mod version: %s", err)
		}
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "usage: <build-image|go-mod> <version>\n")
}

func updateBuildImage(wd string, version string) error {
	err := updateBuildImageVersion(filepath.Join(wd, ".github/workflows/create_build_image.yml"), version, replaceWorkflowBuildImageVersion)
	if err != nil {
		return err
	}

	err = updateBuildImageVersion(filepath.Join(wd, ".github/workflows/check-linux-build-image.yml"), version, replaceWorkflowBuildImageVersion)
	if err != nil {
		log.Fatalf("failed to update build image version: %s", err)
	}

	err = updateBuildImageVersion(filepath.Join(wd, "tools/build-image/windows/Dockerfile"), version, replaceWindowsBuildImageVersion)
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

func updateGoMod(wb string, version string) error {
	var paths []string

	if err := filepath.Walk(wb, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if info.Name() == "vendor" {
				return filepath.SkipDir
			}
		}

		if info.Name() == "go.mod" {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return err
	}

	re := regexp.MustCompile(`(?m)^go 1\.\d+(\.\d+)?\s*$`)

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		newContent := re.ReplaceAllLiteral(content, []byte("go "+version+"\n"))
		if err := os.WriteFile(path, newContent, 0644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	return nil
}
