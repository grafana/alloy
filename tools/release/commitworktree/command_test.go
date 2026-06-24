package commitworktree

import "testing"

func TestSplitCommitMessage(t *testing.T) {
	headline, body := splitCommitMessage("chore: Regenerate collector distro\n\nGenerated after release-please updated versions.\n")

	if headline != "chore: Regenerate collector distro" {
		t.Fatalf("unexpected headline: %q", headline)
	}
	if body != "Generated after release-please updated versions." {
		t.Fatalf("unexpected body: %q", body)
	}
}
