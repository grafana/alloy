package scenariosdocs

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

const (
	// rawBaseURL serves raw file contents for a given commit.
	rawBaseURL = "https://raw.githubusercontent.com"

	// maxReadmeBytes caps how much of the README is read. The real file is a
	// few tens of kilobytes, so anything near the cap is unexpected.
	maxReadmeBytes = 1 << 20
)

// fetchReadme downloads README.md of alloy-scenarios at the given commit.
//
// The URL is built only from constants and a SHA that has been validated as 40
// lowercase hex characters, so the pin file cannot redirect the request.
func fetchReadme(ctx context.Context, client *http.Client, baseURL, sha string) ([]byte, error) {
	if err := validateSHA(sha); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/%s/%s/README.md", baseURL, repoSlug, sha)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", url, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: unexpected status %s", url, resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxReadmeBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	if len(data) > maxReadmeBytes {
		return nil, fmt.Errorf("fetch %s: response larger than %d bytes", url, maxReadmeBytes)
	}
	return data, nil
}
