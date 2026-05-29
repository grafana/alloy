package aireview

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/openai/openai-go/v3"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

type aiReviewFlags struct {
	Model      string
	PromptFile string
	Marker     string
	Slug       string
	PRNumber   int
	NoComment  bool
}

func Command() *cobra.Command {
	var args aiReviewFlags

	cmd := &cobra.Command{
		Use:   "aireview",
		Short: "Analyze a PR diff with OpenAI and post the result as a PR comment",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), args)
		},
	}

	cmd.Flags().StringVar(&args.Model, "model", openai.ChatModelGPT5, "OpenAI model to use")
	cmd.Flags().StringVar(&args.PromptFile, "prompt-file", "", "Path to file containing AI prompt/rules")
	cmd.Flags().StringVar(&args.Marker, "marker", "<!-- ai-review -->", "HTML comment marker to identify bot comments")
	cmd.Flags().StringVar(&args.Slug, "slug", "", "Repository slug (owner/repo) - required for GitHub mode")
	cmd.Flags().IntVar(&args.PRNumber, "pr-number", 0, "Pull request number - required for GitHub mode")
	cmd.Flags().BoolVar(&args.NoComment, "no-comment", false, "Fetch PR from GitHub but output to stdout instead of posting comment")

	return cmd
}

func run(ctx context.Context, args aiReviewFlags) error {
	if args.PromptFile == "" {
		return fmt.Errorf("--prompt-file is required")
	}

	if os.Getenv("OPENAI_API_KEY") == "" {
		return fmt.Errorf("OPENAI_API_KEY environment variable is required")
	}

	githubMode := args.Slug != "" && args.PRNumber > 0
	if !githubMode && (args.Slug != "" || args.PRNumber > 0) {
		return fmt.Errorf("both --slug and --pr-number must be provided together for GitHub mode, or neither for stdin mode")
	}

	var diffContent string

	if githubMode {
		githubToken := os.Getenv("GITHUB_TOKEN")
		if githubToken == "" {
			return fmt.Errorf("GITHUB_TOKEN environment variable is required in GitHub mode")
		}

		parts := strings.Split(args.Slug, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid --slug format: %s (expected: owner/repo)", args.Slug)
		}
		owner, repoName := parts[0], parts[1]

		log.Printf("Fetching PR diff for %s/%s#%d", owner, repoName, args.PRNumber)

		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
		tc := oauth2.NewClient(ctx, ts)
		githubClient := github.NewClient(tc)

		var err error
		diffContent, err = getPRDiff(ctx, githubClient, owner, repoName, args.PRNumber)
		if err != nil {
			return fmt.Errorf("failed to get PR diff: %w", err)
		}
	} else {
		log.Printf("Reading diff from stdin")

		diffBytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read diff from stdin: %w", err)
		}
		diffContent = string(diffBytes)
		if diffContent == "" {
			return fmt.Errorf("no diff content provided on stdin")
		}
	}

	log.Printf("Reading prompt file %s", args.PromptFile)
	promptContent, err := os.ReadFile(args.PromptFile)
	if err != nil {
		return fmt.Errorf("failed to read prompt file: %w", err)
	}
	prompt := string(promptContent)

	log.Printf("Calling OpenAI API with model %s", args.Model)
	openaiClient := openai.NewClient()
	aiResponse, err := analyzeWithAI(ctx, openaiClient, args.Model, prompt, diffContent)
	if err != nil {
		return fmt.Errorf("failed to analyze with AI: %w", err)
	}

	if !githubMode || args.NoComment {
		fmt.Println(aiResponse)
		return nil
	}

	githubToken := os.Getenv("GITHUB_TOKEN")
	parts := strings.Split(args.Slug, "/")
	owner, repoName := parts[0], parts[1]

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
	tc := oauth2.NewClient(ctx, ts)
	githubClient := github.NewClient(tc)

	commentBody := fmt.Sprintf("%s\n\n%s", args.Marker, aiResponse)

	if err := putComment(ctx, githubClient, owner, repoName, args.PRNumber, args.Marker, commentBody); err != nil {
		return fmt.Errorf("failed to post comment: %w", err)
	}

	log.Printf("Successfully posted AI review comment")
	return nil
}

