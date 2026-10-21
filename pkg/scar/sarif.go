package scar

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// SARIFReport represents a SARIF 2.1.0 log document compatible with GitHub Code Scanning.
type SARIFReport struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []SARIFRun `json:"runs"`
}

type SARIFRun struct {
	Tool    SARIFTool     `json:"tool"`
	Results []SARIFResult `json:"results"`
}

type SARIFTool struct {
	Driver SARIFDriver `json:"driver"`
}

type SARIFDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []SARIFRule `json:"rules"`
}

type SARIFRule struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	ShortDescription SARIFDescription `json:"shortDescription"`
	FullDescription  SARIFDescription `json:"fullDescription,omitempty"`
	Help             SARIFHelp        `json:"help,omitempty"`
}

type SARIFDescription struct {
	Text string `json:"text"`
}

type SARIFHelp struct {
	Text     string `json:"text,omitempty"`
	Markdown string `json:"markdown,omitempty"`
}

type SARIFResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"` // "error", "warning", "note"
	Message   SARIFMessage    `json:"message"`
	Locations []SARIFLocation `json:"locations,omitempty"`
}

type SARIFMessage struct {
	Text string `json:"text"`
}

type SARIFLocation struct {
	PhysicalLocation SARIFPhysicalLocation `json:"physicalLocation"`
}

type SARIFPhysicalLocation struct {
	ArtifactLocation SARIFArtifactLocation `json:"artifactLocation"`
	Region           *SARIFRegion          `json:"region,omitempty"`
}

type SARIFArtifactLocation struct {
	URI string `json:"uri"`
}

type SARIFRegion struct {
	StartLine int `json:"startLine"`
}

// sortedRules returns the rules in a stable order so that repeated SARIF
// reports of the same findings are byte-identical. Iterating a map directly
// produces a different ordering on every run.
func sortedRules(rulesMap map[string]SARIFRule) []SARIFRule {
	ids := make([]string, 0, len(rulesMap))
	for id := range rulesMap {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	rules := make([]SARIFRule, 0, len(ids))
	for _, id := range ids {
		rules = append(rules, rulesMap[id])
	}
	return rules
}

// FormatEnforceSARIF formats pre-edit enforcement check results into SARIF 2.1.0 JSON.
func FormatEnforceSARIF(res *EnforceResult) ([]byte, error) {
	rulesMap := make(map[string]SARIFRule)
	var results []SARIFResult

	for _, sc := range res.BlockingScars {
		ruleID := "git-scars/" + sc.Scar.ID
		level := "warning"
		if sc.Scar.Severity == SeverityCritical || sc.Scar.Severity == SeverityHigh {
			level = "error"
		}

		if _, exists := rulesMap[ruleID]; !exists {
			mdHelp := fmt.Sprintf("### %s\n\n**Lesson**: %s\n\n", sc.Scar.Title, sc.Scar.Lesson)
			if sc.Scar.BadPattern != "" {
				mdHelp += fmt.Sprintf("**Avoid**:\n```\n%s\n```\n\n", sc.Scar.BadPattern)
			}
			if sc.Scar.GoodPattern != "" {
				mdHelp += fmt.Sprintf("**Recommended**:\n```\n%s\n```\n\n", sc.Scar.GoodPattern)
			}
			if len(sc.Scar.Verifier.Argv) > 0 {
				mdHelp += fmt.Sprintf("**Verifier**: `%s`\n", strings.Join(sc.Scar.Verifier.Argv, " "))
			}

			rulesMap[ruleID] = SARIFRule{
				ID:   ruleID,
				Name: sc.Scar.Title,
				ShortDescription: SARIFDescription{
					Text: sc.Scar.Title,
				},
				FullDescription: SARIFDescription{
					Text: sc.Scar.Lesson,
				},
				Help: SARIFHelp{
					Markdown: mdHelp,
					Text:     sc.Scar.Lesson,
				},
			}
		}

		msg := fmt.Sprintf("[%s] %s: %s", strings.ToUpper(sc.Scar.Severity), sc.Scar.Title, sc.Scar.Lesson)
		if sc.Scar.FixHint != "" {
			msg += " (Fix: " + sc.Scar.FixHint + ")"
		}

		// Emit location for each anchored file
		for _, f := range sc.Scar.Anchors.Files {
			results = append(results, SARIFResult{
				RuleID: ruleID,
				Level:  level,
				Message: SARIFMessage{
					Text: msg,
				},
				Locations: []SARIFLocation{
					{
						PhysicalLocation: SARIFPhysicalLocation{
							ArtifactLocation: SARIFArtifactLocation{URI: f},
							Region:           &SARIFRegion{StartLine: 1},
						},
					},
				},
			})
		}
	}

	rules := sortedRules(rulesMap)

	report := SARIFReport{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []SARIFRun{
			{
				Tool: SARIFTool{
					Driver: SARIFDriver{
						Name:           "git-scars",
						Version:        Version,
						InformationURI: "https://github.com/Pavanspoojary/scars",
						Rules:          rules,
					},
				},
				Results: results,
			},
		},
	}

	return json.MarshalIndent(report, "", "  ")
}

// FormatCheckSARIF formats check command findings and anchor drift into SARIF 2.1.0 JSON.
func FormatCheckSARIF(scars []ScarSummary, invariantErrors []string) ([]byte, error) {
	rulesMap := make(map[string]SARIFRule)
	var results []SARIFResult

	for _, item := range scars {
		sc := item.Scar
		if sc.Lifecycle.State != LifecycleActive {
			continue
		}

		if item.Freshness == FreshnessChanged {
			ruleID := "git-scars/anchor-drift"
			if _, exists := rulesMap[ruleID]; !exists {
				rulesMap[ruleID] = SARIFRule{
					ID:   ruleID,
					Name: "Anchor Code Drift",
					ShortDescription: SARIFDescription{
						Text: "Anchor code modified since verification baseline",
					},
					Help: SARIFHelp{
						Markdown: "Anchor code has changed. Inspect diff with `git-scars diff <id>` and reaffirm with `git-scars reaffirm <id>`.",
					},
				}
			}

			msg := fmt.Sprintf("Active scar %s (%s) anchor code modified since baseline.", sc.ID, sc.Title)
			for _, f := range sc.Anchors.Files {
				results = append(results, SARIFResult{
					RuleID: ruleID,
					Level:  "warning",
					Message: SARIFMessage{
						Text: msg,
					},
					Locations: []SARIFLocation{
						{
							PhysicalLocation: SARIFPhysicalLocation{
								ArtifactLocation: SARIFArtifactLocation{URI: f},
								Region:           &SARIFRegion{StartLine: 1},
							},
						},
					},
				})
			}
		}
	}

	for _, inv := range invariantErrors {
		ruleID := "git-scars/invariant-violation"
		if _, exists := rulesMap[ruleID]; !exists {
			rulesMap[ruleID] = SARIFRule{
				ID:   ruleID,
				Name: "Lifecycle Invariant Violation",
				ShortDescription: SARIFDescription{
					Text: "Scar violates core lifecycle or verification invariants",
				},
			}
		}

		results = append(results, SARIFResult{
			RuleID: ruleID,
			Level:  "error",
			Message: SARIFMessage{
				Text: inv,
			},
		})
	}

	rules := sortedRules(rulesMap)

	report := SARIFReport{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []SARIFRun{
			{
				Tool: SARIFTool{
					Driver: SARIFDriver{
						Name:           "git-scars",
						Version:        Version,
						InformationURI: "https://github.com/Pavanspoojary/scars",
						Rules:          rules,
					},
				},
				Results: results,
			},
		},
	}

	return json.MarshalIndent(report, "", "  ")
}

// GitHubWorkflowYAML returns a production-ready GitHub Actions workflow for GitScars.
func GitHubWorkflowYAML() string {
	return `name: GitScars Regression Guardrail

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  guardrail:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      security-events: write

    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Build git-scars
        run: go install github.com/Pavanspoojary/scars/cmd/git-scars@latest

      - name: Run git-scars check
        run: git-scars check --format sarif > git-scars.sarif
        continue-on-error: true

      - name: Upload SARIF report
        uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: git-scars.sarif
`
}
