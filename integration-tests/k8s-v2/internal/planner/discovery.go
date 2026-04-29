package planner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type TestCase struct {
	Name     string
	Dir      string
	Requires []string
}

type requirementsFile struct {
	Requires []string `yaml:"requires"`
}

func DiscoverTests(root string) ([]TestCase, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read tests root %q: %w", root, err)
	}

	var tests []TestCase
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		testDir := filepath.Join(root, entry.Name())
		reqPath := filepath.Join(testDir, "requirements.yaml")

		raw, err := os.ReadFile(reqPath)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", reqPath, err)
		}

		var req requirementsFile
		if err := yaml.Unmarshal(raw, &req); err != nil {
			return nil, fmt.Errorf("parse %q: %w", reqPath, err)
		}
		if len(req.Requires) == 0 {
			return nil, fmt.Errorf("%q must declare at least one requirement", reqPath)
		}

		normalized := make([]string, 0, len(req.Requires))
		seen := map[string]struct{}{}
		for _, r := range req.Requires {
			r = strings.TrimSpace(strings.ToLower(r))
			if r == "" {
				continue
			}
			if _, ok := seen[r]; ok {
				continue
			}
			seen[r] = struct{}{}
			normalized = append(normalized, r)
		}
		sort.Strings(normalized)

		tests = append(tests, TestCase{
			Name:     entry.Name(),
			Dir:      testDir,
			Requires: normalized,
		})
	}

	sort.Slice(tests, func(i, j int) bool {
		return tests[i].Name < tests[j].Name
	})

	if len(tests) == 0 {
		return nil, fmt.Errorf("no tests discovered under %q", root)
	}
	return tests, nil
}
