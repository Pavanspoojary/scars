package scar

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

// FailureContext holds structured parsed signals from compiler/runtime failure logs.
type FailureContext struct {
	Language        string   `json:"language"`
	NormalizedError string   `json:"normalized_error"`
	Files           []string `json:"files"`
	Symbols         []string `json:"symbols"`
	Kind            string   `json:"kind"`
}

// TriageResult represents a diagnosed failure match.
type TriageResult struct {
	MatchedSignature string          `json:"matched_signature,omitempty"`
	ExtractedError   string          `json:"extracted_error"`
	Context          *FailureContext `json:"context,omitempty"`
	Matches          []ScoredScar    `json:"matches"`
	SuggestedCommand string          `json:"suggested_command,omitempty"`
}

var (
	// Go regexes
	goPanicRe   = regexp.MustCompile(`^(panic|fatal error):\s*(.*)`)
	goFileLocRe = regexp.MustCompile(`^\s*([^\s:]+\.go):(\d+)`)
	goSymRe     = regexp.MustCompile(`^([a-zA-Z0-9_./\(\)\*]+)\(.*\)$`)

	// Python regexes
	pyTracebackRe = regexp.MustCompile(`^Traceback \(most recent call last\):`)
	pyFrameRe     = regexp.MustCompile(`^\s*File "([^"]+\.py)", line (\d+)(?:, in (\w+))?`)
	pyErrorRe     = regexp.MustCompile(`^([A-Za-z_]\w*(?:Error|Exception|Interrupt|Exit)):\s*(.*)`)

	// JS/TS regexes
	jsFrameRe = regexp.MustCompile(`^\s*at (?:async )?(?:([a-zA-Z0-9_$.]+) )?\(?([^():]+?\.[jt]sx?):(\d+)(?::\d+)?\)?`)
	jsErrorRe = regexp.MustCompile(`^([A-Za-z_]\w*(?:Error|Exception)):\s*(.*)`)

	// Rust regexes
	rustPanicRe = regexp.MustCompile(`thread '([^']*)' panicked at (?:'([^']*)', )?([^:]+\.rs):(\d+)`)

	// Generic test failure regexes
	testFailRe = regexp.MustCompile(`^---\s*FAIL:\s*(\w+)`)
)

// ParseFailureLog parses raw compiler, runtime, or test output across Go, Python, TS/JS, and Rust.
func ParseFailureLog(log string) FailureContext {
	ctx := FailureContext{
		Language: "generic",
		Files:    []string{},
		Symbols:  []string{},
	}

	fileSet := make(map[string]bool)
	symSet := make(map[string]bool)
	var keyErrors []string

	scanner := bufio.NewScanner(strings.NewReader(log))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// 1. Go patterns
		if m := goPanicRe.FindStringSubmatch(line); len(m) > 0 {
			ctx.Language = "go"
			ctx.Kind = m[1]
			keyErrors = append(keyErrors, line)
			continue
		}
		if m := goFileLocRe.FindStringSubmatch(line); len(m) > 0 {
			ctx.Language = "go"
			f := m[1]
			if !fileSet[f] {
				fileSet[f] = true
				ctx.Files = append(ctx.Files, f)
			}
			continue
		}
		if m := goSymRe.FindStringSubmatch(line); len(m) > 0 {
			sym := m[1]
			if !symSet[sym] && !strings.Contains(sym, "goroutine") {
				symSet[sym] = true
				ctx.Symbols = append(ctx.Symbols, sym)
			}
			continue
		}

		// 2. Python patterns
		if pyTracebackRe.MatchString(line) {
			ctx.Language = "python"
			ctx.Kind = "traceback"
			continue
		}
		if m := pyFrameRe.FindStringSubmatch(line); len(m) > 0 {
			ctx.Language = "python"
			f := m[1]
			if !fileSet[f] {
				fileSet[f] = true
				ctx.Files = append(ctx.Files, f)
			}
			if len(m) > 3 && m[3] != "" && !symSet[m[3]] {
				symSet[m[3]] = true
				ctx.Symbols = append(ctx.Symbols, m[3])
			}
			continue
		}
		if m := pyErrorRe.FindStringSubmatch(line); len(m) > 0 {
			ctx.Language = "python"
			ctx.Kind = m[1]
			keyErrors = append(keyErrors, line)
			continue
		}

		// 3. JS/TS patterns
		if m := jsFrameRe.FindStringSubmatch(line); len(m) > 0 {
			ctx.Language = "javascript"
			sym := m[1]
			f := m[2]
			if sym != "" && !symSet[sym] {
				symSet[sym] = true
				ctx.Symbols = append(ctx.Symbols, sym)
			}
			if f != "" && !fileSet[f] {
				fileSet[f] = true
				ctx.Files = append(ctx.Files, f)
			}
			continue
		}
		if m := jsErrorRe.FindStringSubmatch(line); len(m) > 0 {
			ctx.Language = "javascript"
			ctx.Kind = m[1]
			keyErrors = append(keyErrors, line)
			continue
		}

		// 4. Rust patterns
		if m := rustPanicRe.FindStringSubmatch(line); len(m) > 0 {
			ctx.Language = "rust"
			ctx.Kind = "panic"
			keyErrors = append(keyErrors, line)
			f := m[3]
			if !fileSet[f] {
				fileSet[f] = true
				ctx.Files = append(ctx.Files, f)
			}
			continue
		}

		// 5. Generic test failures
		if m := testFailRe.FindStringSubmatch(line); len(m) > 0 {
			sym := m[1]
			if !symSet[sym] {
				symSet[sym] = true
				ctx.Symbols = append(ctx.Symbols, sym)
			}
			keyErrors = append(keyErrors, line)
		}

		// Fallback keyword scanning
		lower := strings.ToLower(line)
		if strings.Contains(lower, "deadlock") ||
			strings.Contains(lower, "race detected") ||
			strings.Contains(lower, "assertionerror") ||
			strings.Contains(lower, "segmentation fault") ||
			strings.Contains(lower, "sigsegv") {
			keyErrors = append(keyErrors, line)
		}
	}

	if len(keyErrors) > 0 {
		ctx.NormalizedError = strings.Join(keyErrors, "\n")
	} else {
		ctx.NormalizedError = strings.TrimSpace(log)
	}

	return ctx
}

// ExtractErrorSignatures scans compiler or test output to isolate error signatures and traces.
func ExtractErrorSignatures(logOutput string) []string {
	ctx := ParseFailureLog(logOutput)
	if ctx.NormalizedError != "" {
		return strings.Split(ctx.NormalizedError, "\n")
	}
	return []string{}
}

// TriageError matches a raw error log or panic trace against recorded repository scars.
func (s *Store) TriageError(logOutput string) (*TriageResult, error) {
	parsed := ParseFailureLog(logOutput)

	extracted := parsed.NormalizedError
	if extracted == "" {
		extracted = strings.TrimSpace(logOutput)
	}

	q := QueryOptions{
		ErrorSignature: extracted,
		Task:           extracted,
		Files:          parsed.Files,
		Symbols:        parsed.Symbols,
		IncludeStale:   true, // Failure triage must inspect all active scars
		Threshold:      2,
		TopK:           5,
	}

	matches, err := s.Recall(q)
	if err != nil {
		return nil, err
	}

	primarySig := ""
	lines := strings.Split(extracted, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
		primarySig = strings.TrimSpace(lines[0])
	}

	res := &TriageResult{
		MatchedSignature: primarySig,
		ExtractedError:   extracted,
		Context:          &parsed,
		Matches:          matches,
	}

	if len(matches) > 0 {
		best := matches[0].Scar
		if len(best.Verifier.Argv) > 0 {
			res.SuggestedCommand = fmt.Sprintf("git-scars verify %s", best.ID)
		}
	}

	return res, nil
}
