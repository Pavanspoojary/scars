package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"
)

const version = "0.1.0-spike"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 1 {
		printUsage()
		return nil
	}

	repoRoot := findRepoRoot()

	switch args[0] {
	case "init":
		return runInit(repoRoot)
	case "add":
		return runAdd(repoRoot, args[1:])
	case "list":
		return runList(repoRoot, args[1:])
	case "verify":
		return runVerify(repoRoot, args[1:])
	case "version", "--version", "-v":
		fmt.Printf("git-scars version %s\n", version)
		return nil
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func printUsage() {
	fmt.Printf(`GitScars CLI (%s) - Failure-mode memory and regression guardrails for coding agents

Usage:
  git-scars <command> [arguments]

Commands:
  init            Initialize .gitscars/ repository metadata and config
  add             Add a new scar with frontmatter and body
  list            List recorded scars with anchors, status, and drift
  verify          Execute verifier command(s) against current tree
  version         Print git-scars version
  help            Show this help text
`, version)
}

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	curr := dir
	for {
		if _, err := os.Stat(filepath.Join(curr, ".gitscars")); err == nil {
			return curr
		}
		if _, err := os.Stat(filepath.Join(curr, ".git")); err == nil {
			return curr
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return dir
}

func runInit(root string) error {
	scarsDir := filepath.Join(root, ".gitscars", "scars")
	if err := os.MkdirAll(scarsDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	configFile := filepath.Join(root, ".gitscars", "config.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		content := `# GitScars configuration
version: 1
scars_dir: scars
abstention_threshold: 0.7
`
		if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write config.yaml: %w", err)
		}
		fmt.Printf("Initialized GitScars repository at %s\n", filepath.Join(root, ".gitscars"))
	} else {
		fmt.Printf("GitScars already initialized at %s\n", filepath.Join(root, ".gitscars"))
	}
	return nil
}

func runAdd(root string, args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	idFlag := fs.String("id", "", "Scar ID (e.g. scar-001)")
	titleFlag := fs.String("title", "", "Short title or summary")
	pathFlag := fs.String("path", "", "Anchored file path relative to repo root")
	symbolFlag := fs.String("symbol", "", "Anchored symbol or function name")
	commitFlag := fs.String("commit", "", "Git commit hash of anchor")
	verifierFlag := fs.String("verifier", "", "Verifier shell command (reproducer/regression test)")
	bodyFlag := fs.String("body", "", "Markdown body details")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *titleFlag == "" {
		return fmt.Errorf("flag -title is required")
	}

	scarsDir := filepath.Join(root, ".gitscars", "scars")
	if err := os.MkdirAll(scarsDir, 0755); err != nil {
		return fmt.Errorf("failed to create scars directory: %w", err)
	}

	id := *idFlag
	if id == "" {
		entries, _ := os.ReadDir(scarsDir)
		count := 0
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".md") {
				count++
			}
		}
		id = fmt.Sprintf("scar-%03d", count+1)
	}

	commit := *commitFlag
	if commit == "" && *pathFlag != "" {
		cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
		cmd.Dir = root
		if out, err := cmd.Output(); err == nil {
			commit = strings.TrimSpace(string(out))
		}
	}

	scar := &Scar{
		ID:      id,
		Title:   *titleFlag,
		Created: time.Now().UTC().Format(time.RFC3339),
		Status:  "active",
		Verifier: Verifier{
			Command:  *verifierFlag,
			Expected: "pass",
		},
	}

	if *pathFlag != "" {
		scar.Anchors = append(scar.Anchors, Anchor{
			Path:   *pathFlag,
			Symbol: *symbolFlag,
			Commit: commit,
		})
	}

	if *bodyFlag != "" {
		scar.Body = *bodyFlag
	} else {
		stat, _ := os.Stdin.Stat()
		if stat != nil && (stat.Mode()&os.ModeCharDevice) == 0 {
			b, _ := io.ReadAll(os.Stdin)
			scar.Body = string(b)
		} else {
			scar.Body = `### Symptom & Root Cause
Describe how this failure manifested and what underlying condition caused it.

### Footgun / Guardrail
Describe rules or architectural guardrails to prevent regression.`
		}
	}

	targetPath := filepath.Join(scarsDir, id+".md")
	content := FormatScar(scar)
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write scar file: %w", err)
	}

	fmt.Printf("Created scar %s at %s\n", id, filepath.Join(".gitscars", "scars", id+".md"))
	return nil
}

func runList(root string, args []string) error {
	scarsDir := filepath.Join(root, ".gitscars", "scars")
	entries, err := os.ReadDir(scarsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No scars found. Run 'git-scars init' and 'git-scars add'.")
			return nil
		}
		return fmt.Errorf("failed to read scars directory: %w", err)
	}

	var scars []*Scar
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(scarsDir, entry.Name()))
		if err != nil {
			continue
		}
		s, err := ParseScar(data)
		if err != nil {
			continue
		}
		scars = append(scars, s)
	}

	if len(scars) == 0 {
		fmt.Println("No scars found in .gitscars/scars/")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tANCHOR\tVERIFIER\tTITLE")
	for _, s := range scars {
		status := s.Status
		if drifted, reason := CheckDrift(root, s); drifted {
			status = fmt.Sprintf("drifted (%s)", reason)
		}

		anchorStr := "-"
		if len(s.Anchors) > 0 {
			a := s.Anchors[0]
			if a.Symbol != "" {
				anchorStr = fmt.Sprintf("%s:%s", a.Path, a.Symbol)
			} else {
				anchorStr = a.Path
			}
			if len(s.Anchors) > 1 {
				anchorStr += fmt.Sprintf(" (+%d more)", len(s.Anchors)-1)
			}
		}

		verStr := "-"
		if s.Verifier.Command != "" {
			verStr = s.Verifier.Command
			if len(verStr) > 25 {
				verStr = verStr[:22] + "..."
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.ID, status, anchorStr, verStr, s.Title)
	}
	return w.Flush()
}

func runVerify(root string, args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	allFlag := fs.Bool("all", false, "Verify all scars")
	timeoutFlag := fs.Duration("timeout", 60*time.Second, "Maximum duration per verifier check")

	if err := fs.Parse(args); err != nil {
		return err
	}

	targetID := ""
	if fs.NArg() > 0 {
		targetID = fs.Arg(0)
	}

	if targetID == "" && !*allFlag {
		return fmt.Errorf("specify a scar ID (e.g. 'git-scars verify scar-001') or use --all")
	}

	scarsDir := filepath.Join(root, ".gitscars", "scars")
	entries, err := os.ReadDir(scarsDir)
	if err != nil {
		return fmt.Errorf("scars directory not found: %w", err)
	}

	var toVerify []*Scar
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(scarsDir, entry.Name()))
		if err != nil {
			continue
		}
		s, err := ParseScar(data)
		if err != nil {
			continue
		}
		if *allFlag || s.ID == targetID {
			toVerify = append(toVerify, s)
		}
	}

	if len(toVerify) == 0 {
		if targetID != "" {
			return fmt.Errorf("scar %q not found", targetID)
		}
		fmt.Println("No scars to verify.")
		return nil
	}

	fmt.Printf("Verifying %d scar(s)...\n", len(toVerify))
	failed := 0
	passed := 0
	skipped := 0

	for _, s := range toVerify {
		if s.Verifier.Command == "" {
			fmt.Printf("[-] %s: no verifier command defined (SKIPPED)\n", s.ID)
			skipped++
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
		cmd := exec.CommandContext(ctx, "sh", "-c", s.Verifier.Command)
		cmd.Dir = root

		out, err := cmd.CombinedOutput()
		cancel()

		if err != nil {
			fmt.Printf("[FAIL] %s: command: %s\n", s.ID, s.Verifier.Command)
			if len(bytes.TrimSpace(out)) > 0 {
				lines := strings.Split(strings.TrimSpace(string(out)), "\n")
				for _, l := range lines {
					fmt.Printf("       | %s\n", l)
				}
			}
			failed++
		} else {
			fmt.Printf("[PASS] %s: command: %s\n", s.ID, s.Verifier.Command)
			passed++
		}
	}

	fmt.Printf("\nSummary: %d passed, %d failed, %d skipped\n", passed, failed, skipped)
	if failed > 0 {
		return fmt.Errorf("%d verification check(s) failed", failed)
	}
	return nil
}
