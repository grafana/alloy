package main

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v3"
)

// analyzeWithAI sends the prompt and diff to OpenAI and returns the response
func analyzeWithAI(ctx context.Context, client openai.Client, model, prompt, diff string) (string, error) {
	// Construct the full message
	fullPrompt := fmt.Sprintf("%s\n\n---\n\nPull Request Diff:\n\n```diff\n%s\n```", prompt, diff)

	completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(fullPrompt),
		},
		Model: model,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return completion.Choices[0].Message.Content, nil
}
