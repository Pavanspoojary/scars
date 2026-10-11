package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/Pavanspoojary/scars/pkg/scar"
)

const version = "1.0.0"

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
	store := scar.NewStore(repoRoot)

	switch args[0] {
	case "init":
		return runInit(store)
	case "add":
		return runAdd(store, args[1:])
	case "list":
		return runList(store, args[1:])
	case "show":
		return runShow(store, args[1:])
	case "promote":
		return runPromote(store, args[1:])
	case "verify":
		return runVerify(store, args[1:])
	case "recall":
		return runRecall(store, args[1:])
	case "check":
		return runCheck(store, args[1:])
	case "lint":
		return runLint(store, args[1:])
	case "feedback":
		return runFeedback(store, args[1:])
	case "retire":
		return runRetire(store, args[1:])
	case "reaffirm":
		return runReaffirm(store, args[1:])
	case "mcp":
		return store.ServeMCP(os.Stdin, os.Stdout)
	case "hook":
		return runHook(store, args[1:])
	case "index":
		return runIndex(store, args[1:])
	case "diff":
		return runDiff(store, args[1:])
	case "stats":
		return runStats(store, args[1:])
	case "doctor":
		return runDoctor(store, args[1:])
	case "inject":
		return runInject(store, args[1:])
	case "sync":
		return runSync(store, args[1:])
	case "triage":
		return runTriage(store, args[1:])
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
	fmt.Printf(`git-scars (%s) - Git-friendly failure memory and regression guardrails

Usage:
  git-scars <command> [arguments]

Lifecycle Commands:
  init            Initialize .gitscars/ repository metadata and config
  add             Create a new scar draft (interactive -i or --title/--lesson/--files)
  list            List recorded scars with lifecycle, review, and dynamic freshness
  show <id>       Display full metadata and lesson for a scar
  diff <id>       Show diff between anchor files and recorded baseline
  promote <id>    Promote a scar to active state (enforces review & verifier invariants)
  reaffirm <id>   Reaffirm a scar after anchor code drift with human rationale
  retire <id>     Retire a scar that is no longer applicable
  verify <id>     Execute verifier with safety checks and record evidence
  recall          Retrieve relevant scars (--files, --diff, --stdin, --json)
  inject          Generate token-budgeted prompt guardrails for AI agents
  sync            Synchronize active scars into AGENTS.md / CLAUDE.md
  triage          Diagnose test failures and panic traces against scars (--error/stdin)
  check           Read-only verification of scar freshness and invariants
  lint            Audit scars for security, wildcards, and schema validity
  doctor          Run repository diagnostics and health checks (--json)
  stats           Display repository metrics and feedback statistics (--json)
  index           Rebuild .gitscars/.local/index.json fast cache
  hook            Manage Git pre-commit hook (install, uninstall, status)
  mcp             Run read-only stdio Model Context Protocol (MCP) server
  feedback <id>   Record feedback on retrieval utility (--rating, --notes)
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
		if _, err := os.Stat(filepath.Join(curr, scar.DefaultStoreDir)); err == nil {
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

func runInit(store *scar.Store) error {
	if err := store.Init(); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}
	fmt.Printf("Initialized GitScars in %s/.gitscars\n", store.RepoRoot)
	return nil
}

func runAdd(store *scar.Store, args []string) error {
	if len(args) > 0 && args[0] == "from-commit" {
		if len(args) < 2 {
			return fmt.Errorf("usage: git-scars add from-commit <sha>")
		}
		sha := args[1]
		draft, path, err := store.DraftFromCommit(sha)
		if err != nil {
			return fmt.Errorf("failed capturing scar from commit %s: %w", sha, err)
		}
		fmt.Printf("Created draft scar from commit %s:\nID: %s\nPath: %s\nBase Commit: %s\nAnchored Files: %s\nInferred Verifier: %s\nStatus: %s (review: %s, verifier: %s)\n",
			sha, draft.ID, path, draft.BaseCommit, strings.Join(draft.Anchors.Files, ", "), strings.Join(draft.Verifier.Argv, " "),
			draft.Lifecycle.State, draft.Review.Status, draft.Verifier.Result)
		return nil
	}

	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	title := fs.String("title", "", "Descriptive failure title")
	lesson := fs.String("lesson", "", "Concise prevention lesson")
	filesStr := fs.String("files", "", "Comma-separated list of anchored files")
	interactive := fs.Bool("interactive", false, "Interactive prompt mode")
	fs.BoolVar(interactive, "i", false, "Interactive prompt mode (shorthand)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *interactive {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Print("Scar Title: ")
		if scanner.Scan() {
			*title = strings.TrimSpace(scanner.Text())
		}
		if *title == "" {
			return fmt.Errorf("title cannot be empty")
		}

		fmt.Print("Prevention Lesson: ")
		if scanner.Scan() {
			*lesson = strings.TrimSpace(scanner.Text())
		}

		fmt.Print("Anchored Files (comma-separated, optional): ")
		if scanner.Scan() {
			*filesStr = strings.TrimSpace(scanner.Text())
		}
	}

	if *title == "" {
		return fmt.Errorf("add requires --title (e.g. git-scars add --title \"Nil pointer in auth\" --lesson \"Check nil\" --files \"auth.go\"), interactive mode (-i), or add from-commit <sha>")
	}

	var files []string
	if *filesStr != "" {
		for _, f := range strings.Split(*filesStr, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				files = append(files, f)
			}
		}
	}

	draft, path, err := store.CreateDraft(*title, *lesson, files)
	if err != nil {
		return fmt.Errorf("failed creating draft: %w", err)
	}

	fmt.Printf("Created draft scar: %s\nPath: %s\nStatus: %s (review: %s, verifier: %s)\n",
		draft.ID, path, draft.Lifecycle.State, draft.Review.Status, draft.Verifier.Result)
	return nil
}

func runVerify(store *scar.Store, args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	interactive := fs.Bool("interactive", false, "Interactively approve and trust verifier command")

	if len(args) < 1 {
		return fmt.Errorf("usage: git-scars verify <id> [--interactive]")
	}
	id := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	s, path, err := store.LoadScar(id)
	if err != nil {
		return err
	}

	ctx := context.Background()
	outcome, err := store.VerifyFailToPass(ctx, s, nil, *interactive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verification notice: %v\n", err)
	}

	_ = store.RecordBaseline(s, s.Verifier.CommandHash)
	if _, saveErr := store.SaveScar(s); saveErr != nil {
		return fmt.Errorf("failed saving verified scar: %w", saveErr)
	}

	fmt.Printf("Recorded verification for scar %s:\nResult: %s\nCommand Hash: %s\nSaved to: %s\n",
		s.ID, outcome, s.Verifier.CommandHash, path)
	if s.Verifier.OutputExcerpt != "" {
		fmt.Printf("Output excerpt:\n%s\n", s.Verifier.OutputExcerpt)
	}
	fmt.Printf("\nNote: Verification recorded. A human reviewer must run 'git-scars promote %s' to approve and activate.\n", s.ID)
	return nil
}

func runList(store *scar.Store, args []string) error {
	scars, err := store.ListScars()
	if err != nil {
		return fmt.Errorf("failed listing scars: %w", err)
	}

	if len(scars) == 0 {
		fmt.Println("No scars found in .gitscars/scars. Run 'git-scars add' to record a lesson.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tSTATE\tREVIEW\tVERIFIER\tFRESHNESS")
	for _, item := range scars {
		s := item.Scar
		title := s.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			s.ID, title, s.Lifecycle.State, s.Review.Status, s.Verifier.Result, item.Freshness)
	}
	return w.Flush()
}

func runShow(store *scar.Store, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: git-scars show <id>")
	}
	id := args[0]
	s, path, err := store.LoadScar(id)
	if err != nil {
		return err
	}

	freshness := store.ComputeFreshness(s)

	fmt.Printf("Scar: %s\n", s.ID)
	fmt.Printf("Path: %s\n", path)
	fmt.Printf("Title: %s\n", s.Title)
	fmt.Printf("Lifecycle State: %s\n", s.Lifecycle.State)
	fmt.Printf("Review Status: %s (Reviewer: %s, Date: %s)\n", s.Review.Status, s.Review.Reviewer, s.Review.Date)
	if s.Review.Rationale != "" {
		fmt.Printf("Review Rationale: %s\n", s.Review.Rationale)
	}
	fmt.Printf("Verifier Result: %s\n", s.Verifier.Result)
	if len(s.Verifier.Argv) > 0 {
		fmt.Printf("Verifier Command: %s\n", strings.Join(s.Verifier.Argv, " "))
	}
	fmt.Printf("Dynamic Freshness: %s\n", freshness)
	if len(s.Anchors.Files) > 0 {
		fmt.Printf("Anchored Files:\n")
		for _, f := range s.Anchors.Files {
			fmt.Printf("  - %s\n", f)
		}
	}
	if len(s.Anchors.Symbols) > 0 {
		fmt.Printf("Anchored Symbols: %s\n", strings.Join(s.Anchors.Symbols, ", "))
	}
	fmt.Printf("\n--- Lesson ---\n%s\n", s.Lesson)
	return nil
}

func runPromote(store *scar.Store, args []string) error {
	fs := flag.NewFlagSet("promote", flag.ContinueOnError)
	status := fs.String("status", scar.LifecycleActive, "Target lifecycle state (active, draft)")
	reviewer := fs.String("reviewer", "", "Reviewer identifier (e.g. GitHub handle or name)")
	rationale := fs.String("rationale", "", "Review approval rationale or exception justification")

	if len(args) < 1 {
		return fmt.Errorf("usage: git-scars promote <id> [--status active] [--reviewer <name>] [--rationale <reason>]")
	}
	id := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	s, path, err := store.LoadScar(id)
	if err != nil {
		return err
	}

	if err := s.Promote(*status, *reviewer, *rationale); err != nil {
		return fmt.Errorf("promotion rejected by invariants: %w", err)
	}

	if _, err := store.SaveScar(s); err != nil {
		return fmt.Errorf("failed saving promoted scar: %w", err)
	}

	fmt.Printf("Successfully promoted scar %s to state %q\nSaved to: %s\n", s.ID, s.Lifecycle.State, path)
	return nil
}

func runRetire(store *scar.Store, args []string) error {
	fs := flag.NewFlagSet("retire", flag.ContinueOnError)
	rationale := fs.String("rationale", "", "Retirement rationale")

	if len(args) < 1 {
		return fmt.Errorf("usage: git-scars retire <id> [--rationale <reason>]")
	}
	id := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	s, _, err := store.LoadScar(id)
	if err != nil {
		return err
	}

	if err := s.Retire(*rationale); err != nil {
		return fmt.Errorf("retire rejected: %w", err)
	}

	if _, err := store.SaveScar(s); err != nil {
		return fmt.Errorf("failed saving retired scar: %w", err)
	}

	fmt.Printf("Retired scar %s\n", s.ID)
	return nil
}

func runReaffirm(store *scar.Store, args []string) error {
	fs := flag.NewFlagSet("reaffirm", flag.ContinueOnError)
	reviewer := fs.String("reviewer", "", "Reviewer performing reaffirmation")
	rationale := fs.String("rationale", "", "Documented rationale explaining why lesson remains applicable")

	if len(args) < 1 {
		return fmt.Errorf("usage: git-scars reaffirm <id> --reviewer <name> --rationale <reason>")
	}
	id := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	reaffirmed, err := store.Reaffirm(id, *reviewer, *rationale)
	if err != nil {
		return fmt.Errorf("reaffirmation failed: %w", err)
	}

	fmt.Printf("Reaffirmed scar %s by %s. Anchor baseline updated to current tree.\n", reaffirmed.ID, *reviewer)
	return nil
}

func runRecall(store *scar.Store, args []string) error {
	fs := flag.NewFlagSet("recall", flag.ContinueOnError)
	task := fs.String("task", "", "Coding task description")
	filesStr := fs.String("files", "", "Comma-separated target files")
	symbolsStr := fs.String("symbols", "", "Comma-separated symbol anchors")
	commandsStr := fs.String("commands", "", "Comma-separated command anchors")
	diffRef := fs.String("diff", "", "Git ref to extract changed files from (e.g. HEAD~1, origin/main)")
	useStdin := fs.Bool("stdin", false, "Read error output or stack trace from stdin")
	includeStale := fs.Bool("include-stale", false, "Include changed or unbaselined scars in search candidates")
	asJSON := fs.Bool("json", false, "Emit versioned reposcars.recall.v1 JSON")
	threshold := fs.Int("threshold", 3, "Abstain threshold score")
	topK := fs.Int("top-k", 3, "Maximum scars to return")

	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := scar.QueryOptions{
		Task:         *task,
		DiffRef:      *diffRef,
		IncludeStale: *includeStale,
		Threshold:    *threshold,
		TopK:         *topK,
	}

	if *filesStr != "" {
		for _, f := range strings.Split(*filesStr, ",") {
			if f = strings.TrimSpace(f); f != "" {
				opts.Files = append(opts.Files, f)
			}
		}
	}
	if *symbolsStr != "" {
		for _, s := range strings.Split(*symbolsStr, ",") {
			if s = strings.TrimSpace(s); s != "" {
				opts.Symbols = append(opts.Symbols, s)
			}
		}
	}
	if *commandsStr != "" {
		for _, c := range strings.Split(*commandsStr, ",") {
			if c = strings.TrimSpace(c); c != "" {
				opts.Commands = append(opts.Commands, c)
			}
		}
	}

	if *useStdin {
		stdinBytes, err := io.ReadAll(os.Stdin)
		if err == nil && len(stdinBytes) > 0 {
			opts.ErrorSignature = string(stdinBytes)
		}
	}

	results, err := store.Recall(opts)
	if err != nil {
		return fmt.Errorf("recall failed: %w", err)
	}

	if *asJSON {
		jsonData, err := scar.FormatRecallJSON(opts, results)
		if err != nil {
			return err
		}
		fmt.Println(string(jsonData))
		return nil
	}

	if len(results) == 0 {
		fmt.Println("No applicable scars found (abstaining: score < threshold).")
		return nil
	}

	fmt.Printf("Retrieved %d relevant scar(s):\n\n", len(results))
	for idx, r := range results {
		s := r.Scar
		fmt.Printf("[%d] %s (Score: %d | Freshness: %s)\n", idx+1, s.Title, r.Score, r.Freshness)
		fmt.Printf("    ID: %s\n", s.ID)
		fmt.Printf("    Matched: %s\n", r.Explanation)
		if len(s.Anchors.Files) > 0 {
			fmt.Printf("    Anchored Files: %s\n", strings.Join(s.Anchors.Files, ", "))
		}
		fmt.Printf("    Lesson: %s\n\n", s.Lesson)
	}
	return nil
}

func runCheck(store *scar.Store, args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	diffRef := fs.String("diff", "", "Optional git diff ref to evaluate anchor changes")
	if err := fs.Parse(args); err != nil {
		return err
	}

	res, err := store.RunReadOnlyCheck(*diffRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Operational error: %v\n", err)
		os.Exit(3)
	}

	fmt.Printf("=== GitScars Read-Only Check ===\n")
	fmt.Printf("Total Scars: %d | Current: %d | Changed (Stale): %d\n",
		res.TotalScars, res.CurrentCount, res.ChangedCount)

	if len(res.InvalidScars) > 0 {
		fmt.Printf("\n[!] Invariant Violations (%d):\n", len(res.InvalidScars))
		for _, inv := range res.InvalidScars {
			fmt.Printf("  - %s\n", inv)
		}
	}

	if len(res.StaleScars) > 0 {
		fmt.Printf("\n[!] Stale Active Scars Requiring Reaffirmation (%d):\n", len(res.StaleScars))
		for _, stale := range res.StaleScars {
			fmt.Printf("  - %s (anchor modified since last baseline)\n", stale)
		}
	}

	if len(res.LintFindings) > 0 {
		fmt.Printf("\nFindings & Warnings (%d):\n", len(res.LintFindings))
		for _, f := range res.LintFindings {
			fmt.Printf("  [%s] %s: %s\n", f.Severity, f.Rule, f.Message)
		}
	}

	if res.ExitCode != 0 {
		fmt.Printf("\nCheck exited with code %d\n", res.ExitCode)
		os.Exit(res.ExitCode)
	}

	fmt.Println("\nCheck passed: no blocking findings (exit code 0).")
	return nil
}

func runLint(store *scar.Store, args []string) error {
	scars, err := store.ListScars()
	if err != nil {
		return fmt.Errorf("failed reading scars: %w", err)
	}

	var allScars []*scar.Scar
	var allFindings []scar.LintFinding

	for _, summary := range scars {
		s := summary.Scar
		allScars = append(allScars, s)
		findings := scar.LintScar(store.RepoRoot, s)
		allFindings = append(allFindings, findings...)
	}

	crossFindings := scar.LintCollection(allScars)
	allFindings = append(allFindings, crossFindings...)

	if len(allFindings) == 0 {
		fmt.Printf("Lint clean: %d scars audited with zero findings.\n", len(allScars))
		return nil
	}

	hasError := false
	fmt.Printf("Lint findings for %d scars:\n", len(allScars))
	for _, f := range allFindings {
		if f.Severity == "ERROR" {
			hasError = true
		}
		fmt.Printf("  [%s] %s (%s): %s\n", f.Severity, f.ScarID, f.Rule, f.Message)
	}

	if hasError {
		return fmt.Errorf("lint discovered blocking errors")
	}
	return nil
}

func runFeedback(store *scar.Store, args []string) error {
	fs := flag.NewFlagSet("feedback", flag.ContinueOnError)
	rating := fs.String("rating", "", "Rating: helpful, irrelevant, incorrect, unrated")
	mode := fs.String("mode", "cli", "Retrieval mode: cli, diff, task, agent")
	notes := fs.String("notes", "", "Optional feedback notes")

	if len(args) < 1 {
		return fmt.Errorf("usage: git-scars feedback <id> --rating helpful|irrelevant|incorrect [--notes <text>]")
	}
	id := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if *rating == "" {
		return fmt.Errorf("--rating required (helpful, irrelevant, incorrect, unrated)")
	}

	if err := store.RecordFeedback(id, *rating, *mode, *notes); err != nil {
		return fmt.Errorf("failed recording feedback: %w", err)
	}

	fmt.Printf("Recorded feedback for scar %s (%s) in .gitscars/.local/feedback.jsonl\n", id, *rating)
	return nil
}

func runHook(store *scar.Store, args []string) error {
	action := "status"
	if len(args) > 0 {
		action = args[0]
	}

	switch action {
	case "install":
		if err := scar.InstallPreCommitHook(store.RepoRoot); err != nil {
			return fmt.Errorf("failed installing pre-commit hook: %w", err)
		}
		fmt.Printf("Installed Git pre-commit hook in %s/.git/hooks/pre-commit\n", store.RepoRoot)
		return nil
	case "uninstall":
		if err := scar.UninstallPreCommitHook(store.RepoRoot); err != nil {
			return fmt.Errorf("failed uninstalling pre-commit hook: %w", err)
		}
		fmt.Println("Uninstalled Git pre-commit hook.")
		return nil
	case "status":
		installed := scar.IsHookInstalled(store.RepoRoot)
		if installed {
			fmt.Println("Git pre-commit hook is INSTALLED (.git/hooks/pre-commit).")
		} else {
			fmt.Println("Git pre-commit hook is NOT installed. Run 'git-scars hook install' to activate.")
		}
		return nil
	default:
		return fmt.Errorf("unknown hook action %q; expected install, uninstall, or status", action)
	}
}

func runIndex(store *scar.Store, args []string) error {
	idx, err := store.RebuildIndex()
	if err != nil {
		return fmt.Errorf("failed rebuilding index: %w", err)
	}
	fmt.Printf("Rebuilt local index with %d scars in %s\n", idx.TotalScars, filepath.Join(store.LocalDir, scar.IndexFileName))
	return nil
}

func runDiff(store *scar.Store, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("diff requires a scar ID or filename")
	}

	sc, _, err := store.LoadScar(args[0])
	if err != nil {
		return err
	}

	results, err := store.DiffScarAnchors(sc)
	if err != nil {
		return err
	}

	fmt.Printf("=== Anchor Baseline Diff for %s (%s) ===\n", sc.ID, sc.Title)
	allCurrent := true
	for _, res := range results {
		switch res.Status {
		case "current":
			fmt.Printf("✓ [current] %s\n", res.FilePath)
		case "modified":
			allCurrent = false
			fmt.Printf("✗ [MODIFIED] %s:\n%s\n", res.FilePath, res.Diff)
		case "missing":
			allCurrent = false
			fmt.Printf("! [MISSING] %s\n", res.FilePath)
		case "unrecorded":
			allCurrent = false
			fmt.Printf("? [UNRECORDED] %s\n", res.FilePath)
		}
	}

	if allCurrent {
		fmt.Println("All anchor baselines are current with working tree.")
	} else {
		fmt.Printf("\nTo reaffirm modified baselines after review: git-scars reaffirm %s\n", sc.ID)
	}

	return nil
}

func runStats(store *scar.Store, args []string) error {
	stats, err := store.GetStats()
	if err != nil {
		return err
	}

	for _, a := range args {
		if a == "--json" {
			data, err := json.MarshalIndent(stats, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}
	}

	fmt.Println("=== GitScars Repository Statistics ===")
	fmt.Printf("Total Scars:         %d\n", stats.TotalScars)
	fmt.Println("Lifecycle States:")
	for state, count := range stats.LifecycleCounts {
		fmt.Printf("  - %-14s: %d\n", state, count)
	}
	fmt.Println("Freshness States:")
	for freshness, count := range stats.FreshnessCounts {
		fmt.Printf("  - %-14s: %d\n", freshness, count)
	}
	fmt.Printf("Anchors Monitored:   %d files, %d symbols, %d error signatures\n",
		stats.TotalFiles, stats.TotalSymbols, stats.TotalErrors)
	if stats.FeedbackTotal > 0 {
		fmt.Printf("Feedback Summary:    %d entries (%.1f%% helpful)\n",
			stats.FeedbackTotal, stats.HelpfulRatio)
	} else {
		fmt.Println("Feedback Summary:    No feedback recorded yet (.gitscars/.local/feedback.jsonl)")
	}

	return nil
}

func runDoctor(store *scar.Store, args []string) error {
	report := store.RunDoctor()

	for _, a := range args {
		if a == "--json" {
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			if !report.Healthy {
				return fmt.Errorf("doctor detected %d diagnostic failure(s)", report.Errors)
			}
			return nil
		}
	}

	fmt.Println("=== GitScars System Diagnostics ===")
	for _, c := range report.Checks {
		symbol := "✓"
		switch c.Status {
		case "OK":
			symbol = "✓"
		case "WARN":
			symbol = "!"
		case "FAIL":
			symbol = "✗"
		}
		fmt.Printf("[%s] %-22s %s\n", symbol, c.Name+":", c.Message)
		for _, d := range c.Details {
			fmt.Printf("      - %s\n", d)
		}
	}

	fmt.Println()
	if report.Healthy {
		fmt.Printf("Diagnostics complete: %d errors, %d warnings (Healthy)\n", report.Errors, report.Warnings)
		return nil
	}

	return fmt.Errorf("diagnostics failed: %d errors, %d warnings", report.Errors, report.Warnings)
}

func runInject(store *scar.Store, args []string) error {
	fs := flag.NewFlagSet("inject", flag.ContinueOnError)
	filesStr := fs.String("files", "", "Comma-separated target files")
	task := fs.String("task", "", "Active task or query context")
	symbolsStr := fs.String("symbols", "", "Comma-separated symbols")
	budget := fs.Int("budget", 400, "Maximum token budget (estimated)")
	format := fs.String("format", "xml", "Output format: xml or markdown")
	includeStale := fs.Bool("include-stale", false, "Include drifted scars")

	if err := fs.Parse(args); err != nil {
		return err
	}

	var files []string
	if *filesStr != "" {
		for _, f := range strings.Split(*filesStr, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				files = append(files, f)
			}
		}
	}

	var symbols []string
	if *symbolsStr != "" {
		for _, s := range strings.Split(*symbolsStr, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				symbols = append(symbols, s)
			}
		}
	}

	snippet, err := store.InjectGuardrails(scar.InjectOptions{
		Files:        files,
		Task:         *task,
		Symbols:      symbols,
		BudgetTokens: *budget,
		Format:       *format,
		IncludeStale: *includeStale,
	})
	if err != nil {
		return err
	}

	if snippet != "" {
		fmt.Println(snippet)
	}
	return nil
}

func runSync(store *scar.Store, args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	targetsStr := fs.String("targets", "", "Comma-separated target files (e.g. AGENTS.md,CLAUDE.md)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	var targets []string
	if *targetsStr != "" {
		for _, t := range strings.Split(*targetsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				targets = append(targets, t)
			}
		}
	}

	results, err := store.SyncExistingInstructions(targets)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		fmt.Println("No matching instruction files found to sync (use --targets AGENTS.md to specify).")
		return nil
	}

	fmt.Println("=== GitScars Instruction Sync ===")
	for _, r := range results {
		fmt.Printf("[%s] %s\n", r.Action, r.FilePath)
	}
	return nil
}

func runTriage(store *scar.Store, args []string) error {
	fs := flag.NewFlagSet("triage", flag.ContinueOnError)
	errStr := fs.String("error", "", "Direct error message or panic trace")
	jsonOut := fs.Bool("json", false, "Output matches in machine-readable JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	input := *errStr
	if input == "" {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed reading stdin: %w", err)
			}
			input = string(data)
		}
	}

	if strings.TrimSpace(input) == "" {
		return fmt.Errorf("triage requires an error message via --error or piped via stdin")
	}

	res, err := store.TriageError(input)
	if err != nil {
		return err
	}

	if *jsonOut {
		data, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println("=== GitScars Failure Triage ===")
	if res.MatchedSignature != "" {
		fmt.Printf("Detected Signature: %s\n\n", res.MatchedSignature)
	}

	if len(res.Matches) == 0 {
		fmt.Println("No known failure memories matched this error trace.")
		return nil
	}

	fmt.Printf("Found %d matching failure memor(ies):\n\n", len(res.Matches))
	for i, m := range res.Matches {
		sc := m.Scar
		fmt.Printf("%d. [%s] (Score: %d) %s\n", i+1, sc.ID, m.Score, sc.Title)
		fmt.Printf("   Lesson:    %s\n", sc.Lesson)
		if len(sc.Anchors.Files) > 0 {
			fmt.Printf("   Anchors:   %s\n", strings.Join(sc.Anchors.Files, ", "))
		}
		if len(sc.Verifier.Argv) > 0 {
			fmt.Printf("   Verifier:  %s\n", strings.Join(sc.Verifier.Argv, " "))
		}
		fmt.Println()
	}

	return nil
}
