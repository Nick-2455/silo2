package synthesis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type opencodeProvider struct {
	binary string
	runner opencodeRunner
}

type opencodeRunner func(ctx context.Context, bin string, args []string) ([]byte, error)

type opencodeEvent struct {
	Type string `json:"type"`
	Part struct {
		Text string `json:"text"`
	} `json:"part"`
	Error struct {
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	} `json:"error"`
}

func NewOpenCodeProvider() Provider {
	return newOpenCodeProviderWithRunner("opencode", execRunner)
}

func newOpenCodeProviderWithRunner(bin string, runner opencodeRunner) Provider {
	if strings.TrimSpace(bin) == "" {
		bin = "opencode"
	}
	if runner == nil {
		runner = execRunner
	}
	return opencodeProvider{binary: bin, runner: runner}
}

func (p opencodeProvider) Complete(ctx context.Context, req Request) (string, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return "", errors.New("opencode model is required")
	}

	args := []string{"run", "--format", "json", "--model", model, buildOpenCodePrompt(req.Source)}
	raw, err := p.runner(ctx, p.binary, args)
	if err != nil {
		return "", fmt.Errorf("run opencode completion: %w", err)
	}

	content, err := extractOpenCodeContent(raw)
	if err != nil {
		return "", err
	}
	return content, nil
}

func execRunner(ctx context.Context, bin string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	return cmd.Output()
}

func extractOpenCodeContent(raw []byte) (string, error) {
	var lastText string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var event opencodeEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		switch event.Type {
		case "text":
			lastText = event.Part.Text
		case "error":
			return "", errors.New("opencode provider returned an error")
		}
	}

	content := strings.TrimSpace(stripMarkdownFences(lastText))
	if content == "" {
		return "", errors.New("opencode response missing text content")
	}
	if !isOpenCodeJSONObjectString(content) {
		return "", errors.New("opencode response content must be a valid json object")
	}
	return content, nil
}

func stripMarkdownFences(s string) string {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "```") || !strings.HasSuffix(trimmed, "```") {
		return s
	}

	body := strings.TrimPrefix(trimmed, "```")
	body = strings.TrimSuffix(body, "```")
	body = strings.TrimLeft(body, " \t\r\n")
	if strings.HasPrefix(strings.ToLower(body), "json") {
		rest := body[len("json"):]
		if rest == "" || strings.HasPrefix(rest, "\n") || strings.HasPrefix(rest, "\r\n") || strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, "\t") {
			body = rest
		}
	}
	return strings.TrimSpace(body)
}

func isOpenCodeJSONObjectString(content string) bool {
	if !json.Valid([]byte(content)) {
		return false
	}
	return strings.HasPrefix(content, "{") && strings.HasSuffix(content, "}")
}

func buildOpenCodePrompt(src Source) string {
	return fmt.Sprintf("You generate conservative proposal text for human review only. Do not use tools. Do not call bash. Do not write files. Return only a JSON object. The JSON object must contain keys proposed_summary, suggested_themes, and why_it_might_matter. Do not add other keys, markdown, or claims beyond the source text.\n\nTitle: %s\nContent: %s\nContext: %s", src.Title, src.Content, src.ContextHint)
}
