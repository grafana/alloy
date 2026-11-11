package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetTagSHA(t *testing.T) {
	testCases := []struct {
		name      string
		refBody   string
		tagBody   string
		wantSHA   string
		wantError string
	}{
		{name: "lightweight tag", refBody: `{"object":{"type":"commit","sha":"tag-commit"}}`, wantSHA: "tag-commit"},
		{name: "annotated tag", refBody: `{"object":{"type":"tag","sha":"tag-object"}}`, tagBody: `{"object":{"type":"commit","sha":"tag-commit"}}`, wantSHA: "tag-commit"},
		{name: "missing tag despite matching branch", wantError: "resolving tag helm-chart/1.2.3"},
		{name: "missing annotated tag object", refBody: `{"object":{"type":"tag","sha":"tag-object"}}`, wantError: "dereferencing annotated tag helm-chart/1.2.3"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(tagTestHandler(testCase.refBody, testCase.tagBody))
			defer server.Close()

			client := NewClient(context.Background(), ClientConfig{Owner: "grafana", Repo: "alloy"})
			baseURL, err := url.Parse(server.URL + "/")
			require.NoError(t, err)
			client.API().BaseURL = baseURL

			sha, err := client.GetTagSHA(context.Background(), "helm-chart/1.2.3")
			if testCase.wantError != "" {
				require.ErrorContains(t, err, testCase.wantError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, testCase.wantSHA, sha)
		})
	}
}

func tagTestHandler(refBody, tagBody string) http.Handler {
	responses := map[string]string{
		"/repos/grafana/alloy/git/ref/tags/helm-chart/1.2.3": refBody,
		"/repos/grafana/alloy/git/tags/tag-object":           tagBody,
		"/repos/grafana/alloy/branches/helm-chart/1.2.3":     `{"commit":{"sha":"branch-commit"}}`,
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body := responses[request.URL.Path]
		if body == "" {
			http.NotFound(response, request)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(body))
	})
}

func TestGetRefSHAPreservesAnnotatedTagError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/repos/grafana/alloy/git/ref/tags/v1.2.3" {
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"object":{"type":"tag","sha":"tag-object"}}`))
			return
		}
		http.NotFound(response, request)
	}))
	defer server.Close()

	client := NewClient(context.Background(), ClientConfig{Owner: "grafana", Repo: "alloy"})
	baseURL, err := url.Parse(server.URL + "/")
	require.NoError(t, err)
	client.API().BaseURL = baseURL

	_, err = client.GetRefSHA(context.Background(), "v1.2.3")
	require.ErrorContains(t, err, "dereferencing annotated tag v1.2.3")
}

func TestGetRefSHAResolutionOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/repos/grafana/alloy/branches/collision":
			_, _ = response.Write([]byte(`{"commit":{"sha":"branch-commit"}}`))
		case "/repos/grafana/alloy/git/ref/tags/collision", "/repos/grafana/alloy/git/ref/tags/tag-only":
			_, _ = response.Write([]byte(`{"object":{"type":"commit","sha":"tag-commit"}}`))
		case "/repos/grafana/alloy/commits/commit-only":
			_, _ = response.Write([]byte(`{"sha":"commit-sha"}`))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	client := NewClient(context.Background(), ClientConfig{Owner: "grafana", Repo: "alloy"})
	baseURL, err := url.Parse(server.URL + "/")
	require.NoError(t, err)
	client.API().BaseURL = baseURL

	for ref, expectedSHA := range map[string]string{
		"collision":   "branch-commit",
		"tag-only":    "tag-commit",
		"commit-only": "commit-sha",
	} {
		t.Run(ref, func(t *testing.T) {
			sha, err := client.GetRefSHA(context.Background(), ref)
			require.NoError(t, err)
			require.Equal(t, expectedSHA, sha)
		})
	}
}
