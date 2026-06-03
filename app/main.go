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

	var messages []openai.ChatCompletionMessageParamUnion

	messages = append(messages, openai.ChatCompletionMessageParamUnion{
		OfUser: &openai.ChatCompletionUserMessageParam{
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfString: openai.String(prompt),
			},
		},
	})

	for i := 0; i < 5; i++ {
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
			fmt.Printf("error %v\n", err)
			os.Exit(1)
		}

		message := resp.Choices[0].Message

		if len(message.ToolCalls) == 0 {
			fmt.Print(message.Content)
			os.Exit(0)
		}


		var assistantToolCalls []openai.ChatCompletionMessageToolCallUnionParam

		for _, toolCall := range assistantToolCalls {
			assistantToolCalls = append(assistantToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
					ID: *toolCall.GetID(),
					Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
						Name: toolCall.GetFunction().Name,
						Arguments: toolCall.GetFunction().Arguments,
					},
					Type: toolCall.OfFunction.Type,
				},
			})
		}

		messages = append(messages, openai.ChatCompletionMessageParamUnion{
			OfAssistant: &openai.ChatCompletionAssistantMessageParam {
				Content: openai.ChatCompletionAssistantMessageParamContentUnion{
					OfString: openai.String(message.Content),
				},
				ToolCalls: assistantToolCalls,
			},
		})

		for _, toolCall := range message.ToolCalls {
			if toolCall.Function.Name == "Read" {
				result := ExecuteTool(&toolCall)

				toolMessage := openai.ChatCompletionMessageParamUnion{
					OfTool: &openai.ChatCompletionToolMessageParam{
						ToolCallID: toolCall.ID,
						Content: openai.ChatCompletionToolMessageParamContentUnion{
							OfString: openai.String(result),
						},
					},
				}

				messages = append(messages, toolMessage)
			}
		}
	}

	fmt.Println("Maximum tool call rounds reached")

}

func ExecuteTool(toolCall *openai.ChatCompletionMessageToolCallUnion) string {
	if toolCall.Function.Name == "Read" {
		var toolArgs ReadArgs

		err := json.Unmarshal([]byte(toolCall.Function.Arguments), &toolArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error parsing args: %v\n", err)
			os.Exit(1)
		}

		fileContent, err := os.ReadFile(toolArgs.FilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
			os.Exit(1)
		}

		return string(fileContent)
	}

	return ""
}
