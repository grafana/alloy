package discover

import "fmt"

// GoModFiles returns every go.mod file under root, skipping vendor,
// node_modules, .git, and testdata directories.
func GoModFiles(root string) (Result, error) {
	result, err := Files(root, "go.mod", WithSkipDirs(".git", "node_modules", "vendor", "testdata"))
	if err != nil {
		return result, fmt.Errorf("discover go.mod file: %w", err)
	}
	return result, nil
}
