package markdown

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nicolasperalta/silo2/internal/engram"
)

func sampleObs() []engram.Observation {
	return []engram.Observation{
		{
			ID: "1", Title: "Silo Design", Type: "decision",
			TopicKey: "architecture/silo-design",
			Content:  "first draft", Project: "silo2",
			CreatedAt: time.Date(2026, 5, 16, 14, 40, 0, 0, time.UTC),
		},
		{
			ID: "2", Title: "Silo Design v2", Type: "decision",
			TopicKey: "architecture/silo-design", // SAME topic_key → must collapse
			Content:  "second draft", Project: "silo2",
			CreatedAt: time.Date(2026, 5, 16, 15, 0, 0, 0, time.UTC),
		},
		{
			ID: "3", Title: "Skills snapshot", Type: "learning",
			Content: "Go, SwiftUI", Project: "silo2",
			CreatedAt: time.Date(2026, 5, 16, 16, 0, 0, 0, time.UTC),
		},
		{
			ID: "4", Title: "Project Engram integration", Type: "project",
			Content: "milestone plan", Project: "silo2",
		},
		{
			ID: "5", Title: "", Type: "note",
			Content: "Untitled thought", Project: "silo2",
		},
	}
}

func TestRenderCurated_GroupsByTopicKey(t *testing.T) {
	out, err := RenderCurated(sampleObs())
	if err != nil {
		t.Fatalf("RenderCurated: %v", err)
	}

	// Obs 1 and 2 share a topic_key → ONE file under Architecture.
	got, ok := out["Architecture/silo-design.md"]
	if !ok {
		t.Fatalf("missing Architecture/silo-design.md; got keys=%v", keysOf(out))
	}
	// It must link to BOTH raw observations.
	for _, want := range []string{"[[Raw/Observations/silo-design]]", "[[Raw/Observations/silo-design-v2]]"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected curated note to link %s, got:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "topic_key: architecture/silo-design") {
		t.Errorf("topic_key not in frontmatter:\n%s", got)
	}
}

func TestRenderCurated_BucketsByType(t *testing.T) {
	out, _ := RenderCurated(sampleObs())

	// Learning → Career
	if _, ok := out["Career/skills-snapshot.md"]; !ok {
		t.Errorf("learning obs should land in Career/; keys=%v", keysOf(out))
	}
	// Project → Projects
	if _, ok := out["Projects/project-engram-integration.md"]; !ok {
		t.Errorf("project obs should land in Projects/; keys=%v", keysOf(out))
	}
	// Untitled → falls back to observation-<id>; default type "note" → Architecture
	if _, ok := out["Architecture/observation-5.md"]; !ok {
		t.Errorf("untitled obs should produce Architecture/observation-5.md; keys=%v", keysOf(out))
	}
}

func TestRenderCurated_AlwaysEmitsBucketReadmes(t *testing.T) {
	out, err := RenderCurated(nil)
	if err != nil {
		t.Fatalf("RenderCurated nil: %v", err)
	}
	for _, b := range CuratedBuckets {
		if _, ok := out[b+"/README.md"]; !ok {
			t.Errorf("missing README for bucket %s", b)
		}
	}
	if len(out) != len(CuratedBuckets) {
		t.Errorf("empty input should only emit bucket READMEs, got keys=%v", keysOf(out))
	}
}

func TestRenderCurated_Deterministic(t *testing.T) {
	a, err1 := RenderCurated(sampleObs())
	b, err2 := RenderCurated(sampleObs())
	if err1 != nil || err2 != nil {
		t.Fatalf("err: %v %v", err1, err2)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("RenderCurated is non-deterministic")
	}
}

func TestClassify_TopicKeyPrefixWinsOverType(t *testing.T) {
	o := engram.Observation{
		ID: "x", Title: "Whatever", Type: "decision",
		TopicKey: "career/promotion-plan",
	}
	bucket, slug, _ := classify(o)
	if bucket != "Career" {
		t.Errorf("expected Career bucket from topic_key prefix, got %s", bucket)
	}
	if slug != "promotion-plan" {
		t.Errorf("expected slug from topic_key tail, got %q", slug)
	}
}

func keysOf(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
