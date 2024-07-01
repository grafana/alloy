//go:build mage

package main

import (
	"github.com/magefile/mage/mg"
	"os"
	"path/filepath"
)

func Lint() error {
	mg.SerialDeps(AlloyLint)
	matches, err := filepath.Glob("./**/go.mod")
	if err != nil {
		return err
	}
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	for _, match := range matches {
		folder := filepath.Dir(match)
		os.Chdir(folder)
		err = ExecNoEnv(folder+" golangcilint", "golangci-lint", "run", "-v", "--timeout=10m")
		if err != nil {
			os.Chdir(wd)
			return err
		}
	}
	os.Chdir(wd)
	return ExecNoEnv("alloy lint", "build/alloylint", "./...")
}

func AlloyLint() error {
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir("./internal/cmd/alloylint")
	return ExecNoEnv("building alloy lint", "go", "build", "-o", "../../../build/alloylint")
}
