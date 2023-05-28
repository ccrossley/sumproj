package main

import (
	"context"
	"encoding/json"
	"fmt"
	openai "github.com/sashabaranov/go-openai"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	requiredArgs        = 3
	directoryPathArg    = 1
	openAIKeyFileArg    = 2
	maxCompletionTokens = 60
	temperature         = 0.5
	keyPlaceholder      = "<INSERT OPENAPI KEY HERE>"
)

var (
	apiKey       string
	openAIClient *openai.Client
)

func main() {
	if len(os.Args) != requiredArgs {
		panic("Incorrect number of arguments. Usage: program <directory_path> <openai_key_file>")
	}
	path := os.Args[directoryPathArg]    // Directory path argument
	keyFile := os.Args[openAIKeyFileArg] // OpenAI Key File argument

	var err error
	apiKey, err = loadAPIKey(keyFile)
	if err != nil {
		panic(err)
	}

	openAIClient = openai.NewClient(apiKey)

	err = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".fish" && ext != ".cs" {
			return nil
		}

		processFile(path)

		return nil
	})
	if err != nil {
		panic(err)
	}
}

func loadAPIKey(keyFile string) (string, error) {
	data, err := ioutil.ReadFile(keyFile)
	if err != nil {
		if os.IsNotExist(err) {
			err = ioutil.WriteFile(keyFile, []byte(`{"key":"`+keyPlaceholder+`"}`), 0600)
			return keyPlaceholder, err
		}
		return "", err
	}

	var keyData struct {
		Key string `json:"key"`
	}
	err = json.Unmarshal(data, &keyData)
	return keyData.Key, err
}

func processFile(filePath string) {
	functionRegex := regexp.MustCompile(`(func|function|class|struct|interface|abstract)\s+([a-zA-Z0-9_]+)`)
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	f, err := os.OpenFile("code_prompt.txt", os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	matches := functionRegex.FindAllStringSubmatch(string(data), -1)
	for _, match := range matches {
		summary := summarizeFunction(match[0])
		if _, err = f.WriteString(match[0] + "\n// " + summary + "\n\n"); err != nil {
			panic(err)
		}
	}
}

func summarizeFunction(functionCode string) string {
	ctx := context.Background()

	prompt := functionCode + "\n# Summarize the above function in one sentence."

	resp, err := openAIClient.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return ""
	}

	return resp.Choices[0].Message.Content
}
