// Command brief prints a short orientation for a new working session (git
// state, the current phase and backlog, and warnings when the handoff files
// disagree with the repository) and validates the project plan. The Claude
// Code SessionStart hook runs it; anyone else runs it from the repository root:
//
//	go run scripts/brief/main.go              print the brief (always exits 0)
//	go run scripts/brief/main.go -check       validate the plan and, on an item branch, the item loop; exit 1 on errors
//	go run scripts/brief/main.go -plan        every item by phase, with status and evidence
//	go run scripts/brief/main.go -item P1-06  one item in full
//
// On an item branch the item loop of docs/briefs/README.md is checked: the
// approved item brief comes before tests, the approved tests before code,
// tests change only in approved test commits, and every changed path is
// inside the Files of the last approved brief.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	featuresFile = "docs/progress/features.json"
	nowFile      = "docs/progress/NOW.md"
	logFile      = "docs/progress/LOG.md"
	briefsDir    = "docs/briefs"
	mainBranch   = "main"

	// An owner approval is a commit whose subject ends like this (AGENTS.md
	// › Conventions).
	approvedSuffix = `\(approved: [^)]+\)$`

	maxRecentCommits = 6
	maxDirtyPaths    = 8
	maxReadyItems    = 3
)

var (
	phaseStatuses = map[string]bool{"todo": true, "in_progress": true, "done": true}
	itemStatuses  = map[string]bool{"todo": true, "in_progress": true, "blocked": true, "done": true}
	commitRe      = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	itemBranchRe  = regexp.MustCompile(`^item/(P\d+-\d+)`)
	headingRe     = regexp.MustCompile(`^## (.+?)\s*$`)
	executorRe    = regexp.MustCompile(`(?m)^Executor: (strong|standard)\b`)
	openRe        = regexp.MustCompile(`\|\s*OPEN\s*\|`)
	backtickRe    = regexp.MustCompile("`([^`]+)`")

	// The headings every approved item brief has (docs/briefs/README.md ›
	// Template); critical items add criticalSections.
	briefSections    = []string{"Goal", "Spec", "Decisions", "Files", "Contracts", "Steps", "Test map", "Out of scope", "Stop rules", "Progress", "Surprises", "Outcome"}
	criticalSections = []string{"Locks", "Race table", "Mutations"}
)

type backlog struct {
	Phases []phase `json:"phases"`
	Items  []item  `json:"items"`
}

type phase struct {
	Phase    int    `json:"phase"`
	Name     string `json:"name"`
	Scope    string `json:"scope"`
	ExitGate string `json:"exit_gate"`
	Status   string `json:"status"`
}

type item struct {
	ID        string    `json:"id"`
	Phase     int       `json:"phase"`
	Title     string    `json:"title"`
	Critical  bool      `json:"critical"`
	Refs      []string  `json:"refs"`
	DependsOn []string  `json:"depends_on"`
	DoneWhen  string    `json:"done_when"`
	Verify    []string  `json:"verify"`
	Status    string    `json:"status"`
	Owner     string    `json:"owner"`
	Notes     string    `json:"notes"`
	Evidence  *evidence `json:"evidence"`
}

// evidence proves a done item passed: the verify commands as they ran on
// Commit, the mutation checks of critical items, and the owner's approval.
type evidence struct {
	Date       string   `json:"date"`
	Commit     string   `json:"commit"`
	Results    []result `json:"results"`
	Mutations  []string `json:"mutations"`
	Review     string   `json:"review"`
	Report     string   `json:"report"`
	ApprovedBy string   `json:"approved_by"`
}

type result struct {
	Command string `json:"command"`
	Outcome string `json:"outcome"`
	Output  string `json:"output"`
}

func main() {
	check := flag.Bool("check", false, "validate "+featuresFile+" and exit 1 on errors")
	plan := flag.Bool("plan", false, "print every item by phase, with status and evidence")
	itemID := flag.String("item", "", "print one item in full")
	flag.Parse()

	root := findRoot()
	b, problems := loadBacklog(filepath.Join(root, featuresFile))
	if b != nil {
		problems = append(problems, checkEvidenceRefs(root, b)...)
	}

	switch {
	case *check:
		var work *itemWork
		if b != nil {
			work = inspectItemWork(root, b)
		}
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "plan: "+p)
		}
		if work != nil {
			for _, p := range work.problems {
				fmt.Fprintln(os.Stderr, "item: "+p)
			}
		}
		if len(problems) > 0 || (work != nil && len(work.problems) > 0) {
			os.Exit(1)
		}
		if work != nil {
			fmt.Printf("plan OK, item %s OK\n", work.id)
		} else {
			fmt.Println("plan OK")
		}
	case *plan && b != nil:
		printPlan(b)
	case *itemID != "" && b != nil:
		if !printItem(b, *itemID) {
			fmt.Fprintf(os.Stderr, "no item %s\n", *itemID)
			os.Exit(1)
		}
	default:
		printBrief(root, b, problems)
	}
}

func printBrief(root string, b *backlog, problems []string) {
	fmt.Printf("== Bastion session brief, %s ==\n", time.Now().Format("2006-01-02 15:04"))
	branch, warnings := printGit(root)
	if b != nil {
		printBacklog(root, b)
		warnings = append(warnings, checkBranchClaim(branch, b)...)
		if w := inspectItemWork(root, b); w != nil {
			fmt.Println(w.stage())
			for _, p := range w.problems {
				warnings = append(warnings, "item: "+p)
			}
		}
	}
	for _, p := range problems {
		warnings = append(warnings, "plan: "+p)
	}
	for _, w := range warnings {
		fmt.Println("WARN " + w)
	}
	fmt.Println("Next: read " + nowFile + ", then follow AGENTS.md > Session protocol > Start.")
}

// findRoot prefers the directory Claude Code was started in, then the git
// top level, then the nearest parent that holds AGENTS.md.
func findRoot() string {
	if d := os.Getenv("CLAUDE_PROJECT_DIR"); d != "" {
		return d
	}
	if d, err := gitOut(".", "rev-parse", "--show-toplevel"); err == nil && d != "" {
		return d
	}
	cwd, _ := os.Getwd()
	for d := cwd; ; {
		if _, err := os.Stat(filepath.Join(d, "AGENTS.md")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return cwd
		}
		d = parent
	}
}

func gitOut(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimRight(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n"), err
}

func hasCommits(root string) bool {
	_, err := gitOut(root, "rev-parse", "--verify", "--quiet", "HEAD")
	return err == nil
}

func printGit(root string) (string, []string) {
	if _, err := gitOut(root, "rev-parse", "--git-dir"); err != nil {
		return "", []string{"not a git repository: history and staleness checks are off; run `git init`"}
	}
	var warns []string
	branch, _ := gitOut(root, "branch", "--show-current")

	if !hasCommits(root) {
		fmt.Printf("Git: branch %s, no commits yet\n", branch)
	} else {
		count, _ := gitOut(root, "rev-list", "--count", "HEAD")
		fmt.Printf("Git: branch %s, %s commit(s). Recent:\n", branch, count)
		recent, _ := gitOut(root, "log", fmt.Sprintf("-%d", maxRecentCommits), "--date=short", "--format=  %h %ad %s")
		fmt.Println(recent)

		if m := itemBranchRe.FindStringSubmatch(branch); m != nil {
			if ahead, err := gitOut(root, "rev-list", "--count", mainBranch+"..HEAD"); err == nil {
				fmt.Printf("Item branch for %s: %s commit(s) ahead of %s\n", m[1], ahead, mainBranch)
			}
		}
		if unmerged := unmergedItemBranches(root, branch); len(unmerged) > 0 {
			fmt.Println("Unmerged item branches: " + strings.Join(unmerged, ", "))
			if branch == mainBranch {
				warns = append(warns, "work is in flight on "+strings.Join(unmerged, ", ")+": continue it there (`git switch <branch>`) before starting anything new")
			}
		}

		// On main every merge brings an updated NOW.md, so commits after it mean
		// a missed handoff. On a work branch they are the checkpoints since the
		// last handoff: the place to resume from.
		nowRev, _ := gitOut(root, "log", "-1", "--format=%h", "--", nowFile)
		if nowRev == "" {
			warns = append(warns, nowFile+" has never been committed")
		} else if n, _ := gitOut(root, "rev-list", "--count", "--no-merges", nowRev+"..HEAD"); n != "0" {
			if strings.HasPrefix(branch, "item/") || strings.HasPrefix(branch, "fix/") {
				fmt.Printf("Since the last NOW.md update: %s checkpoint commit(s); resume from `git log %s..HEAD`\n", n, nowRev)
			} else {
				warns = append(warns, fmt.Sprintf("%s was last committed %s commit(s) before HEAD: it may be stale; read `git log %s..HEAD`", nowFile, n, nowRev))
			}
		}
	}

	status, _ := gitOut(root, "status", "--porcelain")
	if status == "" {
		fmt.Println("Working tree: clean")
		return branch, warns
	}
	lines := strings.Split(status, "\n")
	fmt.Printf("Working tree: %d uncommitted path(s)\n", len(lines))
	for i, l := range lines {
		if i == maxDirtyPaths {
			fmt.Printf("  ... and %d more\n", len(lines)-maxDirtyPaths)
			break
		}
		fmt.Println("  " + l)
	}
	warns = append(warns, "uncommitted changes: the last session may have ended without a handoff; understand them (`git diff`) before new work")
	return branch, warns
}

// unmergedItemBranches lists item branches, other than the current one, whose
// work has not reached main yet: work in flight that a new session must see.
func unmergedItemBranches(root, current string) []string {
	if _, err := gitOut(root, "rev-parse", "--verify", "--quiet", mainBranch); err != nil {
		return nil
	}
	out, err := gitOut(root, "branch", "--no-merged", mainBranch, "--format=%(refname:short)")
	if err != nil || out == "" {
		return nil
	}
	var names []string
	for _, n := range strings.Split(out, "\n") {
		if strings.HasPrefix(n, "item/") && n != current {
			names = append(names, n)
		}
	}
	return names
}

// checkBranchClaim warns when the checked-out item branch does not match the
// plan: working on item/<ID> means <ID> has been claimed (in progress).
func checkBranchClaim(branch string, b *backlog) []string {
	m := itemBranchRe.FindStringSubmatch(branch)
	if m == nil {
		return nil
	}
	for _, it := range b.Items {
		if it.ID == m[1] {
			if it.Status == "todo" {
				return []string{fmt.Sprintf("on %s but %s is still todo: set it in_progress with an owner", branch, it.ID)}
			}
			return nil
		}
	}
	return []string{fmt.Sprintf("branch %s names no item of the plan", branch)}
}

// itemWork is what the checked-out item branch shows about the item loop:
// its approval commits and every rule of docs/briefs/README.md › Machine
// checks it breaks.
type itemWork struct {
	id          string
	briefCommit string // last `docs(<ID>): brief … (approved: …)` commit
	testsCommit string // first `test(<ID>): … (approved: …)` commit
	problems    []string
}

type commitInfo struct {
	hash, subject string
	files         []string
}

func (w *itemWork) addf(format string, args ...any) {
	w.problems = append(w.problems, fmt.Sprintf(format, args...))
}

// inspectItemWork checks the item branch that is checked out, or returns nil
// when none is.
func inspectItemWork(root string, b *backlog) *itemWork {
	branch, _ := gitOut(root, "branch", "--show-current")
	m := itemBranchRe.FindStringSubmatch(branch)
	if m == nil || !hasCommits(root) {
		return nil
	}
	var it *item
	for i := range b.Items {
		if b.Items[i].ID == m[1] {
			it = &b.Items[i]
		}
	}
	if it == nil {
		return nil // checkBranchClaim reports the unknown item
	}
	base, err := gitOut(root, "merge-base", mainBranch, "HEAD")
	if err != nil {
		return nil
	}
	w := &itemWork{id: it.ID}
	commits, err := itemCommits(root, base)
	if err != nil {
		w.addf("cannot read the branch history: %v", err)
		return w
	}
	w.checkOrder(commits)
	allowed := w.alwaysAllowed()
	if w.briefCommit != "" {
		files, problems := approvedBrief(root, w.briefCommit, *it)
		w.problems = append(w.problems, problems...)
		allowed = append(allowed, files...)
	}
	dirty := uncommittedPaths(root)
	w.checkScope(root, base, allowed, dirty)
	if t := testPaths(dirty); w.testsCommit != "" && len(t) > 0 {
		w.addf("uncommitted test changes after the approved tests: %s; commit them as `test(%s): … (approved: <owner>)` once the owner agrees, or revert them", strings.Join(t, ", "), w.id)
	}
	return w
}

func (w *itemWork) stage() string {
	switch {
	case w.briefCommit == "":
		return fmt.Sprintf("Item %s: brief not approved yet (G1)", w.id)
	case w.testsCommit == "":
		return fmt.Sprintf("Item %s: brief approved @%.7s; tests not approved yet (G2)", w.id, w.briefCommit)
	default:
		return fmt.Sprintf("Item %s: brief approved @%.7s, tests approved @%.7s; implementing (see the brief's Progress)", w.id, w.briefCommit, w.testsCommit)
	}
}

// alwaysAllowed are the paths every stage may change: the item's own brief
// and the progress files.
func (w *itemWork) alwaysAllowed() []string {
	return []string{featuresFile, nowFile, logFile,
		briefsDir + "/active/" + w.id + "-*.md", briefsDir + "/completed/" + w.id + "-*.md"}
}

// checkOrder walks the branch oldest first: nothing but the brief and the
// progress files before the approved brief (G1), no code before the approved
// tests (G2), and after them no test change outside an approved test commit.
func (w *itemWork) checkOrder(commits []commitInfo) {
	q := regexp.QuoteMeta(w.id)
	briefRe := regexp.MustCompile(`^docs\(` + q + `\): brief\b.*` + approvedSuffix)
	testRe := regexp.MustCompile(`^test\(` + q + `\): .*` + approvedSuffix)
	codeRe := regexp.MustCompile(`^(feat|fix|refactor)\(` + q + `\):`)
	always := w.alwaysAllowed()
	for _, c := range commits {
		at := fmt.Sprintf("%.7s %q", c.hash, c.subject)
		isBrief, isTest := briefRe.MatchString(c.subject), testRe.MatchString(c.subject)
		if w.briefCommit == "" && !isBrief {
			if out := outside(c.files, always); len(out) > 0 {
				w.addf("%s changes %s before the approved brief (G1)", at, strings.Join(out, ", "))
			}
		}
		if codeRe.MatchString(c.subject) && w.testsCommit == "" {
			w.addf("%s is code before the approved tests (G2)", at)
		}
		if t := testPaths(c.files); w.testsCommit != "" && !isTest && len(t) > 0 {
			w.addf("%s changes tests outside a `test(%s): … (approved: …)` commit: %s", at, w.id, strings.Join(t, ", "))
		}
		if isBrief {
			w.briefCommit = c.hash
		}
		if isTest && w.testsCommit == "" {
			w.testsCommit = c.hash
		}
	}
}

// checkScope reports every changed path the allowed patterns do not cover.
// Before the brief is approved, checkOrder has already judged the commits,
// so only uncommitted paths are left to check.
func (w *itemWork) checkScope(root, base string, allowed, dirty []string) {
	paths, why := dirty, "changed before the brief is approved (G1)"
	if w.briefCommit != "" {
		committed, _ := gitOut(root, "diff", "--name-only", "--no-renames", base, "HEAD")
		paths, why = append(splitLines(committed), dirty...), "outside the Files of the approved brief"
	}
	seen := map[string]bool{}
	for _, p := range outside(paths, allowed) {
		if !seen[p] {
			seen[p] = true
			w.addf("%s: %s", p, why)
		}
	}
}

func itemCommits(root, base string) ([]commitInfo, error) {
	out, err := gitOut(root, "log", "--reverse", "--no-merges", "--format=%H%x09%s", base+"..HEAD")
	if err != nil {
		return nil, err
	}
	var commits []commitInfo
	for _, line := range splitLines(out) {
		hash, subject, _ := strings.Cut(line, "\t")
		files, err := gitOut(root, "diff-tree", "--no-commit-id", "--name-only", "-r", "--no-renames", hash)
		if err != nil {
			return nil, err
		}
		commits = append(commits, commitInfo{hash: hash, subject: subject, files: splitLines(files)})
	}
	return commits, nil
}

// approvedBrief reads the brief as it was approved in commit and returns its
// Files patterns, with a problem for each Definition of Ready rule the
// machine can check.
func approvedBrief(root, commit string, it item) ([]string, []string) {
	files, _ := gitOut(root, "diff-tree", "--no-commit-id", "--name-only", "-r", "--no-renames", commit)
	pathRe := regexp.MustCompile(`^` + regexp.QuoteMeta(briefsDir) + `/(active|completed)/` + regexp.QuoteMeta(it.ID) + `-[^/]+\.md$`)
	var path string
	for _, f := range splitLines(files) {
		if pathRe.MatchString(f) {
			path = f
		}
	}
	if path == "" {
		return nil, []string{fmt.Sprintf("approved brief commit %.7s does not contain %s/active/%s-<slug>.md", commit, briefsDir, it.ID)}
	}
	text, err := gitOut(root, "show", commit+":"+path)
	if err != nil {
		return nil, []string{fmt.Sprintf("cannot read %s at %.7s: %v", path, commit, err)}
	}
	return readyBrief(text, it, fmt.Sprintf("%s @%.7s", path, commit))
}

func readyBrief(text string, it item, where string) ([]string, []string) {
	var errs []string
	add := func(format string, args ...any) { errs = append(errs, where+": "+fmt.Sprintf(format, args...)) }

	secs := sections(text)
	required := briefSections
	if it.Critical {
		required = append(append([]string{}, briefSections...), criticalSections...)
	}
	for _, s := range required {
		if _, ok := secs[s]; !ok {
			add("missing section %q", "## "+s)
		}
	}
	switch m := executorRe.FindStringSubmatch(text); {
	case m == nil:
		add("no `Executor: strong|standard` line")
	case it.Critical && m[1] == "standard":
		add("a critical item needs `Executor: strong`")
	}
	if openRe.MatchString(secs["Decisions"]) {
		add("a decision is still OPEN")
	}
	var patterns []string
	for _, line := range strings.Split(secs["Files"], "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "- ") {
			continue
		}
		for _, m := range backtickRe.FindAllStringSubmatch(line, -1) {
			if _, err := globRe(m[1]); err != nil {
				add("bad pattern %q in Files", m[1])
				continue
			}
			patterns = append(patterns, m[1])
		}
	}
	if len(patterns) == 0 {
		add("Files lists no path")
	}
	return patterns, errs
}

// sections maps each "## " heading of a Markdown text to its body, ignoring
// headings inside code fences.
func sections(text string) map[string]string {
	secs := map[string]string{}
	var cur string
	var body strings.Builder
	inFence := false
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
		}
		if m := headingRe.FindStringSubmatch(line); m != nil && !inFence {
			if cur != "" {
				secs[cur] = body.String()
			}
			cur = m[1]
			body.Reset()
			continue
		}
		body.WriteString(line + "\n")
	}
	if cur != "" {
		secs[cur] = body.String()
	}
	return secs
}

// globRe turns a Files pattern into a regular expression: `*` and `?` stay
// inside one path segment, `**` crosses segments, `{a,b}` is an alternative,
// and a trailing `/` covers the whole directory.
func globRe(pattern string) (*regexp.Regexp, error) {
	if strings.HasSuffix(pattern, "/") {
		pattern += "**"
	}
	var sb strings.Builder
	sb.WriteString("^")
	inBrace := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch {
		case c == '*' && strings.HasPrefix(pattern[i:], "**/"):
			sb.WriteString("(?:.*/)?")
			i += 2
		case c == '*' && strings.HasPrefix(pattern[i:], "**"):
			sb.WriteString(".*")
			i++
		case c == '*':
			sb.WriteString("[^/]*")
		case c == '?':
			sb.WriteString("[^/]")
		case c == '{' && !inBrace:
			inBrace = true
			sb.WriteString("(?:")
		case c == '}' && inBrace:
			inBrace = false
			sb.WriteString(")")
		case c == ',' && inBrace:
			sb.WriteString("|")
		default:
			sb.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	if inBrace {
		return nil, fmt.Errorf("unclosed { in %q", pattern)
	}
	sb.WriteString("$")
	return regexp.Compile(sb.String())
}

// outside returns the paths that no pattern covers.
func outside(paths, patterns []string) []string {
	var res []*regexp.Regexp
	for _, p := range patterns {
		if re, err := globRe(p); err == nil {
			res = append(res, re)
		}
	}
	var out []string
	for _, p := range paths {
		covered := false
		for _, re := range res {
			if re.MatchString(p) {
				covered = true
				break
			}
		}
		if !covered {
			out = append(out, p)
		}
	}
	return out
}

// testPaths returns the paths that are tests: `*_test.go` and anything
// under a `testdata/` directory.
func testPaths(paths []string) []string {
	var tests []string
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") || strings.HasPrefix(p, "testdata/") || strings.Contains(p, "/testdata/") {
			tests = append(tests, p)
		}
	}
	return tests
}

// uncommittedPaths lists every path with uncommitted changes, untracked files
// included.
func uncommittedPaths(root string) []string {
	out, err := exec.Command("git", "-C", root, "status", "--porcelain", "-z", "--untracked-files=all", "--no-renames").Output()
	if err != nil {
		return nil
	}
	var paths []string
	for _, e := range strings.Split(string(out), "\x00") {
		if len(e) > 3 {
			paths = append(paths, e[3:])
		}
	}
	return paths
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func loadBacklog(path string) (*backlog, []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{fmt.Sprintf("cannot read %s: %v", featuresFile, err)}
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var b backlog
	if err := dec.Decode(&b); err != nil {
		return nil, []string{fmt.Sprintf("%s is invalid: %v", featuresFile, err)}
	}
	return &b, validate(&b)
}

func validate(b *backlog) []string {
	var errs []string
	add := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }

	phaseStatus := map[int]string{}
	for _, p := range b.Phases {
		if _, dup := phaseStatus[p.Phase]; dup {
			add("phase %d is listed twice", p.Phase)
		}
		if !phaseStatuses[p.Status] {
			add("phase %d: invalid status %q", p.Phase, p.Status)
		}
		phaseStatus[p.Phase] = p.Status
	}

	byID := map[string]item{}
	for _, it := range b.Items {
		if _, dup := byID[it.ID]; dup {
			add("%s: duplicate id", it.ID)
		}
		byID[it.ID] = it
	}

	inProgress := map[string]int{}
	for _, it := range b.Items {
		if !itemStatuses[it.Status] {
			add("%s: invalid status %q", it.ID, it.Status)
		}
		ps, ok := phaseStatus[it.Phase]
		if !ok {
			add("%s: unknown phase %d", it.ID, it.Phase)
		}
		if it.Title == "" || it.DoneWhen == "" || len(it.Verify) == 0 {
			add("%s: title, done_when and verify are required", it.ID)
		}
		if (it.Status == "in_progress" || it.Status == "blocked") && it.Owner == "" {
			add("%s: %s needs an owner", it.ID, it.Status)
		}
		if it.Status == "blocked" && it.Notes == "" {
			add("%s: blocked needs the reason in notes", it.ID)
		}
		if ps == "done" && it.Status != "done" {
			add("%s: phase %d is done but the item is %s", it.ID, it.Phase, it.Status)
		}
		for _, d := range it.DependsOn {
			dep, ok := byID[d]
			if !ok {
				add("%s: unknown dependency %s", it.ID, d)
			} else if it.Status == "done" && dep.Status != "done" {
				add("%s: done before its dependency %s", it.ID, d)
			}
		}
		if it.Status == "in_progress" {
			inProgress[it.Owner]++
			if inProgress[it.Owner] == 2 {
				add("%s has more than one item in progress; work on one at a time", it.Owner)
			}
		}
		errs = append(errs, validateEvidence(it)...)
	}
	return errs
}

// validateEvidence requires complete evidence on done items and none on the
// others, so a status can never claim more than was proven.
func validateEvidence(it item) []string {
	var errs []string
	add := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(it.ID+": "+format, args...)) }

	e := it.Evidence
	if it.Status != "done" {
		if e != nil {
			add("evidence on an item that is not done; move it to notes or LOG.md")
		}
		return errs
	}
	if e == nil {
		add("done without evidence (docs/REVIEW.md › Evidence)")
		return errs
	}
	if _, err := time.Parse("2006-01-02", e.Date); err != nil {
		add("evidence.date must be YYYY-MM-DD")
	}
	if !commitRe.MatchString(e.Commit) {
		add("evidence.commit must be the hash of the verified commit")
	}
	passed := map[string]bool{}
	for _, r := range e.Results {
		if r.Outcome == "pass" && strings.TrimSpace(r.Output) != "" {
			passed[r.Command] = true
		}
	}
	for _, v := range it.Verify {
		if !passed[v] {
			add("verify command %q has no passing result with its output in evidence.results", v)
		}
	}
	if it.Critical && len(e.Mutations) == 0 {
		add("critical item needs evidence.mutations (REVIEW G4)")
	}
	if e.Review == "" || e.Report == "" || e.ApprovedBy == "" {
		add("evidence.review, evidence.report and evidence.approved_by are required")
	}
	return errs
}

// checkEvidenceRefs checks what evidence points at: the report file exists
// and the verified commit is in the history of HEAD.
func checkEvidenceRefs(root string, b *backlog) []string {
	var errs []string
	commits := hasCommits(root)
	for _, it := range b.Items {
		e := it.Evidence
		if it.Status != "done" || e == nil {
			continue
		}
		if path, _, _ := strings.Cut(e.Report, "#"); path != "" {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
				errs = append(errs, fmt.Sprintf("%s: evidence.report %s does not exist", it.ID, path))
			}
		}
		if commits && commitRe.MatchString(e.Commit) {
			if _, err := gitOut(root, "merge-base", "--is-ancestor", e.Commit, "HEAD"); err != nil {
				errs = append(errs, fmt.Sprintf("%s: evidence.commit %s is not in the history of HEAD", it.ID, e.Commit))
			}
		}
	}
	return errs
}

func printBacklog(root string, b *backlog) {
	done := 0
	for _, it := range b.Items {
		if it.Status == "done" {
			done++
		}
	}
	fmt.Printf("Plan: %d/%d items done across %d phases (full view: -plan; one item: -item <ID>)\n", done, len(b.Items), len(b.Phases))
	var last *item
	for i := range b.Items {
		it := &b.Items[i]
		if it.Status == "done" && it.Evidence != nil && (last == nil || it.Evidence.Date >= last.Evidence.Date) {
			last = it
		}
	}
	if last != nil {
		fmt.Printf("Last done: %s %s (verified %s @%s)\n", last.ID, last.Title, last.Evidence.Date, last.Evidence.Commit)
	}

	var cur *phase
	for i := range b.Phases {
		if b.Phases[i].Status != "done" {
			cur = &b.Phases[i]
			break
		}
	}
	if cur == nil {
		fmt.Println("All phases done")
	} else {
		counts, total := map[string]int{}, 0
		for _, it := range b.Items {
			if it.Phase == cur.Phase {
				counts[it.Status]++
				total++
			}
		}
		fmt.Printf("Phase %d %s (%s): %d/%d items done, %d in progress, %d blocked\n",
			cur.Phase, cur.Name, cur.Status, counts["done"], total, counts["in_progress"], counts["blocked"])
		fmt.Println("  Exit gate: " + cur.ExitGate)
	}

	status := map[string]string{}
	for _, it := range b.Items {
		status[it.ID] = it.Status
	}
	var ready []item
	for _, it := range b.Items {
		switch it.Status {
		case "in_progress":
			fmt.Printf("In progress: %s %s (owner %s)\n", it.ID, it.Title, it.Owner)
		case "blocked":
			fmt.Printf("Blocked: %s %s (owner %s): %s\n", it.ID, it.Title, it.Owner, it.Notes)
		case "todo":
			if len(ready) < maxReadyItems && depsDone(it, status) {
				ready = append(ready, it)
			}
		}
	}
	if len(ready) > 0 {
		fmt.Println("Ready to start, in priority order:")
		for _, it := range ready {
			fmt.Printf("  %s %s\n", it.ID, it.Title)
		}
	}

	briefs, _ := filepath.Glob(filepath.Join(root, briefsDir, "active", "*.md"))
	for _, p := range briefs {
		fmt.Println("Active item brief: " + briefsDir + "/active/" + filepath.Base(p))
	}
}

func depsDone(it item, status map[string]string) bool {
	for _, d := range it.DependsOn {
		if status[d] != "done" {
			return false
		}
	}
	return true
}

func printPlan(b *backlog) {
	for _, p := range b.Phases {
		fmt.Printf("\nPhase %d %s (%s). Exit gate: %s\n", p.Phase, p.Name, p.Status, p.ExitGate)
		for _, it := range b.Items {
			if it.Phase != p.Phase {
				continue
			}
			line := fmt.Sprintf("  %-6s %-11s %s", it.ID, it.Status, it.Title)
			if it.Critical {
				line += " [critical]"
			}
			if it.Owner != "" && it.Status != "done" {
				line += " (owner " + it.Owner + ")"
			}
			if e := it.Evidence; e != nil {
				line += fmt.Sprintf(" | verified %s @%s, approved by %s", e.Date, e.Commit, e.ApprovedBy)
			}
			fmt.Println(line)
		}
	}
}

func printItem(b *backlog, id string) bool {
	status := map[string]string{}
	for _, it := range b.Items {
		status[it.ID] = it.Status
	}
	for _, it := range b.Items {
		if it.ID != id {
			continue
		}
		owner := it.Owner
		if owner == "" {
			owner = "none"
		}
		fmt.Printf("%s %s\n", it.ID, it.Title)
		fmt.Printf("Phase %d · status %s · owner %s · critical %t\n", it.Phase, it.Status, owner, it.Critical)
		fmt.Println("Refs: " + strings.Join(it.Refs, ", "))
		var deps []string
		for _, d := range it.DependsOn {
			deps = append(deps, d+" ("+status[d]+")")
		}
		if len(deps) == 0 {
			deps = []string{"none"}
		}
		fmt.Println("Depends on: " + strings.Join(deps, ", "))
		fmt.Println("Done when: " + it.DoneWhen)
		fmt.Println("Verify:")
		for _, v := range it.Verify {
			fmt.Println("  " + v)
		}
		if it.Notes != "" {
			fmt.Println("Notes: " + it.Notes)
		}
		e := it.Evidence
		if e == nil {
			fmt.Println("Evidence: none")
			return true
		}
		fmt.Printf("Evidence: %s @%s, approved by %s\n", e.Date, e.Commit, e.ApprovedBy)
		for _, r := range e.Results {
			fmt.Printf("  %s → %s: %s\n", r.Command, r.Outcome, r.Output)
		}
		for _, m := range e.Mutations {
			fmt.Println("  mutation: " + m)
		}
		fmt.Println("  review: " + e.Review)
		fmt.Println("  report: " + e.Report)
		return true
	}
	return false
}
