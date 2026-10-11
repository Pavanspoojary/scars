package scar

import (
	"testing"
)

func TestMatchFileAnchorWeights(t *testing.T) {
	anchors := []string{"pkg/auth.go"}

	if s, _ := matchFileAnchor([]string{"pkg/auth.go"}, anchors); s != 5 {
		t.Errorf("expected exact file score 5, got %d", s)
	}
	if s, _ := matchFileAnchor([]string{"other/auth.go"}, anchors); s != 2 {
		t.Errorf("expected basename score 2, got %d", s)
	}
	if s, _ := matchFileAnchor([]string{"pkg/unrelated.go"}, anchors); s != 1 {
		t.Errorf("expected same-dir score 1, got %d", s)
	}
	if s, _ := matchFileAnchor([]string{"other/unrelated.go"}, anchors); s != 0 {
		t.Errorf("expected no-match score 0, got %d", s)
	}
}

func TestMatchSymbolAnchorWeights(t *testing.T) {
	anchors := []string{"RecordMetric"}

	if s, _ := matchSymbolAnchor([]string{"RecordMetric"}, anchors); s != 4 {
		t.Errorf("expected exact symbol score 4, got %d", s)
	}
	if s, _ := matchSymbolAnchor([]string{"(*Exporter).RecordMetric"}, anchors); s != 2 {
		t.Errorf("expected normalized symbol score 2, got %d", s)
	}
	if s, _ := matchSymbolAnchor([]string{"Unrelated"}, anchors); s != 0 {
		t.Errorf("expected no-match score 0, got %d", s)
	}
}

func TestNormalizeErrorSignatureStability(t *testing.T) {
	a := NormalizeErrorSignature("panic: runtime error at 0x4f2a1 main.go:42")
	b := NormalizeErrorSignature("panic: runtime error at 0x9be33 main.go:87")
	if a != b {
		t.Errorf("expected normalized signatures to match, got %q vs %q", a, b)
	}
	if NormalizeErrorSignature("") != "" {
		t.Errorf("expected empty normalization for empty input")
	}
}

func TestConfidenceForScoreBounds(t *testing.T) {
	if c := ConfidenceForScore(0); c != 0 {
		t.Errorf("expected 0 confidence for score 0, got %v", c)
	}
	if c := ConfidenceForScore(MaxPossibleScore); c != 1 {
		t.Errorf("expected 1 confidence for max score, got %v", c)
	}
	if c := ConfidenceForScore(5); c <= 0 || c >= 1 {
		t.Errorf("expected partial confidence in (0,1), got %v", c)
	}
}

func TestPartialFileSignalAbstainsAlone(t *testing.T) {
	s := &Scar{
		SchemaVersion: CurrentSchemaVersion,
		ID:            "test-001",
		Title:         "Test",
		Lesson:        "Test lesson",
		Anchors:       Anchors{Files: []string{"pkg/auth.go"}},
	}
	scored := ScoreScarItem(s, QueryOptions{Files: []string{"pkg/unrelated.go"}})
	if scored.Score >= 3 {
		t.Errorf("expected same-dir partial signal to abstain (score < 3), got %d", scored.Score)
	}
}
