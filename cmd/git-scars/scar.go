package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Anchor defines the code location a scar is tied to.
type Anchor struct {
	Path   string
	Symbol string
	Commit string
}

// Verifier defines the reproduction or regression verification check.
type Verifier struct {
	Command  string
	Expected string
}

// Scar represents a single failure-mode record with frontmatter and body.
type Scar struct {
	ID       string
	Title    string
	Created  string
	Status   string
	Anchors  []Anchor
	Verifier Verifier
	Body     string
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// ParseScar parses a Markdown file containing YAML frontmatter into a Scar.
func ParseScar(content []byte) (*Scar, error) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty file")
	}

	firstLine := strings.TrimSpace(scanner.Text())
	if firstLine != "---" {
		return nil, fmt.Errorf("missing frontmatter delimiter '---'")
	}

	scar := &Scar{
		Status: "active",
	}

	inFrontmatter := true
	var bodyLines []string
	var currentSection string
	var currentAnchor *Anchor

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if inFrontmatter {
			if trimmed == "---" {
				inFrontmatter = false
				if currentAnchor != nil {
					scar.Anchors = append(scar.Anchors, *currentAnchor)
					currentAnchor = nil
				}
				continue
			}

			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}

			if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				if currentAnchor != nil {
					scar.Anchors = append(scar.Anchors, *currentAnchor)
					currentAnchor = nil
				}
				parts := strings.SplitN(trimmed, ":", 2)
				key := strings.ToLower(strings.TrimSpace(parts[0]))
				val := ""
				if len(parts) > 1 {
					val = unquote(parts[1])
				}

				switch key {
				case "id":
					scar.ID = val
					currentSection = ""
				case "title":
					scar.Title = val
					currentSection = ""
				case "created":
					scar.Created = val
					currentSection = ""
				case "status":
					scar.Status = val
					currentSection = ""
				case "anchors":
					currentSection = "anchors"
				case "verifier":
					currentSection = "verifier"
				default:
					currentSection = ""
				}
			} else {
				subTrimmed := strings.TrimSpace(line)
				if currentSection == "anchors" {
					if strings.HasPrefix(subTrimmed, "- ") {
						if currentAnchor != nil {
							scar.Anchors = append(scar.Anchors, *currentAnchor)
						}
						currentAnchor = &Anchor{}
						subTrimmed = strings.TrimPrefix(subTrimmed, "- ")
					}
					parts := strings.SplitN(subTrimmed, ":", 2)
					if len(parts) == 2 && currentAnchor != nil {
						k := strings.ToLower(strings.TrimSpace(parts[0]))
						v := unquote(parts[1])
						switch k {
						case "path":
							currentAnchor.Path = v
						case "symbol":
							currentAnchor.Symbol = v
						case "commit":
							currentAnchor.Commit = v
						}
					}
				} else if currentSection == "verifier" {
					parts := strings.SplitN(subTrimmed, ":", 2)
					if len(parts) == 2 {
						k := strings.ToLower(strings.TrimSpace(parts[0]))
						v := unquote(parts[1])
						switch k {
						case "command":
							scar.Verifier.Command = v
						case "expected":
							scar.Verifier.Expected = v
						}
					}
				}
			}
		} else {
			bodyLines = append(bodyLines, line)
		}
	}

	if currentAnchor != nil {
		scar.Anchors = append(scar.Anchors, *currentAnchor)
	}

	if inFrontmatter {
		return nil, fmt.Errorf("unclosed frontmatter: missing ending '---'")
	}

	scar.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
	return scar, scanner.Err()
}

// FormatScar formats a Scar into Markdown with YAML frontmatter.
func FormatScar(s *Scar) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("id: %s\n", s.ID))
	b.WriteString(fmt.Sprintf("title: %q\n", s.Title))
	if s.Created != "" {
		b.WriteString(fmt.Sprintf("created: %s\n", s.Created))
	} else {
		b.WriteString(fmt.Sprintf("created: %s\n", time.Now().UTC().Format(time.RFC3339)))
	}
	if s.Status == "" {
		b.WriteString("status: active\n")
	} else {
		b.WriteString(fmt.Sprintf("status: %s\n", s.Status))
	}

	if len(s.Anchors) > 0 {
		b.WriteString("anchors:\n")
		for _, a := range s.Anchors {
			b.WriteString(fmt.Sprintf("  - path: %s\n", a.Path))
			if a.Symbol != "" {
				b.WriteString(fmt.Sprintf("    symbol: %s\n", a.Symbol))
			}
			if a.Commit != "" {
				b.WriteString(fmt.Sprintf("    commit: %s\n", a.Commit))
			}
		}
	}

	if s.Verifier.Command != "" {
		b.WriteString("verifier:\n")
		b.WriteString(fmt.Sprintf("  command: %s\n", s.Verifier.Command))
		if s.Verifier.Expected != "" {
			b.WriteString(fmt.Sprintf("  expected: %s\n", s.Verifier.Expected))
		} else {
			b.WriteString("  expected: pass\n")
		}
	}

	b.WriteString("---\n\n")
	if s.Body != "" {
		b.WriteString(strings.TrimSpace(s.Body))
		b.WriteString("\n")
	}
	return b.String()
}

// CheckDrift inspects whether an anchor file is missing or has changed since commit.
func CheckDrift(repoRoot string, scar *Scar) (bool, string) {
	for _, a := range scar.Anchors {
		if a.Path == "" {
			continue
		}
		fullPath := filepath.Join(repoRoot, a.Path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return true, fmt.Sprintf("path missing: %s", a.Path)
		}
		if a.Commit != "" {
			cmd := exec.Command("git", "diff", "--name-only", a.Commit, "--", a.Path)
			cmd.Dir = repoRoot
			out, err := cmd.Output()
			if err == nil && len(bytes.TrimSpace(out)) > 0 {
				return true, fmt.Sprintf("modified since %s: %s", a.Commit, a.Path)
			}
		}
	}
	return false, ""
}
