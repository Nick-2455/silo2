package seed

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"

	"github.com/nicolasperalta/silo2/internal/engram"
)

// Generator turns an observation into a synthesis proposal.
//
// The interface exists so we can swap implementations (mock today, real
// model tomorrow) without touching the capture command, the renderer,
// or the inbox listing. Implementations MUST be pure with respect to
// time: same input → same output, every time. Anything time-dependent
// breaks idempotency in the vault.
type Generator interface {
	Generate(obs engram.Observation) (Seed, error)
}

// MockGenerator is the MVP generator. It produces deterministic,
// intentionally weak proposals so we can validate the human triage loop
// without taking on any LLM dependency.
//
// Design intent — read before changing:
//
//   - Themes are ALWAYS ["unclassified"]. The mock must not teach the
//     system the creator's categories. A real generator may infer themes
//     from content; the mock deliberately refuses.
//   - Summaries are first-N chars of content with a truncation marker.
//     No paraphrase, no synthesis, no claims. The point is "here is what
//     you saved", not "here is what I think".
//   - WhyItMightMatter is a fixed prompt-shaped string. Its job is to
//     invite human reflection, not to assert relevance.
type MockGenerator struct{}

func NewMockGenerator() *MockGenerator {
	return &MockGenerator{}
}

const (
	mockThemeUnclassified = "unclassified"
	mockSummaryMax        = 200
	mockWhyItMightMatter  = "Awaiting human refinement. Open this seed and decide if it deserves a place in your curated knowledge."
	mockUntitledTitle     = "Untitled seed"
	titleWordsFallback    = 6
)

// Generate returns a deterministic Seed for one observation.
//
// Errors only on truly malformed input (currently: nil observation ID).
// All "missing data" cases are handled with documented fallbacks, because
// capture must never fail just because the user typed something terse.
func (g *MockGenerator) Generate(obs engram.Observation) (Seed, error) {
	if strings.TrimSpace(obs.ID) == "" {
		return Seed{}, errors.New("observation ID is empty")
	}

	return Seed{
		ID:                   seedID([]engram.Observation{obs}),
		Title:                mockTitle(obs),
		SourceObservationIDs: []string{obs.ID},
		ProposedSummary:      mockSummary(obs),
		SuggestedThemes:      []string{mockThemeUnclassified},
		WhyItMightMatter:     mockWhyItMightMatter,
		UserWhy:              obs.Why, // verbatim, never modified
	}, nil
}

// seedID derives a stable seed identifier from the observations the seed
// synthesizes. It hashes (ID, Content, Why) per observation, sorted by
// ID for order independence.
//
// Why include Content and Why — not just ID: in mock mode the backend
// resets its ID counter on every process, so two captures with different
// content would otherwise collide on "obs-mock-1" and the second seed
// would be silently skipped by WriteNoteIfAbsent. Hashing the actual
// synthesis input solves that and is also the morally correct identity:
// the seed IS the synthesis of this content, not of this opaque ID.
//
// Idempotency still holds: identical content + identical Why → identical
// seed ID, regardless of which observation ID the backend happened to
// assign that run.
//
// 8 hex chars (32 bits) is plenty for a per-user inbox.
func seedID(obs []engram.Observation) string {
	sorted := append([]engram.Observation(nil), obs...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	h := sha256.New()
	for _, o := range sorted {
		// Field separators (NUL) avoid any chance of two fields running
		// together producing the same digest as a different shape.
		_, _ = h.Write([]byte(o.ID))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(o.Content))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(o.Why))
		_, _ = h.Write([]byte{0xff})
	}
	sum := h.Sum(nil)
	return "seed-" + hex.EncodeToString(sum[:4])
}

// mockTitle picks a heading for the seed. Resolution order:
//  1. Observation title, trimmed.
//  2. First N words of content, trimmed.
//  3. Literal sentinel "Untitled seed".
//
// The renderer will sit this string directly under a `# ` heading, so it
// must already read like a heading (no trailing punctuation cleanup is
// attempted — humans will rewrite it anyway during triage).
func mockTitle(obs engram.Observation) string {
	if t := strings.TrimSpace(obs.Title); t != "" {
		return t
	}
	if c := strings.TrimSpace(obs.Content); c != "" {
		words := strings.Fields(c)
		if len(words) > titleWordsFallback {
			words = words[:titleWordsFallback]
		}
		return strings.Join(words, " ")
	}
	return mockUntitledTitle
}

// mockSummary returns a faithful, non-interpretive preview of the
// observation content. It deliberately does NOT include UserWhy: that
// signal belongs to a dedicated section so its provenance (human, not
// AI) is unambiguous in the rendered seed.
func mockSummary(obs engram.Observation) string {
	body := strings.TrimSpace(obs.Content)
	if body == "" {
		return "(no content captured)"
	}
	// Use rune-aware truncation so we never split a multi-byte rune.
	runes := []rune(body)
	if len(runes) <= mockSummaryMax {
		return body
	}
	return strings.TrimRight(string(runes[:mockSummaryMax]), " \t\n") + " […]"
}
