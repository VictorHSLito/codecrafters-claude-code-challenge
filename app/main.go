package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type ReadArgs struct {
	FilePath string `json:"file_path"`
}

type WriteArgs struct {
	FilePath string `json:"file_path"`
	Content string `json:"content"`
}

type BashArgs struct {
	Command string `json:"command"`
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

	for range 5 {
		resp, err := client.Chat.Completions.New(
			context.Background(),
			openai.ChatCompletionNewParams{
				Model: "anthropic/claude-haiku-4.5",
				Messages: messages,
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
					openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
						Name:       "Write",
						Description: openai.String("Write content to a file"),
						Parameters: openai.FunctionParameters{
							"type": "object",
							"properties": map[string]any{
								"file_path": map[string]any{
									"type": "string",
									"description": "The path to the file to write to",
								},
								"content": map[string]any{
									"type": "string",
									"description": "The content to write to the file",
								},
							},
							"required": []string{"file_path", "content"},
						},
					}),
					openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
						Name:      "Bash",
						Description: openai.String("Execute a shell command"),
						Parameters: openai.FunctionParameters{
							"type": "object",
							"properties": map[string]any{
								"command": map[string]any{
									"type": "string",
									"description": "The command to execute",
								},
							},
							"required": []string{"command"},
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

		for _, toolCall := range message.ToolCalls {
			assistantToolCalls = append(assistantToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
					ID: toolCall.ID,
					Type: toolCall.AsFunction().Type,
					Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
						Name: toolCall.Function.Name,
						Arguments: toolCall.Function.Arguments,
					},
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
			var result string
			var executed bool

			if toolCall.Function.Name == "Read" {
				result = ExecuteReadTool(&toolCall)
				executed = true
			}

			if toolCall.Function.Name == "Write" {
				result = ExecuteWriteTool(&toolCall)
				executed = true
			}

			if toolCall.Function.Name == "Bash" {
				result = ExecuteBashTool(&toolCall)
				executed = true
			}

			if executed {
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

func ExecuteReadTool(toolCall *openai.ChatCompletionMessageToolCallUnion) string {
	if toolCall.Function.Name == "Read" {
		var toolArgs ReadArgs

		err := json.Unmarshal([]byte(toolCall.Function.Arguments), &toolArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing args: %v\n", err)
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

func ExecuteWriteTool(toolCall *openai.ChatCompletionMessageToolCallUnion) string {
	var toolArgs WriteArgs

	err := json.Unmarshal([]byte(toolCall.Function.Arguments), &toolArgs)

	if err != nil {
        return fmt.Sprintf("Error parsing args: %v", err)
    }

    err = os.WriteFile(toolArgs.FilePath, []byte(toolArgs.Content), 0644)
    if err != nil {
        return fmt.Sprintf("Error writing file: %v", err)
    }

    return "File written successfully"
}

func ExecuteBashTool(toolCall *openai.ChatCompletionMessageToolCallUnion) string {
	var toolArgs BashArgs

	err := json.Unmarshal([]byte(toolCall.Function.Arguments), &toolArgs)

	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing args: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command("bash", "-c", toolArgs.Command)

	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Sprintf("Error executing command: %v", err)
	}

	return string(output)
}