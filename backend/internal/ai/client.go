package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"lysis/internal/config"
)

type Client struct {
	endpoint    string
	apiKey      string
	model       string
	maxTokens   int
	temperature float64
	httpClient  *http.Client
}

func NewClient(cfg config.LLMConfig) *Client {
	return &Client{
		endpoint:    strings.TrimRight(cfg.Endpoint, "/"),
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		maxTokens:   cfg.MaxTokens,
		temperature: cfg.Temperature,
		httpClient:  &http.Client{Timeout: 10 * time.Minute},
	}
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type ToolDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type ToolCall struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type ChatClient interface {
	Chat(messages []Message) (string, error)
	ChatWithTools(messages []Message, tools []ToolDef, toolChoice string) (*ChatResult, error)
	ChatWithRetry(messages []Message, tools []ToolDef, toolChoice string) (*ChatResult, error)
}

type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content           string     `json:"content"`
			ToolCalls         []ToolCall `json:"tool_calls"`
			ReasoningContent  string     `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`
}

type ChatResult struct {
	Content          string
	ReasoningContent string
	ToolCalls        []ToolCall
}

func (c *Client) Chat(messages []Message) (string, error) {
	res, err := c.ChatWithRetry(messages, nil, "")
	if err != nil {
		return "", err
	}
	return res.Content, nil
}

func (c *Client) ChatWithTools(messages []Message, tools []ToolDef, toolChoice string) (*ChatResult, error) {
	reqBody := map[string]interface{}{
		"model":       c.model,
		"messages":    messages,
		"max_tokens":  c.maxTokens,
		"temperature": c.temperature,
	}

	if len(tools) > 0 {
		toolDefs := make([]map[string]interface{}, len(tools))
		for i, t := range tools {
			toolDefs[i] = map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.Parameters,
				},
			}
		}
		reqBody["tools"] = toolDefs
		if toolChoice != "" {
			reqBody["tool_choice"] = toolChoice
		}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.endpoint+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusTooManyRequests:
			return nil, fmt.Errorf("AI service is busy, try again later")
		case http.StatusUnauthorized:
			return nil, fmt.Errorf("AI service authentication failed")
		default:
			return nil, fmt.Errorf("AI service error (code %d)", resp.StatusCode)
		}
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	msg := chatResp.Choices[0].Message
	return &ChatResult{
		Content:          msg.Content,
		ReasoningContent: msg.ReasoningContent,
		ToolCalls:        msg.ToolCalls,
	}, nil
}

func (c *Client) ChatWithRetry(messages []Message, tools []ToolDef, toolChoice string) (*ChatResult, error) {
	var lastErr error
	for i := 0; i < 5; i++ {
		res, err := c.ChatWithTools(messages, tools, toolChoice)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if strings.Contains(err.Error(), "try again later") ||
			strings.Contains(err.Error(), "no choices") ||
			strings.Contains(err.Error(), "code 5") {
			time.Sleep(time.Duration(5*(i+1)) * time.Second)
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("retry exhausted: %w", lastErr)
}

type FailoverClient struct {
	Primary   *Client
	Secondary *Client
}

func NewFailoverClient(primary, secondary *Client) *FailoverClient {
	return &FailoverClient{Primary: primary, Secondary: secondary}
}

func (fc *FailoverClient) ChatWithTools(messages []Message, tools []ToolDef, toolChoice string) (*ChatResult, error) {
	res, err := fc.Primary.ChatWithTools(messages, tools, toolChoice)
	if err == nil || fc.Secondary == nil {
		return res, err
	}
	log.Printf("[ai] primary failed, falling back to secondary: %v", err)
	return fc.Secondary.ChatWithTools(messages, tools, toolChoice)
}

func (fc *FailoverClient) ChatWithRetry(messages []Message, tools []ToolDef, toolChoice string) (*ChatResult, error) {
	res, err := fc.Primary.ChatWithRetry(messages, tools, toolChoice)
	if err == nil || fc.Secondary == nil {
		return res, err
	}
	log.Printf("[ai] primary failed after retries, falling back to secondary: %v", err)
	return fc.Secondary.ChatWithRetry(messages, tools, toolChoice)
}

func (fc *FailoverClient) Chat(messages []Message) (string, error) {
	res, err := fc.ChatWithRetry(messages, nil, "")
	if err != nil {
		return "", err
	}
	return res.Content, nil
}
