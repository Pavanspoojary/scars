package scar

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const FeedbackFileName = "feedback.jsonl"

// FeedbackEntry represents a locally stored user or agent feedback record.
type FeedbackEntry struct {
	ScarID        string `json:"scar_id"`
	Rating        string `json:"rating"` // helpful, irrelevant, incorrect, unrated
	Timestamp     string `json:"timestamp"`
	RetrievalMode string `json:"retrieval_mode,omitempty"`
	Notes         string `json:"notes,omitempty"`
}

// RecordFeedback appends a feedback record to .gitscars/.local/feedback.jsonl.
func (s *Store) RecordFeedback(scarID, rating, retrievalMode, notes string) error {
	switch rating {
	case "helpful", "irrelevant", "incorrect", "unrated":
	default:
		return fmt.Errorf("invalid rating %q; expected helpful, irrelevant, incorrect, or unrated", rating)
	}

	if scarID == "" {
		return errors.New("scar_id cannot be empty")
	}

	entry := FeedbackEntry{
		ScarID:        scarID,
		Rating:        rating,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		RetrievalMode: retrievalMode,
		Notes:         notes,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.MkdirAll(s.LocalDir, 0755); err != nil {
		return err
	}

	feedbackPath := filepath.Join(s.LocalDir, FeedbackFileName)
	f, err := os.OpenFile(feedbackPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed opening feedback file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed appending feedback: %w", err)
	}

	return nil
}
