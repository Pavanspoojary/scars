package scar

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	CurrentSchemaVersion = "1"
	DefaultStoreDir      = ".gitscars"
	ScarsDirName         = "scars"
	LocalDirName         = ".local"
)

// Lifecycle states
const (
	LifecycleDraft        = "draft"
	LifecycleActive       = "active"
	LifecycleContradicted = "contradicted"
	LifecycleSuperseded   = "superseded"
	LifecycleRetired      = "retired"
)

// Review states
const (
	ReviewUnreviewed = "unreviewed"
	ReviewApproved   = "approved"
	ReviewRejected   = "rejected"
)

// Verification outcomes
const (
	VerificationNotAttempted = "not_attempted"
	VerificationFailToPass   = "fail_to_pass"
	VerificationException    = "exception"
	VerificationFailed       = "failed"
)

// Freshness states
const (
	FreshnessCurrent     = "current"
	FreshnessChanged     = "changed"
	FreshnessUnavailable = "unavailable"
)

// Severity levels
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
)

// DependencyAnchor defines a library/package requirement.
type DependencyAnchor struct {
	Name    string `json:"name"`
	Version string `json:"version"` // Must be a quoted string
}

// Anchors defines the target contextual points where this Scar applies.
type Anchors struct {
	Files           []string           `json:"files"`
	Symbols         []string           `json:"symbols,omitempty"`
	Dependencies    []DependencyAnchor `json:"dependencies,omitempty"`
	Commands        []string           `json:"commands,omitempty"`
	ErrorSignatures []string           `json:"error_signatures,omitempty"`
}

// Verifier represents executable verification metadata.
type Verifier struct {
	Argv          []string `json:"argv"`
	WorkingDir    string   `json:"working_dir,omitempty"`
	EnvAllowlist  []string `json:"env_allowlist,omitempty"`
	Result        string   `json:"result"`
	CommandHash   string   `json:"command_hash,omitempty"`
	OutputExcerpt string   `json:"output_excerpt,omitempty"`
	OutputHash    string   `json:"output_hash,omitempty"`
}

// Baseline records anchor file hashes at time of verification/reaffirmation.
type Baseline struct {
	AnchorHashes map[string]string `json:"anchor_hashes,omitempty"`
	SymbolHashes map[string]string `json:"symbol_hashes,omitempty"`
	RecordedAt   string            `json:"recorded_at,omitempty"`
	CommandHash  string            `json:"command_hash,omitempty"`
}

// Review records human review decisions separate from automated verification.
type Review struct {
	Status    string `json:"status"`
	Reviewer  string `json:"reviewer,omitempty"`
	Date      string `json:"date,omitempty"`
	Rationale string `json:"rationale,omitempty"`
}

// Lifecycle tracks the validity and supersession status of the Scar.
type Lifecycle struct {
	State         string   `json:"state"`
	Provenance    string   `json:"provenance,omitempty"`
	ConflictsWith []string `json:"conflicts_with,omitempty"`
	Supersedes    []string `json:"supersedes,omitempty"`
	SupersededBy  string   `json:"superseded_by,omitempty"`
}

// Scar represents the complete in-memory representation of a Scar record.
type Scar struct {
	SchemaVersion string    `json:"schema_version"`
	ID            string    `json:"id"`
	Slug          string    `json:"slug"`
	Title         string    `json:"title"`
	Lesson        string    `json:"lesson"`
	Severity      string    `json:"severity,omitempty"` // critical | high | medium | low (default: medium)
	FixHint       string    `json:"fix_hint,omitempty"`
	BadPattern    string    `json:"bad_pattern,omitempty"`
	GoodPattern   string    `json:"good_pattern,omitempty"`
	Applicability string    `json:"applicability,omitempty"`
	Anchors       Anchors   `json:"anchors"`
	BaseCommit    string    `json:"base_commit,omitempty"`
	FixCommit     string    `json:"fix_commit,omitempty"`
	Verifier      Verifier  `json:"verifier"`
	Baseline      Baseline  `json:"baseline"`
	Review        Review    `json:"review"`
	Lifecycle     Lifecycle `json:"lifecycle"`
	Body          string    `json:"body,omitempty"`
}

// NewDraft creates a new unverified, unreviewed Scar in draft state.
func NewDraft(slug, title, lesson string, files []string) (*Scar, error) {
	if strings.TrimSpace(title) == "" {
		return nil, errors.New("title cannot be empty")
	}
	if strings.TrimSpace(slug) == "" {
		slug = Slugify(title)
	}
	shortID, err := RandomShortID(6)
	if err != nil {
		return nil, fmt.Errorf("failed generating random short ID: %w", err)
	}

	id := fmt.Sprintf("%s-%s", slug, shortID)

	return &Scar{
		SchemaVersion: CurrentSchemaVersion,
		ID:            id,
		Slug:          slug,
		Title:         strings.TrimSpace(title),
		Lesson:        strings.TrimSpace(lesson),
		Severity:      SeverityMedium,
		Anchors: Anchors{
			Files: files,
		},
		Verifier: Verifier{
			Result: VerificationNotAttempted,
			Argv:   []string{},
		},
		Review: Review{
			Status: ReviewUnreviewed,
		},
		Lifecycle: Lifecycle{
			State: LifecycleDraft,
		},
	}, nil
}

// ValidateInvariants enforces the core lifecycle and verification invariant rules.
func (s *Scar) ValidateInvariants() error {
	if s.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("unsupported schema_version %q (current supported is %q). Upgrade git-scars to read this file", s.SchemaVersion, CurrentSchemaVersion)
	}
	if strings.TrimSpace(s.ID) == "" {
		return errors.New("scar ID cannot be empty")
	}
	if strings.TrimSpace(s.Title) == "" {
		return errors.New("scar title cannot be empty")
	}

	// Validate severity if specified
	if s.Severity != "" {
		switch s.Severity {
		case SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow:
		default:
			return fmt.Errorf("invalid severity %q", s.Severity)
		}
	}

	// Validate lifecycle state enum
	switch s.Lifecycle.State {
	case LifecycleDraft, LifecycleActive, LifecycleContradicted, LifecycleSuperseded, LifecycleRetired:
	default:
		return fmt.Errorf("invalid lifecycle state %q", s.Lifecycle.State)
	}

	// Validate review status enum
	switch s.Review.Status {
	case ReviewUnreviewed, ReviewApproved, ReviewRejected:
	default:
		return fmt.Errorf("invalid review status %q", s.Review.Status)
	}

	// Validate verification result enum
	switch s.Verifier.Result {
	case VerificationNotAttempted, VerificationFailToPass, VerificationException, VerificationFailed:
	default:
		return fmt.Errorf("invalid verifier result %q", s.Verifier.Result)
	}

	// Invariant: A Scar cannot be active if review is rejected
	if s.Lifecycle.State == LifecycleActive && s.Review.Status == ReviewRejected {
		return errors.New("invariant violation: scar with rejected review cannot be active")
	}

	// Invariant: A Scar cannot be active if verification failed without documented exception
	if s.Lifecycle.State == LifecycleActive && s.Verifier.Result == VerificationFailed {
		return errors.New("invariant violation: scar with failed verification cannot be active")
	}

	// Invariant: A Scar becomes active only if it is approved and has valid fail_to_pass or reviewer exception
	if s.Lifecycle.State == LifecycleActive {
		if s.Review.Status != ReviewApproved {
			return fmt.Errorf("invariant violation: active scar must be approved (current review status is %q)", s.Review.Status)
		}
		if s.Verifier.Result != VerificationFailToPass && s.Verifier.Result != VerificationException {
			return fmt.Errorf("invariant violation: active scar must have fail_to_pass verification or reviewer exception (current result is %q)", s.Verifier.Result)
		}
		if s.Verifier.Result == VerificationException && strings.TrimSpace(s.Review.Rationale) == "" {
			return errors.New("invariant violation: active scar with verification exception must document reviewer rationale")
		}
	}

	// Invariant: Contradicted, superseded, or retired Scars cannot be active
	if s.Lifecycle.State == LifecycleContradicted || s.Lifecycle.State == LifecycleSuperseded || s.Lifecycle.State == LifecycleRetired {
		// History is preserved; cannot be active
	}

	return nil
}

// Promote transitions a scar toward active state while enforcing invariants.
func (s *Scar) Promote(targetState, reviewer, rationale string) error {
	if targetState == LifecycleActive {
		if s.Review.Status != ReviewApproved && reviewer == "" {
			return errors.New("cannot activate scar without reviewer approval")
		}
		if reviewer != "" {
			s.Review.Status = ReviewApproved
			s.Review.Reviewer = reviewer
			s.Review.Date = time.Now().UTC().Format(time.RFC3339)
			s.Review.Rationale = rationale
		}
		if s.Verifier.Result != VerificationFailToPass && s.Verifier.Result != VerificationException {
			return fmt.Errorf("cannot activate scar with verifier result %q (requires %s or documented %s)", s.Verifier.Result, VerificationFailToPass, VerificationException)
		}
	}
	s.Lifecycle.State = targetState
	return s.ValidateInvariants()
}

// Retire marks a scar as retired with an optional rationale.
func (s *Scar) Retire(rationale string) error {
	s.Lifecycle.State = LifecycleRetired
	if rationale != "" {
		if s.Review.Rationale != "" {
			s.Review.Rationale = s.Review.Rationale + " | Retired: " + rationale
		} else {
			s.Review.Rationale = "Retired: " + rationale
		}
	}
	return s.ValidateInvariants()
}

// Contradict marks a scar as contradicted by another scar with documented rationale.
func (s *Scar) Contradict(byScarID, rationale string) error {
	s.Lifecycle.State = LifecycleContradicted
	if byScarID != "" {
		found := false
		for _, c := range s.Lifecycle.ConflictsWith {
			if c == byScarID {
				found = true
				break
			}
		}
		if !found {
			s.Lifecycle.ConflictsWith = append(s.Lifecycle.ConflictsWith, byScarID)
		}
	}
	if rationale != "" {
		if s.Review.Rationale != "" {
			s.Review.Rationale = s.Review.Rationale + " | Contradicted: " + rationale
		} else {
			s.Review.Rationale = "Contradicted: " + rationale
		}
	}
	return s.ValidateInvariants()
}

// Supersede marks a scar as superseded by a successor scar.
func (s *Scar) Supersede(byScarID string) error {
	if strings.TrimSpace(byScarID) == "" {
		return errors.New("supersede requires successor scar ID")
	}
	s.Lifecycle.State = LifecycleSuperseded
	s.Lifecycle.SupersededBy = byScarID
	return s.ValidateInvariants()
}

// RandomShortID generates a cryptographically secure random alphanumeric ID of length n.
func RandomShortID(length int) (string, error) {
	bytes := make([]byte, (length+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:length], nil
}

var nonSlugRegex = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a title string into a url/file-friendly slug.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonSlugRegex.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "scar"
	}
	if len(s) > 32 {
		s = s[:32]
		s = strings.TrimRight(s, "-")
	}
	return s
}
