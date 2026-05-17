package synthesis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	defaultOpenAIModel = "gpt-4.1-mini"
	openAIEndpointPath = "/v1/chat/completions"
)

type openAIProvider struct {
	apiKey   string
	client   *http.Client
	baseURL  string
	endpoint string
}

type openAIChatRequest struct {
	Model    string               `json:"model"`
	Messages []openAIChatMessage  `json:"messages"`
	Response openAIResponseFormat `json:"response_format"`
	Stream   bool                 `json:"stream"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponseFormat struct {
	Type string `json:"type"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func NewOpenAIProvider(apiKey string, client *http.Client) Provider {
	return newOpenAIProvider(apiKey, client, "https://api.openai.com")
}

func newOpenAIProvider(apiKey string, client *http.Client, baseURL string) Provider {
	if client == nil {
		client = http.DefaultClient
	}
	return openAIProvider{
		apiKey:   strings.TrimSpace(apiKey),
		client:   client,
		baseURL:  strings.TrimRight(baseURL, "/"),
		endpoint: openAIEndpointPath,
	}
}

func (p openAIProvider) Complete(ctx context.Context, req Request) (string, error) {
	if p.apiKey == "" {
		return "", errors.New("openai api key is required")
	}

	body, err := json.Marshal(buildOpenAIRequest(req))
	if err != nil {
		return "", fmt.Errorf("encode openai request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+p.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build openai request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request openai completion: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
		return "", fmt.Errorf("openai request failed with status %d", resp.StatusCode)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("openai request returned status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read openai response: %w", err)
	}

	return extractOpenAIContent(raw)
}

func buildOpenAIRequest(req Request) openAIChatRequest {
	return openAIChatRequest{
		Model: resolveOpenAIModel(req.Model),
		Messages: []openAIChatMessage{
			{
				Role:    "system",
				Content: openAISystemPrompt,
			},
			{
				Role:    "user",
				Content: buildOpenAIUserPrompt(req.Source),
			},
		},
		Response: openAIResponseFormat{Type: "json_object"},
		Stream:   false,
	}
}

func resolveOpenAIModel(model string) string {
	if strings.TrimSpace(model) == "" {
		return defaultOpenAIModel
	}
	return strings.TrimSpace(model)
}

func extractOpenAIContent(raw []byte) (string, error) {
	var response openAIChatResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", fmt.Errorf("decode openai response: %w", err)
	}
	if len(response.Choices) == 0 {
		return "", errors.New("openai response missing choices")
	}

	content := strings.TrimSpace(response.Choices[0].Message.Content)
	if content == "" {
		return "", errors.New("openai response missing content")
	}
	if !isJSONObjectString(content) {
		return "", errors.New("openai response content must be a valid json object")
	}
	return content, nil
}

func isJSONObjectString(content string) bool {
	if !json.Valid([]byte(content)) {
		return false
	}
	return strings.HasPrefix(content, "{") && strings.HasSuffix(content, "}")
}

func buildOpenAIUserPrompt(src Source) string {
	return fmt.Sprintf("Return exactly one JSON object with keys proposed_summary, suggested_themes, and why_it_might_matter. Do not add other keys, markdown, or claims beyond the source text.\n\nTitle: %s\nContent: %s\nContext: %s", src.Title, src.Content, src.ContextHint)
}

const openAISystemPrompt = "You generate conservative proposal text for human review only. Respond with strict JSON, keep uncertainty implicit, do not curate, and do not claim facts not present in the source."
