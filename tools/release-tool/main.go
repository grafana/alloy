package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/spf13/cobra"
)

type releaseConfig struct {
	commitSHA     string
	skipOTELCheck bool
}

func main() {
	cfg := &releaseConfig{}

	var rootCmd = &cobra.Command{
		Use:   "release-tool",
		Short: "Alloy Release Tool - Automates the release process",
		Long: `A tool to automate Grafana Alloy releases following the official release process.
Complete documentation is available at https://github.com/grafana/alloy/tree/main/docs/developer/release`,
	}

	// First Release Candidate Command
	var firstRCCmd = &cobra.Command{
		Use:   "first-rc",
		Short: "Create the first release candidate (rc.0)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkOnCleanMainBranch(); err != nil {
				return err
			}

			if !cfg.skipOTELCheck {
				fmt.Println("Checking OpenTelemetry Collector dependency...")
				return fmt.Errorf("please update the OpenTelemetry Collector dependency before creating a release candidate\nSee: https://github.com/grafana/alloy/tree/main/docs/developer/release/0-ensure-otel-dep-updated.md")
			}

			if err := validateCommitSHA(cfg.commitSHA); err != nil {
				return err
			}
			fmt.Printf("Using commit SHA: %s\n", cfg.commitSHA)

			// if err := validateFirstRCVersion(cfg.version); err != nil {
			// 	return err
			// }

			// TODO: Implement remaining first RC creation logic
			return nil
		},
	}

	// Additional Release Candidate Command
	var additionalRCCmd = &cobra.Command{
		Use:   "additional-rc",
		Short: "Create additional release candidate (rc.1, rc.2, etc)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkOnCleanMainBranch(); err != nil {
				return err
			}

			if err := validateCommitSHA(cfg.commitSHA); err != nil {
				return err
			}
			fmt.Printf("Using commit SHA: %s\n", cfg.commitSHA)

			currentMainBranchVersion, err := readCurrentMainBranchVersion()
			if err != nil {
				return err
			}
			fmt.Printf("Current main branch version: %s\n", currentMainBranchVersion)

			decreasedVersion, err := decreaseMinorVersion(currentMainBranchVersion)
			if err != nil {
				return fmt.Errorf("failed to calculate expected version: %w", err)
			}
			existingTagsPrefix := strings.Join(strings.Split(decreasedVersion, ".")[:2], ".")
			fmt.Printf("Existing tags prefix: %s\n", existingTagsPrefix)

			fmt.Println("Creating additional release candidate...")
			return nil
		},
	}

	// Add flags to commands that need them
	addReleaseFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringVar(&cfg.commitSHA, "commit", "", "Commit SHA to release")
		_ = cmd.MarkFlagRequired("commit")
	}

	firstRCCmd.Flags().BoolVar(&cfg.skipOTELCheck, "skip-otel-check", false, "Skip OpenTelemetry dependency check")
	addReleaseFlags(firstRCCmd)
	addReleaseFlags(additionalRCCmd)

	// Stable Release Command
	var stableReleaseCmd = &cobra.Command{
		Use:   "stable-release",
		Short: "Create stable release",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement stable release logic
			fmt.Println("Creating stable release...")
			return nil
		},
	}

	// Patch Release Command
	var patchReleaseCmd = &cobra.Command{
		Use:   "patch-release",
		Short: "Create patch release",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement patch release logic
			fmt.Println("Creating patch release...")
			return nil
		},
	}

	// Add commands to root command
	rootCmd.AddCommand(firstRCCmd)
	rootCmd.AddCommand(additionalRCCmd)
	rootCmd.AddCommand(stableReleaseCmd)
	rootCmd.AddCommand(patchReleaseCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func validateCommitSHA(sha string) error {
	if len(sha) < 7 {
		return fmt.Errorf("commit SHA too short, must be at least 7 characters")
	}

	// Verify the SHA exists
	_, err := exec.Command("git", "rev-parse", "--verify", sha).Output()
	if err != nil {
		return fmt.Errorf("invalid commit SHA - please check it exists in the main branch: %w", err)
	}

	return nil
}

func checkOnCleanMainBranch() error {
	// Check if we're on main branch
	// out, err := exec.Command("git", "branch", "--show-current").Output()
	// if err != nil {
	// 	return fmt.Errorf("failed to check current branch: %w", err)
	// }
	// if strings.TrimSpace(string(out)) != "main" {
	// 	return fmt.Errorf("not on main branch")
	// }
	//
	// // Fetch from origin
	// if err := exec.Command("git", "fetch", "origin", "main").Run(); err != nil {
	// 	return fmt.Errorf("failed to fetch from origin: %w", err)
	// }
	//
	// // Check for uncommitted changes
	// out, err = exec.Command("git", "status", "--porcelain").Output()
	// if err != nil {
	// 	return fmt.Errorf("failed to check git status: %w", err)
	// }
	// if len(out) > 0 {
	// 	return fmt.Errorf("working directory not clean, please commit or stash changes")
	// }
	//
	return nil
}

func validateFirstRCVersion(version string) error {
	v, err := semver.NewVersion(version)
	if err != nil {
		return fmt.Errorf("invalid semantic version %q: %w", version, err)
	}

	if v.PreRelease != "rc.0" || v.Patch != 0 {
		return fmt.Errorf("first RC version must end in .0-rc.0")
	}
	return nil
}

func readCurrentMainBranchVersion() (string, error) {
	content, err := os.ReadFile("VERSION")
	if err != nil {
		return "", fmt.Errorf("failed to read VERSION file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		// Skip empty lines and comments
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return line, nil
	}

	return "", fmt.Errorf("no version found in VERSION file")
}

func decreaseMinorVersion(version string) (string, error) {
	v, err := semver.NewVersion(strings.TrimPrefix(version, "v"))
	if err != nil {
		return "", fmt.Errorf("invalid semantic version %q: %w", version, err)
	}

	if v.Minor == 0 {
		return "", fmt.Errorf("cannot decrease minor version from 0")
	}

	// Create new version with decreased minor version
	newV := semver.New(fmt.Sprintf("%d.%d.%d", v.Major, v.Minor-1, 0))
	return "v" + newV.String(), nil
}
