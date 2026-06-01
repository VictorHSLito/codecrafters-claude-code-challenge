package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type ReadArgs struct {
	FilePath string `json:"file_path"`
}

func main() {
	var prompt string
	flag.StringVar(&prompt, "p", "", "Prompt to send to LLM")
	flag.Parse()

	if prompt == "" {
		panic("Prompt must not be empty")
	}

	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434/v1"
	}

	client := openai.NewClient(option.WithBaseURL(baseURL))
	resp, err := client.Chat.Completions.New(
		context.Background(),
		openai.ChatCompletionNewParams{
			Model: "anthropic/claude-haiku-4.5",
			Messages: []openai.ChatCompletionMessageParamUnion{
				{
					OfUser: &openai.ChatCompletionUserMessageParam{
						Content: openai.ChatCompletionUserMessageParamContentUnion{
							OfString: openai.String(prompt),
						},
					},
				},
			},
			Tools: []openai.ChatCompletionToolUnionParam{
				openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
					Name:        "Read",
					Description: openai.String("Read and return the contents of a file"),
					Parameters: openai.FunctionParameters{
						"type": "object",
						"properties": map[string]any{
							"file_path": map[string]any{
								"type":        "string",
								"description": "The path to the file to read",
							},
						},
						"required": []string{"file_path"},
					},
				}),
			},
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(resp.Choices) == 0 {
		panic("No choices in response")
	}

	if len(resp.Choices) > 0 && len(resp.Choices[0].Message.ToolCalls) > 0 {
		toolCall := resp.Choices[0].Message.ToolCalls[0]

		if toolCall.Function.Name == "Read" {
			var toolArgs ReadArgs

			err = json.Unmarshal([]byte(toolCall.Function.Arguments), &toolArgs)

			if err != nil {
				fmt.Fprintf(os.Stderr, "error parsing args: %v\n", err)
                os.Exit(1)
			}

			fileContent, err := os.ReadFile(toolArgs.FilePath)
            if err != nil {
                fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
                os.Exit(1)
            }

			fmt.Print(string(fileContent))
		}
	} else if len(resp.Choices) > 0 {
		fmt.Print(resp.Choices[0].Message.Content)
	}
}