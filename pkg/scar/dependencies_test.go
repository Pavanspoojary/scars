package scar

import (
	"strings"
	"testing"
)

func TestParseDependencyAnchors(t *testing.T) {
	doc := []byte(`---
schema_version: "1"
id: "dep-parse"
title: "Dependency anchored scar"
lesson: "Upgrade redis client"
anchors:
  files:
    - "pkg/cache/redis.go"
  dependencies:
    - name: "github.com/redis/go-redis/v9"
      version: "9.5.1"
    - name: "example.com/plain"
verifier:
  result: "not_attempted"
lifecycle:
  state: "draft"
---
`)

	scar, err := ParseFrontmatter(doc)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(scar.Anchors.Dependencies) != 2 {
		t.Fatalf("expected 2 dependency anchors, got %d (%+v)", len(scar.Anchors.Dependencies), scar.Anchors.Dependencies)
	}
	if scar.Anchors.Dependencies[0].Name != "github.com/redis/go-redis/v9" {
		t.Errorf("unexpected dependency name: %q", scar.Anchors.Dependencies[0].Name)
	}
	if scar.Anchors.Dependencies[0].Version != "9.5.1" {
		t.Errorf("unexpected dependency version: %q", scar.Anchors.Dependencies[0].Version)
	}
	if scar.Anchors.Dependencies[1].Name != "example.com/plain" {
		t.Errorf("unexpected dependency name: %q", scar.Anchors.Dependencies[1].Name)
	}
	if scar.Anchors.Dependencies[1].Version != "" {
		t.Errorf("expected empty version, got %q", scar.Anchors.Dependencies[1].Version)
	}
}

func TestDependencyAnchorRoundTrip(t *testing.T) {
	scar := &Scar{
		SchemaVersion: CurrentSchemaVersion,
		ID:            "dep-round-trip",
		Title:         "Dependency round trip",
		Lesson:        "Lesson",
		Anchors: Anchors{
			Files: []string{"pkg/cache/redis.go"},
			Dependencies: []DependencyAnchor{
				{Name: "github.com/redis/go-redis/v9", Version: "9.5.1"},
				{Name: "example.com/plain"},
			},
		},
		Verifier:  Verifier{Result: VerificationNotAttempted},
		Review:    Review{Status: ReviewUnreviewed},
		Lifecycle: Lifecycle{State: LifecycleDraft},
	}

	data, err := FormatFrontmatter(scar)
	if err != nil {
		t.Fatalf("unexpected format error: %v", err)
	}
	if !strings.Contains(string(data), "dependencies:") {
		t.Fatalf("expected serialized output to contain dependencies, got:\n%s", data)
	}

	reparsed, err := ParseFrontmatter(data)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(reparsed.Anchors.Dependencies) != 2 {
		t.Fatalf("expected 2 dependency anchors after round trip, got %d", len(reparsed.Anchors.Dependencies))
	}
	if reparsed.Anchors.Dependencies[0] != scar.Anchors.Dependencies[0] {
		t.Errorf("dependency mismatch after round trip: got %+v want %+v",
			reparsed.Anchors.Dependencies[0], scar.Anchors.Dependencies[0])
	}
	if reparsed.Anchors.Dependencies[1] != scar.Anchors.Dependencies[1] {
		t.Errorf("dependency mismatch after round trip: got %+v want %+v",
			reparsed.Anchors.Dependencies[1], scar.Anchors.Dependencies[1])
	}
}

func TestDependencyAnchorMatchesInstalledVersion(t *testing.T) {
	installed := map[string]string{"github.com/redis/go-redis/v9": "v9.5.1"}
	anchor := DependencyAnchor{Name: "github.com/redis/go-redis/v9", Version: "9.5.1"}
	if !MatchDependency(installed, anchor) {
		t.Error("expected dependency anchor to match the installed version")
	}
}
