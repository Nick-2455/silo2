package synthesis

import "context"

type Request struct {
	Source Source
	Model  string
}

type Provider interface {
	Complete(context.Context, Request) (string, error)
}

type providerSynthesizer struct {
	provider Provider
	model    string
	fallback Synthesizer
}

func New(provider Provider, model string) Synthesizer {
	if provider == nil {
		return NewFallback()
	}
	return providerSynthesizer{provider: provider, model: model, fallback: NewFallback()}
}

func (s providerSynthesizer) Synthesize(ctx context.Context, src Source) (Proposal, error) {
	raw, err := s.provider.Complete(ctx, Request{Source: src, Model: s.model})
	if err != nil {
		return s.fallback.Synthesize(ctx, src)
	}
	proposal, err := ParseProposal(raw)
	if err != nil {
		return s.fallback.Synthesize(ctx, src)
	}
	return proposal, nil
}
