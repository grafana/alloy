# AI Review Tool

Uses OpenAI to analyze PR diffs and post review comments on GitHub.

## Usage

**GitHub Mode** - Fetch diff from PR and post comment:
```bash
go run ./tools/ai-review/ \
  --slug="owner/repo" \
  --pr-number=123 \
  --prompt-file=".github/ai-review-prompts/dependency-review.md" \
  --marker="<!-- ai-review -->"
```

**GitHub Mode (no comment)** - Fetch diff from PR but output to stdout (testing):
```bash
go run ./tools/ai-review/ \
  --slug="owner/repo" \
  --pr-number=123 \
  --prompt-file=".github/ai-review-prompts/dependency-review.md" \
  --no-comment
```

**Stdin Mode** - Read diff from stdin and output to stdout:
```bash
git diff main | go run ./tools/ai-review/ \
  --prompt-file=".github/ai-review-prompts/dependency-review.md"
```

## Setup

1. Add `OPENAI_API_KEY` secret to your repository
2. Create prompt files in `.github/ai-review-prompts/`
3. Add workflow step with unique `--marker` and `--prompt-file` for each type of review

See `.github/workflows/ai-dependency-review.yml` for a working example.

**Note**: Each workflow should use a unique marker (e.g., `<!-- ai-deps -->`, `<!-- ai-security -->`) to maintain separate comments on the PR.
