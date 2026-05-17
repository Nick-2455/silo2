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
	default:
		return NewFallback()
	}
}
