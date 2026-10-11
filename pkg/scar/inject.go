package scar

import (
	"fmt"
	"math"
	"strings"
)

// InjectOptions defines configuration for generating agent prompt snippets.
type InjectOptions struct {
	Files        []string `json:"files,omitempty"`
	Task         string   `json:"task,omitempty"`
	Symbols      []string `json:"symbols,omitempty"`
	BudgetTokens int      `json:"budget_tokens,omitempty"` // max estimated tokens (default: 400)
	Format       string   `json:"format,omitempty"`        // "xml" (default) or "markdown"
	IncludeStale bool     `json:"include_stale,omitempty"`
}

// EstimateTokens provides a conservative token count heuristic (~4 characters per token).
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	return int(math.Ceil(float64(len(text)) / 4.0))
}

// InjectGuardrails generates a token-budgeted prompt snippet of relevant scars for AI agents.
func (s *Store) InjectGuardrails(opts InjectOptions) (string, error) {
	budget := opts.BudgetTokens
	if budget <= 0 {
		budget = 400 // standard default budget
	}

	format := strings.ToLower(opts.Format)
	if format == "" {
		format = "xml"
	}

	q := QueryOptions{
		Files:        opts.Files,
		Task:         opts.Task,
		Symbols:      opts.Symbols,
		IncludeStale: opts.IncludeStale,
		Threshold:    2,  // sensitive threshold to capture relevant guardrails
		TopK:         10, // gather up to 10 candidates to fill budget
	}

	scored, err := s.Recall(q)
	if err != nil {
		return "", err
	}

	if len(scored) == 0 {
		return "", nil // abstain cleanly with zero tokens
	}

	if format == "markdown" {
		return formatMarkdownInjection(scored, budget), nil
	}

	return formatXMLInjection(scored, budget), nil
}

func formatXMLInjection(scored []ScoredScar, budgetTokens int) string {
	var included []string
	currentTokens := EstimateTokens("<git_scars>\n</git_scars>")
	omitted := 0

	for _, sc := range scored {
		scar := sc.Scar
		filesAttr := strings.Join(scar.Anchors.Files, ", ")
		symbolsAttr := strings.Join(scar.Anchors.Symbols, ", ")

		var entry strings.Builder
		entry.WriteString(fmt.Sprintf("  <scar id=%q score=%q>\n", scar.ID, fmt.Sprintf("%d", sc.Score)))
		entry.WriteString(fmt.Sprintf("    <title>%s</title>\n", escapeXML(scar.Title)))
		entry.WriteString(fmt.Sprintf("    <lesson>%s</lesson>\n", escapeXML(scar.Lesson)))
		if scar.Applicability != "" {
			entry.WriteString(fmt.Sprintf("    <applicability>%s</applicability>\n", escapeXML(scar.Applicability)))
		}
		if filesAttr != "" || symbolsAttr != "" {
			entry.WriteString(fmt.Sprintf("    <anchors files=%q symbols=%q />\n", filesAttr, symbolsAttr))
		}
		entry.WriteString("  </scar>")

		entryStr := entry.String()
		entryTokens := EstimateTokens(entryStr + "\n")

		if currentTokens+entryTokens > budgetTokens && len(included) > 0 {
			omitted++
			continue
		}

		included = append(included, entryStr)
		currentTokens += entryTokens
	}

	if len(included) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<git_scars count=%q>\n", fmt.Sprintf("%d", len(included))))
	sb.WriteString(strings.Join(included, "\n"))
	if omitted > 0 {
		sb.WriteString(fmt.Sprintf("\n  <!-- %d additional scar(s) omitted to respect token budget -->", omitted))
	}
	sb.WriteString("\n</git_scars>")

	return sb.String()
}

func formatMarkdownInjection(scored []ScoredScar, budgetTokens int) string {
	var included []string
	header := "### 🛡️ Repository Scars & Guardrails\n"
	currentTokens := EstimateTokens(header)
	omitted := 0

	for _, sc := range scored {
		scar := sc.Scar
		var entry strings.Builder
		anchorInfo := ""
		if len(scar.Anchors.Files) > 0 {
			anchorInfo = fmt.Sprintf(" (%s)", strings.Join(scar.Anchors.Files, ", "))
		}
		entry.WriteString(fmt.Sprintf("- **%s**%s: %s", scar.ID, anchorInfo, scar.Lesson))
		if scar.Applicability != "" {
			entry.WriteString(fmt.Sprintf("\n  *Applicability*: %s", scar.Applicability))
		}

		entryStr := entry.String()
		entryTokens := EstimateTokens(entryStr + "\n")

		if currentTokens+entryTokens > budgetTokens && len(included) > 0 {
			omitted++
			continue
		}

		included = append(included, entryStr)
		currentTokens += entryTokens
	}

	if len(included) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString(strings.Join(included, "\n"))
	if omitted > 0 {
		sb.WriteString(fmt.Sprintf("\n\n*(%d additional scar(s) omitted to stay within token budget)*", omitted))
	}

	return sb.String()
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
