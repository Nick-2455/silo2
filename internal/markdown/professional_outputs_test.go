package markdown

import (
	"reflect"
	"strings"
	"testing"

	"github.com/nicolasperalta/silo2/internal/identity"
)

func sampleIdentity() *identity.Identity {
	return &identity.Identity{
		Name:      "Nicolas Peralta",
		Role:      "Software Architect",
		Areas:     []string{"Developer Tooling", "Knowledge Management"},
		Skills:    []string{"Architecture", "Go", "SwiftUI"},
		Interests: []string{"Local-first software"},
		Projects: []identity.Project{
			{Name: "Engram", Description: "Persistent memory store.", Status: "active"},
			{Name: "Silo", Description: "Markdown projection over Engram.", Status: "active"},
		},
		Goals: []string{"Keep Engram as source of truth"},
		Evidence: []identity.Evidence{
			{Source: "Curated Curated/Identity/profile.md", Summary: "Profile"},
			{Source: "Curated Curated/Architecture/silo2-mvp.md", Summary: "Silo2 MVP"},
		},
		Outputs: identity.DefaultOutputs(),
	}
}

func TestRenderProfessionalOutputs_EmitsAllThree(t *testing.T) {
	out, err := RenderProfessionalOutputs(sampleIdentity(), "curated")
	if err != nil {
		t.Fatalf("RenderProfessionalOutputs: %v", err)
	}
	for _, want := range []string{"CV.md", "LinkedIn.md", "ProfessionalBio.md"} {
		if _, ok := out[want]; !ok {
			t.Errorf("missing %s; got keys=%v", want, keysOfOutputs(out))
		}
	}
}

func TestRenderProfessionalOutputs_Deterministic(t *testing.T) {
	a, err1 := RenderProfessionalOutputs(sampleIdentity(), "curated")
	b, err2 := RenderProfessionalOutputs(sampleIdentity(), "curated")
	if err1 != nil || err2 != nil {
		t.Fatalf("errs: %v %v", err1, err2)
	}
	if !reflect.DeepEqual(a, b) {
		t.Error("RenderProfessionalOutputs is non-deterministic")
	}
}

func TestRenderProfessionalOutputs_StampsSource(t *testing.T) {
	out, _ := RenderProfessionalOutputs(sampleIdentity(), "raw/engram")
	for name, content := range out {
		if !strings.Contains(content, "source: raw/engram") {
			t.Errorf("%s missing source frontmatter, got:\n%s", name, content)
		}
	}
}

func TestRenderProfessionalOutputs_EmptySource_DefaultsToUnknown(t *testing.T) {
	out, _ := RenderProfessionalOutputs(sampleIdentity(), "  ")
	if !strings.Contains(out["CV.md"], "source: unknown") {
		t.Errorf("expected fallback source=unknown, got:\n%s", out["CV.md"])
	}
}

func TestRenderProfessionalOutputs_CV_ContainsIdentityFields(t *testing.T) {
	out, _ := RenderProfessionalOutputs(sampleIdentity(), "curated")
	cv := out["CV.md"]
	for _, want := range []string{
		"Nicolas Peralta", "Software Architect",
		"- Go", "- SwiftUI", "- Architecture",
		"**Engram**", "**Silo**",
		"TODO: list previous roles",
		"TODO: list degrees",
		"TODO: list relevant certifications",
	} {
		if !strings.Contains(cv, want) {
			t.Errorf("CV.md missing %q. content:\n%s", want, cv)
		}
	}
}

func TestRenderProfessionalOutputs_LinkedIn_HeadlineAndAbout(t *testing.T) {
	out, _ := RenderProfessionalOutputs(sampleIdentity(), "curated")
	li := out["LinkedIn.md"]
	for _, want := range []string{
		"Software Architect — Developer Tooling and Knowledge Management",
		"Nicolas Peralta is a Software Architect.",
		"Focused on Developer Tooling and Knowledge Management.",
		"TODO: paste roles",
		"TODO: paste degrees",
	} {
		if !strings.Contains(li, want) {
			t.Errorf("LinkedIn.md missing %q. content:\n%s", want, li)
		}
	}
}

func TestRenderProfessionalOutputs_Bio_ThreeLengths(t *testing.T) {
	out, _ := RenderProfessionalOutputs(sampleIdentity(), "curated")
	bio := out["ProfessionalBio.md"]
	for _, want := range []string{
		"## Short Bio",
		"## Medium Bio",
		"## Long Bio",
		"## Key Themes",
		"Nicolas Peralta, Software Architect, focused on Developer Tooling and Knowledge Management.",
		"Works primarily with Architecture, Go, and SwiftUI.",
		// Long bio pulls evidence summaries.
		"Recent threads:",
	} {
		if !strings.Contains(bio, want) {
			t.Errorf("ProfessionalBio.md missing %q. content:\n%s", want, bio)
		}
	}
	// Themes section must combine Areas + Interests, dedup, ordered.
	if !strings.Contains(bio, "- Developer Tooling") ||
		!strings.Contains(bio, "- Knowledge Management") ||
		!strings.Contains(bio, "- Local-first software") {
		t.Errorf("Themes incomplete:\n%s", bio)
	}
}

func TestRenderProfessionalOutputs_NilIdentity_ReturnsError(t *testing.T) {
	if _, err := RenderProfessionalOutputs(nil, "curated"); err == nil {
		t.Error("expected error for nil identity")
	}
}

func TestJoinHuman(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b"}, "a and b"},
		{[]string{"a", "b", "c"}, "a, b, and c"},
		{[]string{"a", "b", "c", "d"}, "a, b, c, and d"},
	}
	for _, c := range cases {
		if got := joinHuman(c.in); got != c.want {
			t.Errorf("joinHuman(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func keysOfOutputs(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
