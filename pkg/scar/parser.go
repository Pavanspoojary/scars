package scar

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ParseFrontmatter parses markdown content with strict YAML frontmatter.
func ParseFrontmatter(data []byte) (*Scar, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	if !scanner.Scan() {
		return nil, errors.New("empty scar document")
	}

	firstLine := strings.TrimSpace(scanner.Text())
	if firstLine != "---" {
		return nil, fmt.Errorf("missing frontmatter delimiter '---', found %q", firstLine)
	}

	scar := &Scar{
		SchemaVersion: CurrentSchemaVersion,
		Review: Review{
			Status: ReviewUnreviewed,
		},
		Lifecycle: Lifecycle{
			State: LifecycleDraft,
		},
		Verifier: Verifier{
			Result: VerificationNotAttempted,
		},
	}

	var frontmatterLines []string
	var bodyLines []string
	inFrontmatter := true

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if inFrontmatter {
			if trimmed == "---" {
				inFrontmatter = false
				continue
			}
			// Security rule: Reject YAML aliases and anchor references (Billion Laughs protection)
			if strings.Contains(line, "*") || strings.Contains(line, "&") {
				// Check if it's an alias/anchor token rather than comment or text
				words := strings.Fields(line)
				for _, w := range words {
					if strings.HasPrefix(w, "*") || strings.HasPrefix(w, "&") {
						return nil, errors.New("strict YAML decoder rejected alias or anchor reference")
					}
				}
			}
			frontmatterLines = append(frontmatterLines, line)
		} else {
			bodyLines = append(bodyLines, line)
		}
	}

	if inFrontmatter {
		return nil, errors.New("unclosed frontmatter: missing closing '---'")
	}

	if err := parseYAMLMap(frontmatterLines, scar); err != nil {
		return nil, fmt.Errorf("frontmatter syntax error: %w", err)
	}

	scar.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
	if scar.Lesson == "" && scar.Body != "" {
		scar.Lesson = scar.Body
	}

	if err := scar.ValidateInvariants(); err != nil {
		return nil, err
	}

	return scar, nil
}

func parseYAMLMap(lines []string, s *Scar) error {
	var currentSection string
	var currentSubSection string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))

		// Top-level field
		if indent == 0 {
			currentSubSection = ""
			parts := strings.SplitN(trimmed, ":", 2)
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			val := ""
			if len(parts) > 1 {
				val = strings.TrimSpace(parts[1])
			}

			switch key {
			case "schema_version":
				s.SchemaVersion = unquoteVal(val)
			case "id":
				s.ID = unquoteVal(val)
			case "slug":
				s.Slug = unquoteVal(val)
			case "title":
				s.Title = unquoteVal(val)
			case "lesson":
				s.Lesson = unquoteVal(val)
			case "applicability":
				s.Applicability = unquoteVal(val)
			case "base_commit":
				s.BaseCommit = unquoteVal(val)
			case "fix_commit":
				s.FixCommit = unquoteVal(val)
			case "anchors", "verifier", "baseline", "review", "lifecycle":
				currentSection = key
			default:
				currentSection = ""
			}
			continue
		}

		// Indented sections (level 1 or 2)
		if indent >= 2 {
			if strings.HasPrefix(trimmed, "- ") {
				// List item
				item := unquoteVal(strings.TrimPrefix(trimmed, "- "))
				switch currentSection {
				case "anchors":
					switch currentSubSection {
					case "files":
						s.Anchors.Files = append(s.Anchors.Files, item)
					case "symbols":
						s.Anchors.Symbols = append(s.Anchors.Symbols, item)
					case "commands":
						s.Anchors.Commands = append(s.Anchors.Commands, item)
					case "error_signatures":
						s.Anchors.ErrorSignatures = append(s.Anchors.ErrorSignatures, item)
					}
				case "verifier":
					if currentSubSection == "argv" {
						s.Verifier.Argv = append(s.Verifier.Argv, item)
					} else if currentSubSection == "env_allowlist" {
						s.Verifier.EnvAllowlist = append(s.Verifier.EnvAllowlist, item)
					}
				case "lifecycle":
					switch currentSubSection {
					case "conflicts_with":
						s.Lifecycle.ConflictsWith = append(s.Lifecycle.ConflictsWith, item)
					case "supersedes":
						s.Lifecycle.Supersedes = append(s.Lifecycle.Supersedes, item)
					}
				}
				continue
			}

			parts := strings.SplitN(trimmed, ":", 2)
			subKey := strings.ToLower(strings.TrimSpace(parts[0]))
			subVal := ""
			if len(parts) > 1 {
				subVal = strings.TrimSpace(parts[1])
			}

			// Sub-section headers (e.g. `files:` under `anchors:`)
			if subVal == "" {
				currentSubSection = subKey
				continue
			}

			// Sub-section scalar fields
			switch currentSection {
			case "anchors":
				if subKey == "dependency" || strings.HasPrefix(subKey, "dep") {
					// Handle dependency object
				}
			case "verifier":
				switch subKey {
				case "working_dir":
					s.Verifier.WorkingDir = unquoteVal(subVal)
				case "result":
					s.Verifier.Result = unquoteVal(subVal)
				case "command_hash":
					s.Verifier.CommandHash = unquoteVal(subVal)
				case "output_excerpt":
					s.Verifier.OutputExcerpt = unquoteVal(subVal)
				case "output_hash":
					s.Verifier.OutputHash = unquoteVal(subVal)
				}
			case "baseline":
				if s.Baseline.AnchorHashes == nil {
					s.Baseline.AnchorHashes = make(map[string]string)
				}
				switch subKey {
				case "recorded_at":
					s.Baseline.RecordedAt = unquoteVal(subVal)
				case "command_hash":
					s.Baseline.CommandHash = unquoteVal(subVal)
				default:
					s.Baseline.AnchorHashes[subKey] = unquoteVal(subVal)
				}
			case "review":
				switch subKey {
				case "status":
					s.Review.Status = unquoteVal(subVal)
				case "reviewer":
					s.Review.Reviewer = unquoteVal(subVal)
				case "date":
					s.Review.Date = unquoteVal(subVal)
				case "rationale":
					s.Review.Rationale = unquoteVal(subVal)
				}
			case "lifecycle":
				switch subKey {
				case "state":
					s.Lifecycle.State = unquoteVal(subVal)
				case "provenance":
					s.Lifecycle.Provenance = unquoteVal(subVal)
				case "superseded_by":
					s.Lifecycle.SupersededBy = unquoteVal(subVal)
				}
			}
		}
	}

	return nil
}

func unquoteVal(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// FormatFrontmatter serializes a Scar into clean Markdown with strict YAML frontmatter.
func FormatFrontmatter(s *Scar) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("---\n")
	buf.WriteString(fmt.Sprintf("schema_version: %s\n", strconv.Quote(s.SchemaVersion)))
	buf.WriteString(fmt.Sprintf("id: %s\n", strconv.Quote(s.ID)))
	buf.WriteString(fmt.Sprintf("title: %s\n", strconv.Quote(s.Title)))
	if s.Slug != "" {
		buf.WriteString(fmt.Sprintf("slug: %s\n", strconv.Quote(s.Slug)))
	}
	if s.Lesson != "" {
		buf.WriteString(fmt.Sprintf("lesson: %s\n", strconv.Quote(s.Lesson)))
	}
	if s.Applicability != "" {
		buf.WriteString(fmt.Sprintf("applicability: %s\n", strconv.Quote(s.Applicability)))
	}
	if s.BaseCommit != "" {
		buf.WriteString(fmt.Sprintf("base_commit: %s\n", strconv.Quote(s.BaseCommit)))
	}
	if s.FixCommit != "" {
		buf.WriteString(fmt.Sprintf("fix_commit: %s\n", strconv.Quote(s.FixCommit)))
	}

	// Anchors
	buf.WriteString("anchors:\n")
	buf.WriteString("  files:\n")
	for _, f := range s.Anchors.Files {
		buf.WriteString(fmt.Sprintf("    - %s\n", strconv.Quote(f)))
	}
	if len(s.Anchors.Symbols) > 0 {
		buf.WriteString("  symbols:\n")
		for _, sym := range s.Anchors.Symbols {
			buf.WriteString(fmt.Sprintf("    - %s\n", strconv.Quote(sym)))
		}
	}
	if len(s.Anchors.Commands) > 0 {
		buf.WriteString("  commands:\n")
		for _, cmd := range s.Anchors.Commands {
			buf.WriteString(fmt.Sprintf("    - %s\n", strconv.Quote(cmd)))
		}
	}
	if len(s.Anchors.ErrorSignatures) > 0 {
		buf.WriteString("  error_signatures:\n")
		for _, errSig := range s.Anchors.ErrorSignatures {
			buf.WriteString(fmt.Sprintf("    - %s\n", strconv.Quote(errSig)))
		}
	}

	// Verifier
	buf.WriteString("verifier:\n")
	buf.WriteString(fmt.Sprintf("  result: %s\n", strconv.Quote(s.Verifier.Result)))
	if len(s.Verifier.Argv) > 0 {
		buf.WriteString("  argv:\n")
		for _, a := range s.Verifier.Argv {
			buf.WriteString(fmt.Sprintf("    - %s\n", strconv.Quote(a)))
		}
	}
	if s.Verifier.WorkingDir != "" {
		buf.WriteString(fmt.Sprintf("  working_dir: %s\n", strconv.Quote(s.Verifier.WorkingDir)))
	}
	if len(s.Verifier.EnvAllowlist) > 0 {
		buf.WriteString("  env_allowlist:\n")
		for _, e := range s.Verifier.EnvAllowlist {
			buf.WriteString(fmt.Sprintf("    - %s\n", strconv.Quote(e)))
		}
	}
	if s.Verifier.CommandHash != "" {
		buf.WriteString(fmt.Sprintf("  command_hash: %s\n", strconv.Quote(s.Verifier.CommandHash)))
	}
	if s.Verifier.OutputExcerpt != "" {
		buf.WriteString(fmt.Sprintf("  output_excerpt: %s\n", strconv.Quote(s.Verifier.OutputExcerpt)))
	}
	if s.Verifier.OutputHash != "" {
		buf.WriteString(fmt.Sprintf("  output_hash: %s\n", strconv.Quote(s.Verifier.OutputHash)))
	}

	// Baseline
	buf.WriteString("baseline:\n")
	if s.Baseline.RecordedAt != "" {
		buf.WriteString(fmt.Sprintf("  recorded_at: %s\n", strconv.Quote(s.Baseline.RecordedAt)))
	}
	if s.Baseline.CommandHash != "" {
		buf.WriteString(fmt.Sprintf("  command_hash: %s\n", strconv.Quote(s.Baseline.CommandHash)))
	}
	for p, h := range s.Baseline.AnchorHashes {
		buf.WriteString(fmt.Sprintf("  %s: %s\n", p, strconv.Quote(h)))
	}

	// Review
	buf.WriteString("review:\n")
	buf.WriteString(fmt.Sprintf("  status: %s\n", strconv.Quote(s.Review.Status)))
	if s.Review.Reviewer != "" {
		buf.WriteString(fmt.Sprintf("  reviewer: %s\n", strconv.Quote(s.Review.Reviewer)))
	}
	if s.Review.Date != "" {
		buf.WriteString(fmt.Sprintf("  date: %s\n", strconv.Quote(s.Review.Date)))
	}
	if s.Review.Rationale != "" {
		buf.WriteString(fmt.Sprintf("  rationale: %s\n", strconv.Quote(s.Review.Rationale)))
	}

	// Lifecycle
	buf.WriteString("lifecycle:\n")
	buf.WriteString(fmt.Sprintf("  state: %s\n", strconv.Quote(s.Lifecycle.State)))
	if s.Lifecycle.Provenance != "" {
		buf.WriteString(fmt.Sprintf("  provenance: %s\n", strconv.Quote(s.Lifecycle.Provenance)))
	}
	if s.Lifecycle.SupersededBy != "" {
		buf.WriteString(fmt.Sprintf("  superseded_by: %s\n", strconv.Quote(s.Lifecycle.SupersededBy)))
	}

	buf.WriteString("---\n\n")
	if s.Body != "" {
		buf.WriteString(s.Body)
		buf.WriteString("\n")
	} else if s.Lesson != "" {
		buf.WriteString(s.Lesson)
		buf.WriteString("\n")
	}

	return buf.Bytes(), nil
}
