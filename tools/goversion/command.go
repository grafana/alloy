package goversion

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/tools/internal/git"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-go-version <command>",
		Short: "go version update",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}

	cmd.AddCommand(
		pr1Command(),
		pr2Command(),
	)

	return cmd
}

func pr1Command() *cobra.Command {
	return &cobra.Command{
		Use: "pr-1 <version>",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("missing argument")
			}

			root, err := git.Root()
			if err != nil {
				return err
			}

			return updateBuildImage(root, args[0])
		},
	}
}

func pr2Command() *cobra.Command {
	return &cobra.Command{
		Use: "pr-2 <version>",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("missing argument")
			}

			root, err := git.Root()
			if err != nil {
				return err
			}

			version := args[0]

			if err := updateGoModFiles(root, version); err != nil {
				log.Fatalf("failed to update go.mod files: %s", err)
				return fmt.Errorf("error updating go.mod files: %w", err)
			}
			if err := updateDockerFiles(root, version); err != nil {
				log.Fatalf("failed to update Dockerfiles: %s", err)
				return fmt.Errorf("error updating Dockerfiles: %w", err)
			}
			if err := bumpBuildImage(root); err != nil {
				return fmt.Errorf("error updating build image: %w", err)
			}

			return nil
		},
	}
}

func updateBuildImage(root string, version string) error {
	paths := []string{
		".github/workflows/create_build_image.yml",
		".github/workflows/check-linux-build-image.yml",
		"build-tools/build-image/windows/Dockerfile",
	}

	for _, path := range paths {
		path = filepath.Join(root, path)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		if err := os.WriteFile(path, replaceDockerGoVersion(content, version), 0644); err != nil {
			return fmt.Errorf("failed to update file: %w", err)
		}

	}

	return nil
}

func updateGoModFiles(root, version string) error {
	paths, err := getPaths(root, "go.mod", "tools/generate/testdata")
	if err != nil {
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

func updateDockerFiles(root, version string) error {
	paths, err := getPaths(root, "Dockerfile", "Dockerfile.windows", "build-tools/build-image")
	if err != nil {
		return err
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if err := os.WriteFile(path, replaceDockerGoVersion(content, version), 0644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	return nil
}

func bumpBuildImage(root string) error {
	data, err := fetchBuildImageTags()
	if err != nil {
		return err
	}
	refs, err := buildImageRefsFromTags(data)
	if err != nil {
		return err
	}

	var paths = []string{
		"Dockerfile",
		".github/workflows/build.yml",
		".github/workflows/release-publish-alloy-artifacts.yml",
		".github/workflows/test_full.yml",
		".github/workflows/publish-alloy-linux.yml",
		".github/workflows/test_linux_system_packages.yml",
	}

	for _, relPath := range paths {
		path := filepath.Join(root, relPath)
		content, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read %s: %w", relPath, err)
		}
		newContent := replaceBuildImageRefs(content, refs)
		if err := os.WriteFile(path, newContent, 0644); err != nil {
			return fmt.Errorf("write %s: %w", relPath, err)
		}
	}
	return nil
}

const dockerHubTagsURL = "https://hub.docker.com/v2/repositories/grafana/alloy-build-image/tags?page_size=2"

type dockerTagsResponse struct {
	Results []dockerTag `json:"results"`
}

type dockerTag struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

type buildImageRefs struct {
	Default    string
	Boring     string
	DefaultTag string
}

func fetchBuildImageTags() (*dockerTagsResponse, error) {
	resp, err := http.Get(dockerHubTagsURL)
	if err != nil {
		return nil, fmt.Errorf("fetch tags: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch tags: %s", resp.Status)
	}
	var data dockerTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode tags: %w", err)
	}
	return &data, nil
}

// buildImageRefsFromTags assumes 2 tags: one default, one boringcrypto.
func buildImageRefsFromTags(data *dockerTagsResponse) (*buildImageRefs, error) {
	if len(data.Results) != 2 {
		return nil, fmt.Errorf("expected 2 tags, got %d", len(data.Results))
	}
	var defaultTag, boringTag *dockerTag
	for i := range data.Results {
		t := &data.Results[i]
		if strings.HasSuffix(t.Name, "-boringcrypto") {
			boringTag = t
		} else {
			defaultTag = t
		}
	}
	if defaultTag == nil || boringTag == nil {
		return nil, fmt.Errorf("expected one default and one boringcrypto tag")
	}
	return &buildImageRefs{
		Default:    "grafana/alloy-build-image:" + defaultTag.Name + "@" + defaultTag.Digest,
		Boring:     "grafana/alloy-build-image:" + boringTag.Name + "@" + boringTag.Digest,
		DefaultTag: "grafana/alloy-build-image:" + defaultTag.Name,
	}, nil
}

var (
	buildImageWithDigestRE   = regexp.MustCompile(`grafana/alloy-build-image:v\d+\.\d+\.\d+@sha256:[a-f0-9]+`)
	buildImageBoringDigestRE = regexp.MustCompile(`grafana/alloy-build-image:v\d+\.\d+\.\d+-boringcrypto@sha256:[a-f0-9]+`)
	buildImageTagOnlyRE      = regexp.MustCompile(`grafana/alloy-build-image:v\d+\.\d+\.\d+(\s|$)`)
)

func replaceBuildImageRefs(content []byte, refs *buildImageRefs) []byte {
	out := buildImageBoringDigestRE.ReplaceAllLiteral(content, []byte(refs.Boring))
	out = buildImageWithDigestRE.ReplaceAllLiteral(out, []byte(refs.Default))
	out = buildImageTagOnlyRE.ReplaceAll(out, []byte(refs.DefaultTag+"$1"))
	return out
}

func getPaths(root, pattern string, exclude ...string) ([]string, error) {
	var paths []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasPrefix(info.Name(), pattern) {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		for _, ex := range exclude {
			if rel == ex || strings.HasPrefix(rel, ex+string(filepath.Separator)) {
				return nil
			}
		}

		paths = append(paths, path)
		return nil
	})

	return paths, err
}

var dockerGoVersionRE = regexp.MustCompile(`golang:1\.\d+(\.\d+)?`)

func replaceDockerGoVersion(content []byte, version string) []byte {
	out := dockerGoVersionRE.ReplaceAllLiteral(content, []byte("golang:"+version))
	return out
}

