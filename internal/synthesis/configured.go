package synthesis

import "strings"

func NewConfigured(providerName, model, apiKey string) Synthesizer {
	switch strings.ToLower(strings.TrimSpace(providerName)) {
	case "", "none":
		return NewFallback()
	case "openai":
		if strings.TrimSpace(apiKey) == "" {
			return NewFallback()
		}
		return New(NewOpenAIProvider(apiKey, nil), model)
	case "opencode":
		if strings.TrimSpace(model) == "" {
			return NewFallback()
		}
		return New(NewOpenCodeProvider(), model)
	default:
		return NewFallback()
	}
}
