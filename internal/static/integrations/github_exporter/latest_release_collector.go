package github_exporter

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"time"

	gh_config "github.com/githubexporter/github-exporter/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tomnomnom/linkheader"
)

type latestReleaseCollector struct {
	log    *slog.Logger
	config *gh_config.Config
	client *http.Client
	desc   *prometheus.Desc
}

type githubRepository struct {
	Name  string `json:"name"`
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type githubRelease struct {
	Name        string `json:"name"`
	TagName     string `json:"tag_name"`
	CreatedAt   string `json:"created_at"`
	PublishedAt string `json:"published_at"`
}

func newLatestReleaseCollector(log *slog.Logger, config *gh_config.Config) *latestReleaseCollector {
	return &latestReleaseCollector{
		log:    log,
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
		desc: prometheus.NewDesc(
			prometheus.BuildFQName("github", "repo", "latest_release_info"),
			"Latest published full release for a given repository.",
			[]string{"repo", "user", "release", "tag", "created_at", "published_at"},
			nil,
		),
	}
}

func (c *latestReleaseCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *latestReleaseCollector) Collect(ch chan<- prometheus.Metric) {
	repositories, err := c.collectRepositories()
	if err != nil {
		c.log.Error("error collecting GitHub repositories for latest release metric", "err", err)
		return
	}

	for _, repository := range repositories {
		release, ok := c.getLatestRelease(repository)
		if !ok {
			continue
		}

		ch <- prometheus.MustNewConstMetric(
			c.desc,
			prometheus.GaugeValue,
			1,
			repository.Name,
			repository.Owner.Login,
			release.Name,
			release.TagName,
			release.CreatedAt,
			release.PublishedAt,
		)
	}
}

func (c *latestReleaseCollector) collectRepositories() ([]githubRepository, error) {
	var repositories []githubRepository
	seenRepositories := map[string]struct{}{}

	targetURLs := append([]string(nil), c.config.TargetURLs()...)
	for len(targetURLs) > 0 {
		targetURL := targetURLs[0]
		targetURLs = targetURLs[1:]

		resp, body, err := c.get(targetURL)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusNotFound {
			c.log.Debug("GitHub repository target not found while collecting latest release metric", "url", targetURL)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("unexpected GitHub API status %s for %s", resp.Status, targetURL)
		}

		targetURLs = append(targetURLs, paginationURLs(resp.Header)...)

		for _, repository := range decodeRepositories(body) {
			if repository.Name == "" || repository.Owner.Login == "" {
				continue
			}

			key := repository.Owner.Login + "/" + repository.Name
			if _, ok := seenRepositories[key]; ok {
				continue
			}

			repositories = append(repositories, repository)
			seenRepositories[key] = struct{}{}
		}
	}

	return repositories, nil
}

func (c *latestReleaseCollector) getLatestRelease(repository githubRepository) (githubRelease, bool) {
	resp, body, err := c.get(c.latestReleaseURL(repository))
	if err != nil {
		c.log.Error("error fetching latest GitHub release", "repo", repository.Name, "user", repository.Owner.Login, "err", err)
		return githubRelease{}, false
	}

	if resp.StatusCode == http.StatusNotFound {
		c.log.Debug("GitHub repository has no latest release", "repo", repository.Name, "user", repository.Owner.Login)
		return githubRelease{}, false
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.log.Error("unexpected GitHub API status fetching latest release", "repo", repository.Name, "user", repository.Owner.Login, "status", resp.Status)
		return githubRelease{}, false
	}

	var release githubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		c.log.Error("error decoding latest GitHub release", "repo", repository.Name, "user", repository.Owner.Login, "err", err)
		return githubRelease{}, false
	}

	if release.TagName == "" {
		c.log.Debug("latest GitHub release has no tag", "repo", repository.Name, "user", repository.Owner.Login)
		return githubRelease{}, false
	}

	return release, true
}

func (c *latestReleaseCollector) latestReleaseURL(repository githubRepository) string {
	u := *c.config.APIURL()
	u.Path = path.Join(u.Path, "repos", repository.Owner.Login, repository.Name, "releases", "latest")
	u.RawQuery = ""
	return u.String()
}

func (c *latestReleaseCollector) get(url string) (*http.Response, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}

	if token := c.config.APIToken(); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	return resp, body, nil
}

func decodeRepositories(body []byte) []githubRepository {
	if isJSONArray(body) {
		var repositories []githubRepository
		_ = json.Unmarshal(body, &repositories)
		return repositories
	}

	var repository githubRepository
	_ = json.Unmarshal(body, &repository)
	return []githubRepository{repository}
}

func isJSONArray(body []byte) bool {
	for _, c := range body {
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			continue
		}
		return c == '['
	}
	return false
}

func paginationURLs(header http.Header) []string {
	var urls []string
	for _, link := range linkheader.ParseMultiple(header.Values("Link")) {
		if link.Rel == "next" {
			urls = append(urls, link.URL)
		}
	}
	return urls
}
