package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

func TestCreateCommitOnBranchUsesGitHubSignedCommitMutation(t *testing.T) {
	ctx := context.Background()
	request := &graphQLRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleCreateCommitMutation(t, request, w, r)
	}))
	defer server.Close()

	client := NewClient(ctx, ClientConfig{
		Token: "test-token",
		Owner: "grafana",
		Repo:  "alloy",
	})
	client.graphqlURL = server.URL

	oid, err := client.CreateCommitOnBranch(ctx, CreateCommitOnBranchParams{
		Branch:          "release-please--branches--main",
		ExpectedHeadOID: "base-sha",
		Headline:        "chore: Regenerate collector distro",
		Body:            "Generated after release-please updated versions.",
		Additions: []FileAddition{{
			Path:     "go.mod",
			Contents: []byte("module github.com/grafana/alloy\n"),
		}},
		Deletions: []string{"collector/removed.go"},
	})
	if err != nil {
		t.Fatalf("CreateCommitOnBranch returned error: %v", err)
	}
	if oid != "new-commit-sha" {
		t.Fatalf("expected oid new-commit-sha, got %q", oid)
	}

	assertCreateCommitRequest(t, request)
}

func handleCreateCommitMutation(t *testing.T, request *graphQLRequest, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	if r.Method != http.MethodPost {
		t.Fatalf("expected POST, got %s", r.Method)
	}
	if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Fatalf("expected bearer token, got %q", got)
	}

	if err := json.NewDecoder(r.Body).Decode(request); err != nil {
		t.Fatalf("decoding request: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"data":{"createCommitOnBranch":{"commit":{"oid":"new-commit-sha"}}}}`))
}

func assertCreateCommitRequest(t *testing.T, request *graphQLRequest) {
	t.Helper()

	if !strings.Contains(request.Query, "createCommitOnBranch") {
		t.Fatalf("expected createCommitOnBranch mutation, got %q", request.Query)
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshaling captured request: %v", err)
	}
	for _, forbidden := range []string{"author", "committer", "signature"} {
		if strings.Contains(string(requestJSON), forbidden) {
			t.Fatalf("request must not include custom %s information: %s", forbidden, requestJSON)
		}
	}

	input := request.Variables["input"].(map[string]any)
	assertCreateCommitBranch(t, input)
	assertCreateCommitMessage(t, input)
	assertCreateCommitFileChanges(t, input)
}

func assertCreateCommitBranch(t *testing.T, input map[string]any) {
	t.Helper()

	branch := input["branch"].(map[string]any)
	if branch["repositoryNameWithOwner"] != "grafana/alloy" {
		t.Fatalf("expected repositoryNameWithOwner grafana/alloy, got %v", branch["repositoryNameWithOwner"])
	}
	if branch["branchName"] != "release-please--branches--main" {
		t.Fatalf("expected release-please branch, got %v", branch["branchName"])
	}
	if input["expectedHeadOid"] != "base-sha" {
		t.Fatalf("expected base-sha, got %v", input["expectedHeadOid"])
	}
}

func assertCreateCommitMessage(t *testing.T, input map[string]any) {
	t.Helper()

	message := input["message"].(map[string]any)
	if message["headline"] != "chore: Regenerate collector distro" {
		t.Fatalf("unexpected headline: %v", message["headline"])
	}
	if message["body"] != "Generated after release-please updated versions." {
		t.Fatalf("unexpected body: %v", message["body"])
	}
}

func assertCreateCommitFileChanges(t *testing.T, input map[string]any) {
	t.Helper()

	fileChanges := input["fileChanges"].(map[string]any)
	additions := fileChanges["additions"].([]any)
	addition := additions[0].(map[string]any)
	if addition["path"] != "go.mod" {
		t.Fatalf("unexpected addition path: %v", addition["path"])
	}
	if addition["contents"] != "bW9kdWxlIGdpdGh1Yi5jb20vZ3JhZmFuYS9hbGxveQo=" {
		t.Fatalf("unexpected base64 contents: %v", addition["contents"])
	}

	deletions := fileChanges["deletions"].([]any)
	deletion := deletions[0].(map[string]any)
	if deletion["path"] != "collector/removed.go" {
		t.Fatalf("unexpected deletion path: %v", deletion["path"])
	}
}
