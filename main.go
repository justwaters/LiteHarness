package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	cfg := LoadConfig()

	// Parse simple CLI flags
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider":
			if i+1 < len(args) {
				cfg.Provider = args[i+1]
				i++
			}
		case "--model":
			if i+1 < len(args) {
				cfg.Model = args[i+1]
				i++
			}
		case "--base-url":
			if i+1 < len(args) {
				cfg.BaseURL = args[i+1]
				i++
			}
		case "--api-key":
			if i+1 < len(args) {
				cfg.APIKey = args[i+1]
				i++
			}
		case "--help", "-h":
			printHelp()
			return
		}
	}

	provider, err := NewProvider(cfg)
	if err != nil {
		PrintError("%s", err)
		os.Exit(1)
	}

	registry := NewRegistry()

	fmt.Println(c(ColorBold, "LiteHarness"))
	fmt.Printf("Provider: %s  Model: %s\n", c(ColorCyan, provider.Name()), c(ColorCyan, cfg.Model))
	fmt.Println(c(ColorDim, "Type your message, or /help for commands. Ctrl-D to exit.\n"))

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // allow up to 1MB lines

	messages := []Message{}

	for {
		fmt.Print(c(ColorBold+ColorGreen, "> "))
		if !scanner.Scan() {
			fmt.Println() // ^D
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Slash commands
		if strings.HasPrefix(line, "/") {
			newProvider, err := handleCommand(line, cfg, provider, &messages)
			if err != nil {
				PrintError("%s", err)
			}
			if newProvider != nil {
				provider = newProvider
			}
			continue
		}

		// Add user message
		messages = append(messages, Message{Role: "user", Content: line})
		PrintUser(line)

		// Agent loop: keep going until model gives a final answer (no tool calls)
		for {
			spin := StartSpinner()

			req := ChatRequest{
				Model:       cfg.Model,
				System:      cfg.SystemPrompt,
				Messages:    messages,
				Tools:       registry.List(),
				Temperature: 0.7,
			}

			chunkCh, err := provider.Chat(context.Background(), req)
			if err != nil {
				spin.Stop()
				PrintError("%s", err)
				break
			}

			var assistantContent strings.Builder
			var toolCalls []ToolCall

			for chunk := range chunkCh {
				switch chunk.Type {
				case ChunkText:
					spin.Stop()
					PrintModel(os.Stdout, chunk.Content)
					assistantContent.WriteString(chunk.Content)
				case ChunkToolCall:
					spin.Stop()
					toolCalls = append(toolCalls, chunk.ToolCalls...)
				case ChunkDone:
					spin.Stop()
				case ChunkError:
					spin.Stop()
					PrintError("%s", chunk.Err)
				}
			}
			fmt.Println() // newline after streaming

			assistantMsg := Message{Role: "assistant", Content: assistantContent.String()}
			messages = append(messages, assistantMsg)

			if len(toolCalls) == 0 {
				// Final answer received
				break
			}

			// Execute tool calls and feed results back
			for _, tc := range toolCalls {
				PrintToolTrace("%s(%s)", tc.Name, tc.Arguments)
				result, err := registry.Call(tc.Name, tc.Arguments)
				if err != nil {
					result = "Error: " + err.Error()
				}
				PrintToolResult("%s", result)

				// Tool results as user messages work for both OpenAI-compatible and Gemini
				messages = append(messages, Message{
					Role:    "user",
					Content: fmt.Sprintf("[tool result for %s] %s", tc.Name, result),
				})
			}
			// Loop continues: model will process tool results and either
			// make more tool calls or give a final answer.
		}
	}
}

func handleCommand(line string, cfg *Config, provider Provider, messages *[]Message) (Provider, error) {
	cmd := strings.TrimSpace(strings.TrimPrefix(line, "/"))
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil, nil
	}

	switch parts[0] {
	case "help", "h":
		printHelp()
	case "exit", "quit", "q":
		fmt.Println("Goodbye!")
		os.Exit(0)
	case "clear", "cls":
		*messages = nil
		fmt.Print("\033[2J\033[H")
	case "model":
		if len(parts) > 1 {
			cfg.Model = parts[1]
			fmt.Printf("Model set to: %s\n", c(ColorCyan, cfg.Model))
		} else {
			fmt.Printf("Current model: %s\n", c(ColorCyan, cfg.Model))
		}
	case "provider":
		if len(parts) > 1 {
			cfg.Provider = parts[1]
			p, err := NewProvider(cfg)
			if err != nil {
				return nil, err
			}
			return p, nil
		} else {
			fmt.Printf("Current provider: %s\n", c(ColorCyan, cfg.Provider))
		}
	case "tools":
		registry := NewRegistry()
		for _, t := range registry.List() {
			params, _ := json.Marshal(t.Parameters)
			fmt.Printf("  %s%s%s - %s\n", c(ColorBold, t.Name), c(ColorDim, "("), c(ColorDim, string(params)), t.Description)
		}
	case "system":
		if len(parts) > 1 {
			cfg.SystemPrompt = strings.Join(parts[1:], " ")
			fmt.Printf("System prompt set.\n")
		} else {
			fmt.Printf("System prompt: %s\n", c(ColorCyan, cfg.SystemPrompt))
		}
	case "save":
		if err := SaveConfig(cfg); err != nil {
			PrintError("failed to save config: %s", err)
		} else {
			home, _ := os.UserHomeDir()
			fmt.Printf("Config saved to %s/%s\n", home, defaultConfigFile)
		}
	default:
		return nil, fmt.Errorf("unknown command: /%s. Type /help for commands.", parts[0])
	}
	return nil, nil
}

func printHelp() {
	fmt.Println(c(ColorBold, "LiteHarness - AI coding harness"))
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  /help              Show this help")
	fmt.Println("  /exit              Exit LiteHarness")
	fmt.Println("  /clear             Clear conversation history")
	fmt.Println("  /model [name]      Get or set the model")
	fmt.Println("  /provider [name]   Get or set the provider (openai, gemini)")
	fmt.Println("  /system [text]     Get or set the system prompt")
	fmt.Println("  /tools             List available tools")
	fmt.Println("  /save              Save config to ~/.liteharnessrc")
	fmt.Println()
	fmt.Println("Environment variables:")
	fmt.Println("  LITEHARNESS_PROVIDER   openai | gemini")
	fmt.Println("  LITEHARNESS_BASE_URL   API base URL (for OpenAI-compatible)")
	fmt.Println("  LITEHARNESS_MODEL      Model name")
	fmt.Println("  LITEHARNESS_API_KEY    API key")
	fmt.Println("  LITEHARNESS_SYSTEM_PROMPT  System prompt")
	fmt.Println()
	fmt.Println("Config file: ~/.liteharnessrc (key=value format)")
	fmt.Println()
	fmt.Println("Tools: read_file, write_file, edit_file, ls, grep, glob, run_command, todo_write")
}
