package createrc

import (
	"testing"

	"github.com/google/go-github/v57/github"
)

func pr(number int, headRef string, labels ...string) *github.PullRequest {
	p := &github.PullRequest{
		Number: github.Int(number),
		Head:   &github.PullRequestBranch{Ref: github.String(headRef)},
	}
	for _, l := range labels {
		p.Labels = append(p.Labels, &github.Label{Name: github.String(l)})
	}
	return p
}

func TestSelectMainModuleReleasePR(t *testing.T) {
	tests := []struct {
		name       string
		prs        []*github.PullRequest
		baseBranch string
		wantNumber int
		wantErr    bool
	}{
		{
			// The component PR carries "autorelease: pending" and sorts first, but
			// only the main module's PR (no --components-- segment) should win.
			name:       "skips syntax component PR ordered before main release PR",
			baseBranch: "main",
			prs: []*github.PullRequest{
				pr(6266, "release-please--branches--main--components--syntax", "autorelease: pending"),
				pr(6111, "release-please--branches--main", "autorelease: pending"),
			},
			wantNumber: 6111,
		},
		{
			name:       "selects release PR on a release branch",
			baseBranch: "release/v1.16",
			prs: []*github.PullRequest{
				pr(7000, "release-please--branches--release/v1.16", "autorelease: pending"),
			},
			wantNumber: 7000,
		},
		{
			name:       "tolerates label without space after colon",
			baseBranch: "main",
			prs: []*github.PullRequest{
				pr(6111, "release-please--branches--main", "autorelease:pending"),
			},
			wantNumber: 6111,
		},
		{
			// An already-tagged release-please PR drops the pending label.
			name:       "ignores main release PR missing the pending label",
			baseBranch: "main",
			prs: []*github.PullRequest{
				pr(6111, "release-please--branches--main", "autorelease: tagged"),
			},
			wantErr: true,
		},
		{
			name:       "errors when only a component PR is open",
			baseBranch: "main",
			prs: []*github.PullRequest{
				pr(6266, "release-please--branches--main--components--syntax", "autorelease: pending"),
			},
			wantErr: true,
		},
		{
			// A non-release-please branch that happens to carry the label is ignored.
			name:       "ignores a non-release-please branch with the pending label",
			baseBranch: "main",
			prs: []*github.PullRequest{
				pr(9000, "kgeckhart/some-feature", "autorelease: pending"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectMainModuleReleasePR(tt.prs, tt.baseBranch)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got PR #%d", got.GetNumber())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.GetNumber() != tt.wantNumber {
				t.Errorf("selected PR #%d, want #%d", got.GetNumber(), tt.wantNumber)
			}
		})
	}
}
