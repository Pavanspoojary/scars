package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/Pavanspoojary/scars/pkg/scar"
)

// HealCommandFailedExitCode is returned by 'git-scars heal' when the observed
// command failed. The observed command's own exit code is reported in the
// heal output, but it is not reused as the exit code of git-scars, because
// exit code 1 already means an operational error of git-scars itself.
const HealCommandFailedExitCode = 4

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
		return runInit(store, args[1:])
	case "enforce":
		return runEnforce(store, args[1:])
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
	case "contradict":
		return runContradict(store, args[1:])
	case "supersede":
		return runSupersede(store, args[1:])
	case "dedupe":
		return runDedupe(store, args[1:])
	case "harvest":
		return runHarvest(store, args[1:])
	case "bundle":
		return runBundle(store, args[1:])
	case "heal":
		return runHeal(store, args[1:])
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
		fmt.Printf("git-scars version %s\n", scar.Version)
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
  init            Initialize .gitscars/ repository metadata and config (--agent, --ci)
  add             Create a new scar draft (interactive -i or --title/--lesson/--files)
  list            List recorded scars with lifecycle, review, and dynamic freshness
  show <id>       Display full metadata and lesson for a scar
  diff <id>       Show diff between anchor files and recorded baseline
  promote <id>    Promote a scar to active state (enforces review & verifier invariants)
  reaffirm <id>   Reaffirm a scar after anchor code drift with human rationale
  contradict <id> Transition scar to contradicted state (--by <new-id>, --rationale)
  supersede <id>  Supersede scar with successor scar (--by <new-id>)
  retire <id>     Retire a scar that is no longer applicable
  verify <id>     Execute verifier with safety checks and record evidence
  recall          Retrieve relevant scars (--files, --diff, --stdin, --json)
  enforce         Pre-edit blocking guardrail check (--files, --fail-on, --format sarif)
  inject          Generate token-budgeted prompt guardrails for AI agents
  sync            Synchronize active scars into AGENTS.md / CLAUDE.md
  triage          Diagnose test failures and panic traces against scars (--error/stdin)
  dedupe          Detect duplicate and near-duplicate scars (--min, --dry-run, --json)
  harvest         Automatically mine bug fixes from Git history into scar drafts
  bundle          Export and import portable memory bundles (export, import)
  heal            Run test command and automatically triage failures (-- <cmd>)
  check           Read-only verification of freshness and invariants (--format sarif)
  lint            Audit scars for security, wildcards, and schema validity
  doctor          Run repository diagnostics and health checks (--json, --fix)
  stats           Display repository metrics and feedback statistics (--json)
  index           Rebuild .gitscars/.local/index.json fast cache
  hook            Manage Git pre-commit hook (install, uninstall, status)
  mcp             Run read-only stdio Model Context Protocol (MCP) server
  feedback <id>   Record feedback on retrieval utility (--rating, --notes)
  version         Print git-scars version
  help            Show this help text
`, scar.Version)
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

func runInit(store *scar.Store, args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	agent := fs.String("agent", "", "Optional comma-separated agent configuration targets (claude, cursor, zed, antigravity)")
	ci := fs.String("ci", "", "Optional CI/CD workflow generator (github)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := store.Init(); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}
	fmt.Printf("Initialized GitScars in %s/.gitscars\n", store.RepoRoot)

	if *agent != "" {
		for _, a := range strings.Split(*agent, ",") {
			a = strings.ToLower(strings.TrimSpace(a))
			if err := scaffoldAgent(store.RepoRoot, a); err != nil {
				fmt.Printf("warning: failed configuring agent %s: %v\n", a, err)
			} else {
				fmt.Printf("Configured %s integration files successfully.\n", a)
			}
		}
	}

	if *ci != "" {
		switch strings.ToLower(strings.TrimSpace(*ci)) {
		case "github":
			ciPath := filepath.Join(store.RepoRoot, ".github", "workflows", "git-scars.yml")
			if err := os.MkdirAll(filepath.Dir(ciPath), 0755); err != nil {
				return fmt.Errorf("failed creating .github/workflows directory: %w", err)
			}
			if err := os.WriteFile(ciPath, []byte(scar.GitHubWorkflowYAML()), 0644); err != nil {
				return fmt.Errorf("failed writing github workflow: %w", err)
			}
			fmt.Println("Configured GitHub Actions workflow in .github/workflows/git-scars.yml")
		default:
			return fmt.Errorf("unsupported CI target: %s (supported: github)", *ci)
		}
	}
	return nil
}

// splitFileList splits a list of paths separated by commas or newlines. The
// newline form allows a file list to be piped directly from a git command.
func splitFileList(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
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
	depsStr := fs.String("deps", "", "Comma-separated dependency anchors in name@version form")
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

	files := splitFileList(*filesStr)

	var deps []scar.DependencyAnchor
	if *depsStr != "" {
		for _, d := range strings.Split(*depsStr, ",") {
			d = strings.TrimSpace(d)
			if d == "" {
				continue
			}
			name, version, _ := strings.Cut(d, "@")
			deps = append(deps, scar.DependencyAnchor{
				Name:    strings.TrimSpace(name),
				Version: strings.TrimSpace(version),
			})
		}
	}

	draft, path, err := store.CreateDraft(*title, *lesson, files)
	if err != nil {
		return fmt.Errorf("failed creating draft: %w", err)
	}

	if len(deps) > 0 {
		draft.Anchors.Dependencies = deps
		if _, err := store.SaveScar(draft); err != nil {
			return fmt.Errorf("failed saving dependency anchors: %w", err)
		}
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

	// The fail-to-pass procedure overlays the test files of the fix commit onto
	// the base commit. Without them the base would simply run the tests that
	// already existed, which is not a reproduction of the regression.
	var testFiles []string
	if s.FixCommit != "" {
		if info, err := scar.InspectCommit(store.RepoRoot, s.FixCommit); err == nil {
			testFiles = info.TestFiles
		}
	}

	outcome, verifyErr := store.VerifyFailToPass(ctx, s, testFiles, *interactive)
	if verifyErr != nil {
		fmt.Fprintf(os.Stderr, "verification notice: %v\n", verifyErr)
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

	// A verification that did not produce a passable result must be visible to
	// callers and CI, not reported as a successful command.
	if verifyErr != nil {
		return fmt.Errorf("verification did not pass: %w", verifyErr)
	}
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
	if q, err := store.GetScarQuality(s.ID); err == nil && q.Total > 0 {
		fmt.Printf("Feedback Quality: Score %.2f (Helpful: %d, Irrelevant: %d, Incorrect: %d, Total: %d)\n",
			q.Score, q.Helpful, q.Irrelevant, q.Incorrect, q.Total)
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

func runContradict(store *scar.Store, args []string) error {
	fs := flag.NewFlagSet("contradict", flag.ContinueOnError)
	by := fs.String("by", "", "Successor or conflicting scar ID")
	rationale := fs.String("rationale", "", "Rationale explaining why this scar is contradicted")

	if len(args) < 1 {
		return fmt.Errorf("usage: git-scars contradict <id> --by <new-id> [--rationale <reason>]")
	}
	id := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if *by == "" {
		return fmt.Errorf("contradict requires --by <new-id>")
	}

	scar, err := store.ContradictScar(id, *by, *rationale)
	if err != nil {
		return fmt.Errorf("failed contradicting scar %s: %w", id, err)
	}

	fmt.Printf("Successfully transitioned scar %s to state %q (conflicts with %s)\n", scar.ID, scar.Lifecycle.State, *by)
	return nil
}

func runSupersede(store *scar.Store, args []string) error {
	fs := flag.NewFlagSet("supersede", flag.ContinueOnError)
	by := fs.String("by", "", "Successor scar ID that supersedes this scar")

	if len(args) < 1 {
		return fmt.Errorf("usage: git-scars supersede <old-id> --by <new-id>")
	}
	id := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if *by == "" {
		return fmt.Errorf("supersede requires --by <new-id>")
	}

	scar, err := store.SupersedeScar(id, *by)
	if err != nil {
		return fmt.Errorf("failed superseding scar %s: %w", id, err)
	}

	fmt.Printf("Successfully superseded scar %s by successor %s (state: %q)\n", scar.ID, *by, scar.Lifecycle.State)
	return nil
}

func runDedupe(store *scar.Store, args []string) error {
	fs := flag.NewFlagSet("dedupe", flag.ContinueOnError)
	minSim := fs.Float64("min", 0.85, "Minimum Jaccard lexical similarity threshold (0.0 - 1.0)")
	dryRun := fs.Bool("dry-run", true, "Perform dry-run inspection without altering scar records")
	jsonOut := fs.Bool("json", false, "Output duplicate candidate pairs in JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	candidates, err := store.FindDuplicates(*minSim)
	if err != nil {
		return fmt.Errorf("failed analyzing duplicate scars: %w", err)
	}

	if *jsonOut {
		data, err := json.MarshalIndent(candidates, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(candidates) == 0 {
		fmt.Printf("No duplicate or near-duplicate scars found (threshold: %.2f).\n", *minSim)
		return nil
	}

	fmt.Printf("Discovered %d candidate duplicate pair(s) (threshold: %.2f, dry-run: %v):\n\n", len(candidates), *minSim, *dryRun)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SIMILARITY\tSCAR A\tSCAR B\tRECOMMENDED ACTION")
	for _, c := range candidates {
		fmt.Fprintf(w, "%.2f\t%s\t%s\t%s\n", c.Similarity, c.ScarA.ID, c.ScarB.ID, c.Suggestion)
	}
	return w.Flush()
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
	threshold := fs.Int("threshold", 0, "Abstain threshold score (0 uses the configured value)")
	topK := fs.Int("top-k", 0, "Maximum scars to return (0 uses the configured value)")

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

	opts.Files = splitFileList(*filesStr)
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
	format := fs.String("format", "text", "Output format: text, sarif")
	if err := fs.Parse(args); err != nil {
		return err
	}

	res, err := store.RunReadOnlyCheck(*diffRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Operational error: %v\n", err)
		os.Exit(3)
	}

	if *format == "sarif" {
		summaries, _ := store.ListScars()
		sarifData, err := scar.FormatCheckSARIF(summaries, res.InvalidScars)
		if err != nil {
			return err
		}
		fmt.Println(string(sarifData))
		if res.ExitCode != 0 {
			os.Exit(res.ExitCode)
		}
		return nil
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
	scars, failures, err := store.ListScarsDetailed()
	if err != nil {
		return fmt.Errorf("failed reading scars: %w", err)
	}

	var allScars []*scar.Scar
	var allFindings []scar.LintFinding

	for _, f := range failures {
		allFindings = append(allFindings, scar.LintFinding{
			ScarID:   f.ID,
			Severity: "ERROR",
			Rule:     "unloadable_scar",
			Message:  fmt.Sprintf("scar file could not be loaded: %s", f.Error),
		})
	}

	for _, summary := range scars {
		s := summary.Scar
		allScars = append(allScars, s)
		findings := scar.LintScar(store.RepoRoot, s)
		allFindings = append(allFindings, findings...)
	}

	crossFindings := scar.LintCollection(allScars)
	allFindings = append(allFindings, crossFindings...)

	qualities, _ := store.GetAllScarQualities()
	if qualities != nil {
		for _, s := range allScars {
			if q, ok := qualities[s.ID]; ok && q.Incorrect >= 3 {
				allFindings = append(allFindings, scar.LintFinding{
					ScarID:   s.ID,
					Severity: "WARNING",
					Rule:     "noisy_scar",
					Message:  fmt.Sprintf("scar has accumulated %d incorrect feedback ratings; consider updating anchors, superseding, or retiring", q.Incorrect),
				})
			}
		}
	}

	audited := len(scars) + len(failures)
	if len(allFindings) == 0 {
		fmt.Printf("Lint clean: %d scar file(s) audited with zero findings.\n", audited)
		return nil
	}

	hasError := false
	fmt.Printf("Lint findings across %d scar file(s):\n", audited)
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
			fmt.Printf("[current] %s\n", res.FilePath)
		case "modified":
			allCurrent = false
			fmt.Printf("[MODIFIED] %s:\n%s\n", res.FilePath, res.Diff)
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
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "Emit machine-readable JSON metrics")
	if err := fs.Parse(args); err != nil {
		return err
	}

	stats, err := store.GetStats()
	if err != nil {
		return err
	}

	if *asJSON {
		data, err := json.MarshalIndent(stats, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
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
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output diagnostic report in JSON")
	fix := fs.Bool("fix", false, "Automatically repair recoverable configuration, cache, and hook issues")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *fix {
		actions, err := store.FixDoctor()
		if err != nil {
			return fmt.Errorf("doctor --fix error: %w", err)
		}
		if len(actions) == 0 {
			fmt.Println("No automated repairs required.")
		} else {
			fmt.Println("Applied automated repairs:")
			for _, act := range actions {
				fmt.Printf("  [ok] %s\n", act)
			}
			fmt.Println()
		}
	}

	report := store.RunDoctor()

	if *jsonOut {
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

	fmt.Println("=== GitScars System Diagnostics ===")
	for _, c := range report.Checks {
		symbol := "ok"
		switch c.Status {
		case "OK":
			symbol = "ok"
		case "WARN":
			symbol = "warn"
		case "FAIL":
			symbol = "fail"
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

	files := splitFileList(*filesStr)

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

func runEnforce(store *scar.Store, args []string) error {
	fs := flag.NewFlagSet("enforce", flag.ContinueOnError)
	filesStr := fs.String("files", "", "Comma-separated target file paths")
	symbolsStr := fs.String("symbols", "", "Comma-separated target symbols")
	failOn := fs.String("fail-on", "high", "Failure threshold severity: critical, high, medium, low")
	format := fs.String("format", "text", "Output format: text, json, sarif")
	staged := fs.Bool("staged", false, "Automatically inspect staged files from git diff --cached")
	asJSON := fs.Bool("json", false, "Output result in JSON format (shorthand for --format json)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	var files []string
	if *filesStr != "" {
		files = splitFileList(*filesStr)
	}

	if *staged && scar.GitAvailable(store.RepoRoot) {
		cmd := exec.Command("git", "diff", "--cached", "--name-only")
		cmd.Dir = store.RepoRoot
		out, err := cmd.Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if line = strings.TrimSpace(line); line != "" {
					files = append(files, line)
				}
			}
		}
	}

	var symbols []string
	if *symbolsStr != "" {
		for _, s := range strings.Split(*symbolsStr, ",") {
			if s = strings.TrimSpace(s); s != "" {
				symbols = append(symbols, s)
			}
		}
	}

	res, err := store.Enforce(scar.EnforceOptions{
		Files:   files,
		Symbols: symbols,
		FailOn:  *failOn,
	})
	if err != nil {
		return err
	}

	if *asJSON || *format == "json" {
		data, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		if res.Blocked {
			os.Exit(2)
		}
		return nil
	}

	if *format == "sarif" {
		sarifData, err := scar.FormatEnforceSARIF(res)
		if err != nil {
			return err
		}
		fmt.Println(string(sarifData))
		if res.Blocked {
			os.Exit(2)
		}
		return nil
	}

	if !res.Blocked {
		fmt.Println("clean: no blocking regression scars detected")
		return nil
	}

	fmt.Fprintf(os.Stderr, "=== [GUARDRAIL BLOCKED] Pre-Edit Safety Check Failed ===\n")
	fmt.Fprintf(os.Stderr, "%s\n\n", res.Reason)
	for i, sc := range res.BlockingScars {
		s := sc.Scar
		sev := s.Severity
		if sev == "" {
			sev = scar.SeverityMedium
		}
		fmt.Fprintf(os.Stderr, "[%d] %s (%s severity | Score: %d | Confidence: %.2f)\n", i+1, s.ID, sev, sc.Score, sc.Confidence)
		fmt.Fprintf(os.Stderr, "    Title:  %s\n", s.Title)
		fmt.Fprintf(os.Stderr, "    Lesson: %s\n", s.Lesson)
		if s.FixHint != "" {
			fmt.Fprintf(os.Stderr, "    Hint:   %s\n", s.FixHint)
		}
		if s.BadPattern != "" {
			fmt.Fprintf(os.Stderr, "    Don't:  %s\n", s.BadPattern)
		}
		if s.GoodPattern != "" {
			fmt.Fprintf(os.Stderr, "    Do:     %s\n", s.GoodPattern)
		}
		if len(s.Verifier.Argv) > 0 {
			fmt.Fprintf(os.Stderr, "    Verify: %s\n", strings.Join(s.Verifier.Argv, " "))
		}
		fmt.Fprintln(os.Stderr)
	}
	os.Exit(2)
	return nil
}

func runHarvest(store *scar.Store, args []string) error {
	fs := flag.NewFlagSet("harvest", flag.ContinueOnError)
	maxCommits := fs.Int("max", 50, "Maximum recent git commits to scan")
	since := fs.String("since", "", "Optional Git revision to start scanning from (e.g. HEAD~20, main)")
	dryRun := fs.Bool("dry-run", false, "Preview candidate fix commits without creating draft scars")
	allowUntested := fs.Bool("allow-untested", false, "Include candidate fix commits that lack co-located tests")
	maxFiles := fs.Int("max-files", 5, "Maximum modified files allowed in candidate commit (skips large refactors)")
	minConfidence := fs.Int("min-confidence", 7, "Minimum quality confidence threshold 1-10 (default: 7)")
	asJSON := fs.Bool("json", false, "Output results in JSON format")

	if err := fs.Parse(args); err != nil {
		return err
	}

	res, err := store.HarvestCommits(scar.HarvestOptions{
		MaxCommits:    *maxCommits,
		SinceRef:      *since,
		DryRun:        *dryRun,
		AllowUntested: *allowUntested,
		MaxFiles:      *maxFiles,
		MinConfidence: *minConfidence,
	})
	if err != nil {
		return fmt.Errorf("harvest failed: %w", err)
	}

	if *asJSON {
		data, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println("=== GitScars Regression Harvest ===")
	fmt.Printf("Scanned Commits: %d | Qualified Fixes: %d | Drafts Created: %d\n\n",
		res.ScannedCommits, res.FoundFixes, res.DraftsCreated)

	if len(res.Candidates) == 0 {
		fmt.Println("No qualified high-confidence bug-fix commits found in scanned history.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "COMMIT\tCONF\tFILES\tVERIFIER\tSUBJECT")
	for _, c := range res.Candidates {
		shortSha := c.CommitSHA
		if len(shortSha) > 8 {
			shortSha = shortSha[:8]
		}
		files := strings.Join(c.Files, ",")
		if len(files) > 22 {
			files = files[:19] + "..."
		}
		verifier := "-"
		if len(c.Verifier) > 0 {
			verifier = strings.Join(c.Verifier, " ")
			if len(verifier) > 25 {
				verifier = verifier[:22] + "..."
			}
		}
		subj := c.Subject
		if len(subj) > 35 {
			subj = subj[:32] + "..."
		}
		fmt.Fprintf(w, "%s\t%d/10\t%s\t%s\t%s\n", shortSha, c.Confidence, files, verifier, subj)
	}
	_ = w.Flush()

	if *dryRun {
		fmt.Println("\n(Dry run: no drafts were written. Run without --dry-run to create drafts)")
	} else if res.DraftsCreated > 0 {
		fmt.Printf("\nCreated %d reviewable draft scar(s) in .gitscars/scars/. Review and promote with:\n", res.DraftsCreated)
		for _, c := range res.Candidates {
			if c.DraftPath != "" {
				fmt.Printf("  git-scars verify %s && git-scars promote %s\n", c.SuggestedID, c.SuggestedID)
			}
		}
	}
	return nil
}

func runBundle(store *scar.Store, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: git-scars bundle export [options] | git-scars bundle import <file> [options]")
	}

	switch args[0] {
	case "export":
		fs := flag.NewFlagSet("bundle export", flag.ContinueOnError)
		out := fs.String("out", "", "Output bundle file path (defaults to stdout if omitted)")
		all := fs.Bool("all", false, "Include drafts and unreviewed scars in export")
		sev := fs.String("severity", "", "Filter by severity (critical, high, medium, low)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		data, err := store.ExportBundle(scar.BundleExportOptions{
			FilterSeverity: *sev,
			IncludeDrafts:  *all,
		})
		if err != nil {
			return fmt.Errorf("bundle export failed: %w", err)
		}

		if *out != "" {
			if err := os.WriteFile(*out, data, 0644); err != nil {
				return fmt.Errorf("failed writing bundle file: %w", err)
			}
			fmt.Printf("Exported memory bundle to %s (%d bytes)\n", *out, len(data))
		} else {
			fmt.Println(string(data))
		}
		return nil

	case "import":
		if len(args) < 2 {
			return fmt.Errorf("usage: git-scars bundle import <bundle.json> [--prefix <org->] [--dry-run]")
		}
		filePath := args[1]
		fs := flag.NewFlagSet("bundle import", flag.ContinueOnError)
		prefix := fs.String("prefix", "", "Optional ID prefix for imported scars (e.g. org-, team-)")
		dryRun := fs.Bool("dry-run", false, "Preview import without creating draft files")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed reading bundle file %s: %w", filePath, err)
		}

		res, err := store.ImportBundle(data, scar.BundleImportOptions{
			IDPrefix: *prefix,
			DryRun:   *dryRun,
		})
		if err != nil {
			return fmt.Errorf("bundle import failed: %w", err)
		}

		fmt.Println("=== GitScars Bundle Import ===")
		fmt.Printf("Total in Bundle: %d | Imported Drafts: %d | Skipped: %d\n",
			res.TotalInBundle, res.Imported, res.Skipped)
		if len(res.ImportedIDs) > 0 {
			fmt.Printf("Imported Scars: %s\n", strings.Join(res.ImportedIDs, ", "))
			fmt.Println("All imported scars are drafted as unreviewed. Verify and promote to activate.")
		}
		return nil

	default:
		return fmt.Errorf("unknown bundle action %q; expected export or import", args[0])
	}
}

func runHeal(store *scar.Store, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: git-scars heal [--json] -- <test-command...>")
	}

	fs := flag.NewFlagSet("heal", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "Output heal report in JSON format")

	cmdArgs := args
	for i, arg := range args {
		if arg == "--" {
			if err := fs.Parse(args[:i]); err != nil {
				return err
			}
			cmdArgs = args[i+1:]
			break
		}
	}
	if len(cmdArgs) == len(args) {
		if err := fs.Parse(args); err != nil {
			return err
		}
		cmdArgs = fs.Args()
	}

	if len(cmdArgs) == 0 {
		return fmt.Errorf("no test command specified to heal; example: git-scars heal -- go test ./...")
	}

	ctx := context.Background()
	report, err := store.RunHealCommand(ctx, cmdArgs)
	if err != nil {
		return fmt.Errorf("heal execution failed: %w", err)
	}

	if *asJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		if !report.Success {
			os.Exit(HealCommandFailedExitCode)
		}
		return nil
	}

	if report.Success {
		fmt.Printf("Command %q succeeded in %dms (exit code 0)\n",
			strings.Join(report.Command, " "), report.DurationMs)
		return nil
	}

	fmt.Fprintf(os.Stderr, "Command %q failed with exit code %d (%dms)\n",
		strings.Join(report.Command, " "), report.ExitCode, report.DurationMs)

	if report.Language != "" || report.FailureKind != "" {
		fmt.Fprintf(os.Stderr, "  Failure Signature: [%s] %s\n", report.Language, report.FailureKind)
	}
	if report.ExtractedError != "" {
		fmt.Fprintf(os.Stderr, "  Extracted Error:   %s\n", report.ExtractedError)
	}

	if len(report.MatchedScars) > 0 {
		fmt.Fprintf(os.Stderr, "\n=== Matched Regression Scars (%d) ===\n", len(report.MatchedScars))
		for i, m := range report.MatchedScars {
			sc := m.Scar
			fmt.Fprintf(os.Stderr, "[%d] %s (Score: %d | Severity: %s)\n", i+1, sc.ID, m.Score, sc.Severity)
			fmt.Fprintf(os.Stderr, "    Title:  %s\n", sc.Title)
			fmt.Fprintf(os.Stderr, "    Lesson: %s\n", sc.Lesson)
			if sc.FixHint != "" {
				fmt.Fprintf(os.Stderr, "    Fix:    %s\n", sc.FixHint)
			}
			if sc.BadPattern != "" {
				fmt.Fprintf(os.Stderr, "    Don't:  %s\n", sc.BadPattern)
			}
			if sc.GoodPattern != "" {
				fmt.Fprintf(os.Stderr, "    Do:     %s\n", sc.GoodPattern)
			}
			if len(sc.Verifier.Argv) > 0 {
				fmt.Fprintf(os.Stderr, "    Verify: %s\n", strings.Join(sc.Verifier.Argv, " "))
			}
			fmt.Fprintln(os.Stderr)
		}
	}

	if report.SuggestedCommand != "" {
		fmt.Fprintf(os.Stderr, "Suggested Verification: %s\n", report.SuggestedCommand)
	}

	os.Exit(HealCommandFailedExitCode)
	return nil
}

func scaffoldAgent(repoRoot, agent string) error {
	switch agent {
	case "claude":
		dir := filepath.Join(repoRoot, ".claude")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		mcpPath := filepath.Join(dir, "mcp.json")
		content := `{
  "mcpServers": {
    "git-scars": {
      "command": "git-scars",
      "args": ["mcp"]
    }
  }
}
`
		return os.WriteFile(mcpPath, []byte(content), 0644)
	case "cursor":
		dir := filepath.Join(repoRoot, ".cursor", "rules")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		rulePath := filepath.Join(dir, "git-scars.mdc")
		content := `---
description: GitScars regression guardrails
globs: *
alwaysApply: true
---
Before editing code files, execute:
git-scars enforce --files "<file1,file2>" --fail-on high

If tests or builds fail, execute:
git-scars triage --error "<error_text>"
`
		return os.WriteFile(rulePath, []byte(content), 0644)
	case "zed":
		dir := filepath.Join(repoRoot, ".zed")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		path := filepath.Join(dir, "settings.json")
		content := `{
  "context_servers": [
    {
      "id": "git-scars",
      "command": {
        "path": "git-scars",
        "args": ["mcp"]
      }
    }
  ]
}
`
		return os.WriteFile(path, []byte(content), 0644)
	case "antigravity":
		dir := filepath.Join(repoRoot, ".gemini", "config", "rules")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		path := filepath.Join(dir, "git-scars.md")
		content := `# GitScars Regression Guardrail Policy
Before modifying repository code, query active guardrails:
` + "```bash\n" + `git-scars enforce --files "<target_files>" --fail-on high` + "\n```\n"
		return os.WriteFile(path, []byte(content), 0644)
	default:
		return fmt.Errorf("unsupported agent target: %s (supported: claude, cursor, zed, antigravity)", agent)
	}
}
