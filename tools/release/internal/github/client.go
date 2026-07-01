// Package github provides shared GitHub client utilities for release tools.
package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client with repository context.
type Client struct {
	api        *github.Client
	owner      string
	repo       string
	graphqlURL string
}

// ClientConfig holds configuration for creating a new Client.
type ClientConfig struct {
	Token string
	Owner string
	Repo  string
}

// AppIdentity represents a GitHub App's git identity for commits.
type AppIdentity struct {
	Name  string // e.g., "my-app[bot]"
	Email string // e.g., "12345+my-app[bot]@users.noreply.github.com"
}

// CreateBranchParams holds parameters for CreateBranch.
type CreateBranchParams struct {
	Branch string
	SHA    string
}

// CreateTagParams holds parameters for CreateTag.
type CreateTagParams struct {
	Tag     string
	SHA     string
	Message string
}

// CreatePRParams holds parameters for CreatePR.
type CreatePRParams struct {
	Title string
	Head  string
	Base  string
	Body  string
	Draft bool
}

// FindCherryPickedCommitParams holds parameters for FindCherryPickedCommit.
type FindCherryPickedCommitParams struct {
	Branch      string
	OriginalSHA string
}

// FileAddition represents a file addition or update for CreateCommitOnBranch.
type FileAddition struct {
	Path     string
	Contents []byte
}

// CreateCommitOnBranchParams holds parameters for CreateCommitOnBranch.
type CreateCommitOnBranchParams struct {
	Branch          string
	ExpectedHeadOID string
	Headline        string
	Body            string
	Additions       []FileAddition
	Deletions       []string
}

// BackportLabelColor is the hex color for backport labels (without '#' prefix).
const BackportLabelColor = "63a504"

const defaultGraphQLURL = "https://api.github.com/graphql"

// CreateLabelParams holds parameters for CreateLabel.
type CreateLabelParams struct {
	Name        string // Label name
	Color       string // Hex color without '#' prefix (e.g., "ff0000")
	Description string // Optional description
}

// NewClientFromEnv creates a new Client from environment variables.
// Reads GITHUB_TOKEN and GITHUB_REPOSITORY (format: owner/repo).
func NewClientFromEnv(ctx context.Context) (*Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable is required")
	}

	var owner, repo string
	if ghRepo := os.Getenv("GITHUB_REPOSITORY"); ghRepo != "" {
		parts := strings.SplitN(ghRepo, "/", 2)
		if len(parts) == 2 {
			owner = parts[0]
			repo = parts[1]
		}
	}

	if owner == "" || repo == "" {
		return nil, fmt.Errorf("GITHUB_REPOSITORY environment variable is required (format: owner/repo)")
	}

	return NewClient(ctx, ClientConfig{
		Token: token,
		Owner: owner,
		Repo:  repo,
	}), nil
}

// NewClient creates a new Client with the given configuration.
func NewClient(ctx context.Context, cfg ClientConfig) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.Token})
	tc := oauth2.NewClient(ctx, ts)

	return &Client{
		api:        github.NewClient(tc),
		owner:      cfg.Owner,
		repo:       cfg.Repo,
		graphqlURL: defaultGraphQLURL,
	}
}

// API returns the underlying go-github client for advanced usage.
func (c *Client) API() *github.Client {
	return c.api
}

// Owner returns the repository owner.
func (c *Client) Owner() string {
	return c.owner
}

// Repo returns the repository name.
func (c *Client) Repo() string {
	return c.repo
}

// BranchExists checks if a branch exists in the repository.
func (c *Client) BranchExists(ctx context.Context, branch string) (bool, error) {
	_, resp, err := c.api.Repositories.GetBranch(ctx, c.owner, c.repo, branch, 0)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		var errResp *github.ErrorResponse
		if errors.As(err, &errResp) && errResp.Response.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetRefSHA resolves a ref (branch, tag, or commit SHA) to its SHA.
func (c *Client) GetRefSHA(ctx context.Context, ref string) (string, error) {
	// Try as a branch first
	branch, _, err := c.api.Repositories.GetBranch(ctx, c.owner, c.repo, ref, 0)
	if err == nil {
		return branch.GetCommit().GetSHA(), nil
	}

	// Try as a tag
	tagRef, _, err := c.api.Git.GetRef(ctx, c.owner, c.repo, "tags/"+ref)
	if err == nil {
		sha := tagRef.GetObject().GetSHA()
		if tagRef.GetObject().GetType() == "tag" {
			tagObj, _, tagErr := c.api.Git.GetTag(ctx, c.owner, c.repo, sha)
			if tagErr != nil {
				return "", fmt.Errorf("dereferencing annotated tag %s: %w", ref, tagErr)
			}
			sha = tagObj.GetObject().GetSHA()
		}
		return sha, nil
	}

	// Try as a commit SHA
	commit, err := c.GetCommit(ctx, ref)
	if err == nil {
		sha := commit.GetSHA()
		if sha == "" {
			return "", fmt.Errorf("commit SHA is empty for ref: %s", ref)
		}
		return sha, nil
	}

	return "", fmt.Errorf("could not resolve ref: %s", ref)
}

// CreateBranch creates a new branch from the given SHA.
func (c *Client) CreateBranch(ctx context.Context, p CreateBranchParams) error {
	ref := &github.Reference{
		Ref: github.String("refs/heads/" + p.Branch),
		Object: &github.GitObject{
			SHA: github.String(p.SHA),
		},
	}

	_, _, err := c.api.Git.CreateRef(ctx, c.owner, c.repo, ref)
	if err != nil {
		return fmt.Errorf("creating branch ref: %w", err)
	}

	return nil
}

// CreateTag creates an annotated tag ref for the given SHA.
func (c *Client) CreateTag(ctx context.Context, p CreateTagParams) error {
	identity, err := c.GetAppIdentity(ctx)
	if err != nil {
		return fmt.Errorf("getting app identity for tagger: %w", err)
	}

	tagObj := &github.Tag{
		Tag:     github.String(p.Tag),
		Message: github.String(p.Message),
		Tagger: &github.CommitAuthor{
			Name:  github.String(identity.Name),
			Email: github.String(identity.Email),
			Date:  &github.Timestamp{Time: time.Now().UTC()},
		},
		Object: &github.GitObject{
			SHA:  github.String(p.SHA),
			Type: github.String("commit"),
		},
	}

	created, _, err := c.api.Git.CreateTag(ctx, c.owner, c.repo, tagObj)
	if err != nil {
		return fmt.Errorf("creating tag object: %w", err)
	}

	ref := &github.Reference{
		Ref: github.String("refs/tags/" + p.Tag),
		Object: &github.GitObject{
			SHA: github.String(created.GetSHA()),
		},
	}

	_, _, err = c.api.Git.CreateRef(ctx, c.owner, c.repo, ref)
	if err != nil {
		return fmt.Errorf("creating tag ref: %w", err)
	}

	return nil
}

// GetAppIdentity returns the GitHub App's identity for use in git commits.
// It checks for APP_SLUG environment variable and fetches the bot user ID from the API.
// The bot user ID is required for GitHub to properly attribute commits.
func (c *Client) GetAppIdentity(ctx context.Context) (AppIdentity, error) {
	// Prefer APP_SLUG env var - fetch bot user ID from users API
	appSlug := os.Getenv("APP_SLUG")
	if appSlug != "" {
		botUsername := fmt.Sprintf("%s[bot]", appSlug)
		// Look up the bot user to get its actual user ID
		botUser, _, err := c.api.Users.Get(ctx, botUsername)
		if err != nil {
			return AppIdentity{}, fmt.Errorf("getting bot user %q: %w", botUsername, err)
		}
		return AppIdentity{
			Name:  botUsername,
			Email: fmt.Sprintf("%d+%s@users.noreply.github.com", botUser.GetID(), botUsername),
		}, nil
	}

	// Fall back to API call (requires JWT authentication, not installation token)
	app, _, err := c.api.Apps.Get(ctx, "")
	if err != nil {
		return AppIdentity{}, fmt.Errorf("getting app info: %w (hint: set APP_SLUG env var when using installation tokens)", err)
	}

	slug := app.GetSlug()
	botUsername := fmt.Sprintf("%s[bot]", slug)

	// Look up the bot user to get its actual user ID
	botUser, _, err := c.api.Users.Get(ctx, botUsername)
	if err != nil {
		return AppIdentity{}, fmt.Errorf("getting bot user %q: %w", botUsername, err)
	}

	return AppIdentity{
		Name:  botUsername,
		Email: fmt.Sprintf("%d+%s@users.noreply.github.com", botUser.GetID(), botUsername),
	}, nil
}

// GetPR fetches a pull request by number.
func (c *Client) GetPR(ctx context.Context, number int) (*github.PullRequest, error) {
	pr, _, err := c.api.PullRequests.Get(ctx, c.owner, c.repo, number)
	if err != nil {
		return nil, fmt.Errorf("getting PR #%d: %w", number, err)
	}
	return pr, nil
}

// CreatePR creates a new pull request.
func (c *Client) CreatePR(ctx context.Context, p CreatePRParams) (*github.PullRequest, error) {
	newPR := &github.NewPullRequest{
		Title: new(p.Title),
		Head:  new(p.Head),
		Base:  new(p.Base),
		Body:  new(p.Body),
		Draft: new(p.Draft),
	}

	pr, _, err := c.api.PullRequests.Create(ctx, c.owner, c.repo, newPR)
	if err != nil {
		return nil, fmt.Errorf("creating pull request: %w", err)
	}

	return pr, nil
}

// CreateCommitOnBranch creates a commit through GitHub's GraphQL API.
//
// Do not add custom author, committer, or signature fields here. GitHub only
// applies bot signature verification when the authenticated App request omits
// those fields.
func (c *Client) CreateCommitOnBranch(ctx context.Context, p CreateCommitOnBranchParams) (string, error) {
	if p.Branch == "" {
		return "", fmt.Errorf("branch is required")
	}
	if p.ExpectedHeadOID == "" {
		return "", fmt.Errorf("expected head OID is required")
	}
	if p.Headline == "" {
		return "", fmt.Errorf("commit headline is required")
	}
	if len(p.Additions) == 0 && len(p.Deletions) == 0 {
		return "", fmt.Errorf("at least one file addition or deletion is required")
	}

	additions := make([]map[string]string, 0, len(p.Additions))
	for _, addition := range p.Additions {
		if addition.Path == "" {
			return "", fmt.Errorf("addition path is required")
		}
		additions = append(additions, map[string]string{
			"path":     addition.Path,
			"contents": base64.StdEncoding.EncodeToString(addition.Contents),
		})
	}

	deletions := make([]map[string]string, 0, len(p.Deletions))
	for _, deletion := range p.Deletions {
		if deletion == "" {
			return "", fmt.Errorf("deletion path is required")
		}
		deletions = append(deletions, map[string]string{
			"path": deletion,
		})
	}

	message := map[string]string{
		"headline": p.Headline,
	}
	if p.Body != "" {
		message["body"] = p.Body
	}

	input := map[string]any{
		"branch": map[string]string{
			"repositoryNameWithOwner": c.owner + "/" + c.repo,
			"branchName":              p.Branch,
		},
		"message":         message,
		"expectedHeadOid": p.ExpectedHeadOID,
		"fileChanges": map[string]any{
			"additions": additions,
			"deletions": deletions,
		},
	}

	const mutation = `
mutation CreateCommitOnBranch($input: CreateCommitOnBranchInput!) {
  createCommitOnBranch(input: $input) {
    commit {
      oid
    }
  }
}`

	var result struct {
		Data struct {
			CreateCommitOnBranch struct {
				Commit struct {
					OID string `json:"oid"`
				} `json:"commit"`
			} `json:"createCommitOnBranch"`
		} `json:"data"`
	}

	if err := c.GraphQL(ctx, mutation, map[string]any{"input": input}, &result); err != nil {
		return "", fmt.Errorf("creating commit on branch %s: %w", p.Branch, err)
	}
	if result.Data.CreateCommitOnBranch.Commit.OID == "" {
		return "", fmt.Errorf("createCommitOnBranch returned an empty commit oid")
	}

	return result.Data.CreateCommitOnBranch.Commit.OID, nil
}

// FindCherryPickedCommit searches the commit history of a branch for a commit
// message containing the original commit SHA. It returns nil when no matching
// commit is found.
func (c *Client) FindCherryPickedCommit(ctx context.Context, p FindCherryPickedCommitParams) (*github.RepositoryCommit, error) {
	opts := &github.CommitsListOptions{
		SHA: p.Branch,
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	// Search through recent commits (up to 500)
	for range 5 {
		commits, resp, err := c.api.Repositories.ListCommits(ctx, c.owner, c.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("listing commits: %w", err)
		}

		for _, commit := range commits {
			message := commit.GetCommit().GetMessage()
			trailer := fmt.Sprintf("(cherry picked from commit %s)", p.OriginalSHA)

			if strings.Contains(strings.ToLower(message), strings.ToLower(trailer)) {
				return commit, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return nil, nil
}

// EnsureLabel creates a label if it doesn't already exist.
// Returns true if the label was created, false if it already existed.
func (c *Client) EnsureLabel(ctx context.Context, p CreateLabelParams) (bool, error) {
	label := &github.Label{
		Name:        github.String(p.Name),
		Color:       github.String(p.Color),
		Description: github.String(p.Description),
	}

	_, _, err := c.api.Issues.CreateLabel(ctx, c.owner, c.repo, label)
	if err == nil {
		return true, nil
	}

	var errResp *github.ErrorResponse
	if errors.As(err, &errResp) {
		for _, e := range errResp.Errors {
			if e.Code == "already_exists" {
				return false, nil
			}
		}
	}

	return false, fmt.Errorf("creating label %q: %w", p.Name, err)
}

// GetReleaseByTag fetches a release by its tag name.
func (c *Client) GetReleaseByTag(ctx context.Context, tag string) (*github.RepositoryRelease, error) {
	release, _, err := c.api.Repositories.GetReleaseByTag(ctx, c.owner, c.repo, tag)
	if err != nil {
		return nil, fmt.Errorf("getting release for tag %s: %w", tag, err)
	}
	return release, nil
}

// UpdateReleaseBody updates only the body of a release.
func (c *Client) UpdateReleaseBody(ctx context.Context, releaseID int64, body string) error {
	_, _, err := c.api.Repositories.EditRelease(ctx, c.owner, c.repo, releaseID, &github.RepositoryRelease{
		Body: github.String(body),
	})
	if err != nil {
		return fmt.Errorf("updating release %d body: %w", releaseID, err)
	}
	return nil
}

// CreateIssueComment adds a comment to an issue or pull request.
func (c *Client) CreateIssueComment(ctx context.Context, issueNumber int, body string) error {
	comment := &github.IssueComment{
		Body: github.String(body),
	}
	_, _, err := c.api.Issues.CreateComment(ctx, c.owner, c.repo, issueNumber, comment)
	if err != nil {
		return fmt.Errorf("creating comment on issue #%d: %w", issueNumber, err)
	}
	return nil
}

// GetCommit fetches a commit by SHA.
func (c *Client) GetCommit(ctx context.Context, sha string) (*github.RepositoryCommit, error) {
	commit, _, err := c.api.Repositories.GetCommit(ctx, c.owner, c.repo, sha, nil)
	if err != nil {
		return nil, fmt.Errorf("getting commit %s: %w", sha, err)
	}
	return commit, nil
}

// GraphQL executes a GraphQL query against the GitHub API.
// The result parameter should be a pointer to a struct that will be decoded from the response.
func (c *Client) GraphQL(ctx context.Context, query string, variables map[string]any, result any) error {
	reqBody := map[string]any{
		"query":     query,
		"variables": variables,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.graphqlURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Get the HTTP client from the underlying go-github client (has auth configured)
	httpClient := c.api.Client()
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing graphql request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading graphql response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("graphql request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var payload struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return fmt.Errorf("decoding graphql response: %w", err)
	}
	if len(payload.Errors) > 0 {
		messages := make([]string, 0, len(payload.Errors))
		for _, graphQLError := range payload.Errors {
			messages = append(messages, graphQLError.Message)
		}
		return fmt.Errorf("graphql errors: %s", strings.Join(messages, "; "))
	}
	if err := json.Unmarshal(respBody, result); err != nil {
		return fmt.Errorf("decoding graphql response: %w", err)
	}

	return nil
}

// IsBot checks if a username appears to be a bot account.
func IsBot(username string) bool {
	return strings.HasSuffix(username, "[bot]") ||
		strings.HasSuffix(username, "-bot") ||
		username == "Copilot" ||
		username == "claude"
}
