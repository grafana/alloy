package goversion

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const golangTagURL = "https://hub.docker.com/v2/repositories/library/golang/tags/"

var golangDigestCache = map[string]string{}

// resolveGolangDigest returns the multi-arch index digest (e.g. "sha256:...")
// for a golang image tag such as "1.27.0-alpine", using the Docker Hub per-tag
// API. Only Docker Hub library/golang images are supported, which covers every
// digest-pinned golang reference in this repo. Results are cached for the run.
func resolveGolangDigest(tag string) (string, error) {
	if digest, ok := golangDigestCache[tag]; ok {
		return digest, nil
	}

	resp, err := http.Get(golangTagURL + tag)
	if err != nil {
		return "", fmt.Errorf("fetch tag %s: %w", tag, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch tag %s: %s", tag, resp.Status)
	}

	var data dockerTag
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("decode tag %s: %w", tag, err)
	}
	if data.Digest == "" {
		return "", fmt.Errorf("no digest for tag %s", tag)
	}

	golangDigestCache[tag] = data.Digest
	return data.Digest, nil
}

const alloyBuildImageTagsURL = "https://hub.docker.com/v2/repositories/grafana/alloy-build-image/tags?page_size=1"

type dockerTagsResponse struct {
	Results []dockerTag `json:"results"`
}

type dockerTag struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

func fetchBuildImageTags() (*dockerTagsResponse, error) {
	resp, err := http.Get(alloyBuildImageTagsURL)
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
