package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/openai/openai-go/v3"
	"golang.org/x/oauth2"
)

func main() {
	// Parse command-line flags
	var (
		model      = flag.String("model", "gpt-5", "OpenAI model to use")
		promptFile = flag.String("prompt-file", "", "Path to file containing AI prompt/rules")
		marker     = flag.String("marker", "<!-- ai-review -->", "HTML comment marker to identify bot comments")
		slug       = flag.String("slug", "", "Repository slug (owner/repo) - required for GitHub mode")
		prNumber   = flag.Int("pr-number", 0, "Pull request number - required for GitHub mode")
		noComment  = flag.Bool("no-comment", false, "Fetch PR from GitHub but output to stdout instead of posting comment")
	)
	flag.Parse()

	// Validate required flags
	if *promptFile == "" {
		log.Fatal("--prompt-file is required")
	}

	// Get required environment variables
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	ctx := context.Background()

	var diffContent string
	githubMode := *slug != "" && *prNumber > 0

	// Mode 1: GitHub mode - fetch diff from GitHub
	if githubMode {
		if *slug == "" {
			log.Fatal("--slug is required when using --pr-number")
		}
		if *prNumber == 0 {
			log.Fatal("--pr-number is required when using --slug")
		}

		githubToken := os.Getenv("GITHUB_TOKEN")
		if githubToken == "" {
			log.Fatal("GITHUB_TOKEN environment variable is required in GitHub mode")
		}

		// Parse repository owner and name
		parts := strings.Split(*slug, "/")
		if len(parts) != 2 {
			log.Fatalf("Invalid --repo format: %s (expected: owner/repo)", *slug)
		}
		owner, repoName := parts[0], parts[1]

		log.Printf("Fetching PR diff for %s/%s#%d", owner, repoName, *prNumber)

		// Initialize GitHub client
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
		tc := oauth2.NewClient(ctx, ts)
		githubClient := github.NewClient(tc)

		// Get PR diff
		var err error
		diffContent, err = getPRDiff(ctx, githubClient, owner, repoName, *prNumber)
		if err != nil {
			log.Fatalf("Failed to get PR diff: %v", err)
		}
	} else {
		log.Printf("Reading diff from stdin")

		// Mode 2: Stdin mode - read diff from stdin
		if *slug != "" || *prNumber > 0 {
			log.Fatal("Both --slug and --pr-number must be provided together for GitHub mode, or neither for stdin mode")
		}

		diffBytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("Failed to read diff from stdin: %v", err)
		}
		diffContent = string(diffBytes)
		if diffContent == "" {
			log.Fatal("No diff content provided on stdin")
		}
	}

	log.Printf("Reading prompt file %s", *promptFile)

	// Read prompt file
	promptContent, err := os.ReadFile(*promptFile)
	if err != nil {
		log.Fatalf("Failed to read prompt file: %v", err)
	}
	prompt := string(promptContent)

	log.Printf("Calling OpenAI API with model %s", *model)

	// Call OpenAI API
	openaiClient := openai.NewClient()
	aiResponse, err := analyzeWithAI(ctx, openaiClient, *model, prompt, diffContent)
	if err != nil {
		log.Fatalf("Failed to analyze with AI: %v", err)
	}

	// If in stdin mode or --no-comment flag is set, just output to stdout
	if !githubMode || *noComment {
		fmt.Println(aiResponse)
		return
	}

	// Otherwise, post/update comment on GitHub
	githubToken := os.Getenv("GITHUB_TOKEN")
	parts := strings.Split(*slug, "/")
	owner, repoName := parts[0], parts[1]

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
	tc := oauth2.NewClient(ctx, ts)
	githubClient := github.NewClient(tc)

	// Format the comment with marker
	commentBody := fmt.Sprintf("%s\n\n%s", *marker, aiResponse)

	// Post or update comment on PR
	if err := putComment(ctx, githubClient, owner, repoName, *prNumber, *marker, commentBody); err != nil {
		log.Fatalf("Failed to post comment: %v", err)
	}

	log.Printf("Successfully posted AI review comment")
}
