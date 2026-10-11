package scar

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// HealReport summarizes the outcome of an automated execution and triage healing loop.
type HealReport struct {
	Command           []string     `json:"command"`
	Success           bool         `json:"success"`
	ExitCode          int          `json:"exit_code"`
	DurationMs        int64        `json:"duration_ms"`
	Output            string       `json:"output,omitempty"`
	Language          string       `json:"language,omitempty"`
	FailureKind       string       `json:"failure_kind,omitempty"`
	ExtractedError    string       `json:"extracted_error,omitempty"`
	MatchedScars      []ScoredScar `json:"matched_scars,omitempty"`
	RemediationPrompt string       `json:"remediation_prompt,omitempty"`
	SuggestedCommand  string       `json:"suggested_command,omitempty"`
}

// RunHealCommand executes a test or build command, intercepts failures, and triages regressions.
func (s *Store) RunHealCommand(ctx context.Context, argv []string) (*HealReport, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("no command provided to heal")
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = s.RepoRoot

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	err := cmd.Run()
	durationMs := time.Since(start).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	rawOutput := outBuf.String()
	report := &HealReport{
		Command:    argv,
		Success:    exitCode == 0,
		ExitCode:   exitCode,
		DurationMs: durationMs,
		Output:     rawOutput,
	}

	if exitCode == 0 {
		return report, nil
	}

	// Failure triage
	triageRes, triageErr := s.TriageError(rawOutput)
	if triageErr == nil && triageRes != nil {
		if triageRes.Context != nil {
			report.Language = triageRes.Context.Language
			report.FailureKind = triageRes.Context.Kind
		}
		report.ExtractedError = triageRes.ExtractedError
		report.MatchedScars = triageRes.Matches
		report.SuggestedCommand = triageRes.SuggestedCommand

		if len(triageRes.Matches) > 0 {
			var sb strings.Builder
			sb.WriteString("<git_scars_remediation count=\"")
			sb.WriteString(fmt.Sprintf("%d", len(triageRes.Matches)))
			sb.WriteString("\">\n")
			for _, m := range triageRes.Matches {
				sc := m.Scar
				sb.WriteString(fmt.Sprintf("  <scar id=%q score=\"%d\" severity=%q>\n", sc.ID, m.Score, sc.Severity))
				sb.WriteString(fmt.Sprintf("    <title>%s</title>\n", sc.Title))
				sb.WriteString(fmt.Sprintf("    <lesson>%s</lesson>\n", sc.Lesson))
				if sc.FixHint != "" {
					sb.WriteString(fmt.Sprintf("    <fix_hint>%s</fix_hint>\n", sc.FixHint))
				}
				if sc.BadPattern != "" {
					sb.WriteString(fmt.Sprintf("    <dont>%s</dont>\n", sc.BadPattern))
				}
				if sc.GoodPattern != "" {
					sb.WriteString(fmt.Sprintf("    <do>%s</do>\n", sc.GoodPattern))
				}
				if len(sc.Verifier.Argv) > 0 {
					sb.WriteString(fmt.Sprintf("    <verify>%s</verify>\n", strings.Join(sc.Verifier.Argv, " ")))
				}
				sb.WriteString("  </scar>\n")
			}
			sb.WriteString("</git_scars_remediation>")
			report.RemediationPrompt = sb.String()
		}
	}

	return report, nil
}
