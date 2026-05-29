package discover

// GoModFiles returns every go.mod file under root, skipping vendor,
// node_modules, .git, and testdata directories.
func GoModFiles(root string) (Result, error) {
	return Files(root, "go.mod", WithSkipDirs(".git", "node_modules", "vendor", "testdata"))
}
