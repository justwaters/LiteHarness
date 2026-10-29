package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Tool represents an available tool for the model.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatChunk is a single streaming chunk from the provider.
type ChatChunk struct {
	Type      ChunkType
	Content   string
	ToolCalls []ToolCall
	Err       error
}

// ChunkType identifies the kind of chunk.
type ChunkType int

const (
	ChunkText ChunkType = iota
	ChunkToolCall
	ChunkDone
	ChunkError
)

// Provider is the LLM provider interface.
type Provider interface {
	Name() string
	Chat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
}

// ChatRequest is sent to a provider.
type ChatRequest struct {
	Model       string
	System      string
	Messages    []Message
	Tools       []Tool
	Temperature float64
}

// OpenAIProvider implements the OpenAI-compatible chat completions API.
// Works with Ollama, OpenRouter, NVIDIA NIM, OpenAI, and any OpenAI-compatible endpoint.
type OpenAIProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewOpenAIProvider(baseURL, apiKey string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIProvider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	ch := make(chan ChatChunk, 8)

	go func() {
		defer close(ch)

		// Build messages array
		messages := []map[string]interface{}{}
		if req.System != "" {
			messages = append(messages, map[string]interface{}{
				"role":    "system",
				"content": req.System,
			})
		}
		for _, m := range req.Messages {
			messages = append(messages, map[string]interface{}{
				"role":    m.Role,
				"content": m.Content,
			})
		}

		body := map[string]interface{}{
			"model":       req.Model,
			"messages":    messages,
			"stream":      true,
			"temperature": req.Temperature,
		}
		if len(req.Tools) > 0 {
			tools := []map[string]interface{}{}
			for _, t := range req.Tools {
				tools = append(tools, map[string]interface{}{
					"type": "function",
					"function": map[string]interface{}{
						"name":        t.Name,
						"description": t.Description,
						"parameters":  t.Parameters,
					},
				})
			}
			body["tools"] = tools
		}

		payload, err := json.Marshal(body)
		if err != nil {
			ch <- ChatChunk{Type: ChunkError, Err: fmt.Errorf("marshal request: %w", err)}
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(payload))
		if err != nil {
			ch <- ChatChunk{Type: ChunkError, Err: fmt.Errorf("create request: %w", err)}
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

		resp, err := p.client.Do(httpReq)
		if err != nil {
			ch <- ChatChunk{Type: ChunkError, Err: fmt.Errorf("request: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			ch <- ChatChunk{Type: ChunkError, Err: fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))}
			return
		}

		reader := bufio.NewReaderSize(resp.Body, 4096)
		for {
			line, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				ch <- ChatChunk{Type: ChunkError, Err: fmt.Errorf("read stream: %w", err)}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, "data: ") {
				if err == io.EOF {
					break
				}
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			data = strings.TrimSpace(data)
			if data == "[DONE]" {
				ch <- ChatChunk{Type: ChunkDone}
				if err == io.EOF {
					break
				}
				continue
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Role      string `json:"role"`
						Content   string `json:"content"`
						ToolCalls []struct {
							Index   int `json:"index"`
							ID      string `json:"id"`
							Type    string `json:"type"`
							Function struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							} `json:"function"`
						} `json:"tool_calls"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				// Skip malformed lines
				if err == io.EOF {
					break
				}
				continue
			}

			if len(chunk.Choices) == 0 {
				if err == io.EOF {
					break
				}
				continue
			}

			choice := chunk.Choices[0]

			if choice.Delta.Content != "" {
				ch <- ChatChunk{Type: ChunkText, Content: choice.Delta.Content}
			}

			if len(choice.Delta.ToolCalls) > 0 {
				tcs := []ToolCall{}
				for _, tc := range choice.Delta.ToolCalls {
					tcs = append(tcs, ToolCall{
						ID:        tc.ID,
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					})
				}
				ch <- ChatChunk{Type: ChunkToolCall, ToolCalls: tcs}
			}

			if choice.FinishReason != "" {
				ch <- ChatChunk{Type: ChunkDone}
				break
			}

			if err == io.EOF {
				break
			}
		}
	}()

	return ch, nil
}

// GeminiProvider implements the Gemini API.
type GeminiProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewGeminiProvider(apiKey string) *GeminiProvider {
	return &GeminiProvider{
		baseURL: "https://generativelanguage.googleapis.com/v1beta",
		apiKey:  apiKey,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *GeminiProvider) Name() string { return "gemini" }

func (p *GeminiProvider) Chat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	ch := make(chan ChatChunk, 8)

	go func() {
		defer close(ch)

		// Build contents array
		contents := []map[string]interface{}{}
		for _, m := range req.Messages {
			role := "user"
			if m.Role == "assistant" {
				role = "model"
			}
			contents = append(contents, map[string]interface{}{
				"role": role,
				"parts": []map[string]interface{}{
					{"text": m.Content},
				},
			})
		}

		body := map[string]interface{}{
			"contents": contents,
		}

		if req.System != "" {
			body["system_instruction"] = map[string]interface{}{
				"parts": []map[string]interface{}{
					{"text": req.System},
				},
			}
		}

		if len(req.Tools) > 0 {
			decls := []map[string]interface{}{}
			for _, t := range req.Tools {
				decls = append(decls, map[string]interface{}{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.Parameters,
				})
			}
			body["tools"] = []map[string]interface{}{
				{"function_declarations": decls},
			}
		}

		payload, err := json.Marshal(body)
		if err != nil {
			ch <- ChatChunk{Type: ChunkError, Err: fmt.Errorf("marshal request: %w", err)}
			return
		}

		url := p.baseURL + "/models/" + req.Model + ":generateContent?key=" + p.apiKey
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
		if err != nil {
			ch <- ChatChunk{Type: ChunkError, Err: fmt.Errorf("create request: %w", err)}
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := p.client.Do(httpReq)
		if err != nil {
			ch <- ChatChunk{Type: ChunkError, Err: fmt.Errorf("request: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			ch <- ChatChunk{Type: ChunkError, Err: fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))}
			return
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			ch <- ChatChunk{Type: ChunkError, Err: fmt.Errorf("read response: %w", err)}
			return
		}

		var result struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text   string `json:"text"`
						FnCall struct {
							Name  string `json:"name"`
							Args  string `json:"args"`
						} `json:"functionCall"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}

		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			ch <- ChatChunk{Type: ChunkError, Err: fmt.Errorf("parse response: %w", err)}
			return
		}

		if len(result.Candidates) == 0 {
			ch <- ChatChunk{Type: ChunkDone}
			return
		}

		for _, part := range result.Candidates[0].Content.Parts {
			if part.Text != "" {
				ch <- ChatChunk{Type: ChunkText, Content: part.Text}
			}
			if part.FnCall.Name != "" {
				ch <- ChatChunk{Type: ChunkToolCall, ToolCalls: []ToolCall{
					{ID: "call_" + part.FnCall.Name, Name: part.FnCall.Name, Arguments: part.FnCall.Args},
				}}
			}
		}

		ch <- ChatChunk{Type: ChunkDone}
	}()

	return ch, nil
}

// NewProvider creates a provider from config.
func NewProvider(cfg *Config) (Provider, error) {
	switch cfg.Provider {
	case "openai", "ollama", "openrouter", "nvidia", "nim", "generic":
		return NewOpenAIProvider(cfg.BaseURL, cfg.APIKey), nil
	case "gemini":
		return NewGeminiProvider(cfg.APIKey), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}


