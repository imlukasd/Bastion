package archtest

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// repoRoot is the repository root, seen from this package's folder.
const repoRoot = "../.."

// fixtureDir holds one package per lint rule (folder eNN = rule ENN) and
// clean, which follows every rule.
const fixtureDir = "internal/archtest/testdata/lint"

// issue is one golangci-lint finding; file is relative to fixtureDir.
type issue struct {
	file   string
	line   int
	linter string
}

func (i issue) String() string { return fmt.Sprintf("%s:%d %s", i.file, i.line, i.linter) }

// TestLint_RulesMarkedLintHaveFixtures: every rule docs/ENGINEERING.md marks
// "lint" has a fixture folder and at least one expected issue.
func TestLint_RulesMarkedLintHaveFixtures(t *testing.T) {
	rules := lintRules(t)
	if len(rules) == 0 {
		t.Fatal("no rule in docs/ENGINEERING.md is marked lint")
	}
	want := expectedIssues()
	for _, id := range rules {
		folder := fmt.Sprintf("e%02d", id)
		if fi, err := os.Stat(filepath.Join("testdata", "lint", folder)); err != nil || !fi.IsDir() {
			t.Errorf("E%d is marked lint in docs/ENGINEERING.md but %s/%s does not exist", id, fixtureDir, folder)
		}
		if !slices.ContainsFunc(want, func(i issue) bool { return strings.HasPrefix(i.file, folder+"/") }) {
			t.Errorf("E%d is marked lint but expectedIssues has no issue in %s", id, folder)
		}
	}
}

// TestLint_FixturesReportExpectedIssues runs golangci-lint with the
// repository's configuration on the fixtures: it reports exactly the
// expected issues, forbidigo names the rule, and clean has no issue.
func TestLint_FixturesReportExpectedIssues(t *testing.T) {
	if testing.Short() {
		t.Skip("runs golangci-lint; task test runs it")
	}
	got := runLint(t)

	var gotIssues []issue
	for _, r := range got {
		gotIssues = append(gotIssues, r.issue)
		if r.linter == "forbidigo" {
			ruleID := ruleOf(t, r.file) + ":"
			if !strings.Contains(r.text, ruleID) {
				t.Errorf("%v: text %q does not name %s", r.issue, r.text, ruleID)
			}
		}
	}
	missing, unexpected := diff(expectedIssues(), gotIssues)
	for _, i := range missing {
		t.Errorf("expected, not reported: %v", i)
	}
	for _, i := range unexpected {
		t.Errorf("reported, not expected: %v", i)
	}
}

// expectedIssues is everything golangci-lint must report on the fixtures.
func expectedIssues() []issue {
	return []issue{
		{"e05/state.go", 4, "gochecknoglobals"},
		{"e05/state.go", 6, "gochecknoinits"},
		{"e08/money.go", 10, "forbidigo"},    // float64 parameter
		{"e08/money.go", 11, "forbidigo"},    // float64(...) conversion
		{"e08/money.go", 16, "forbidigo"},    // strconv.ParseFloat
		{"e09/status.go", 16, "exhaustive"},  // switch without default
		{"e09/status.go", 27, "exhaustive"},  // a default does not count
		{"e11/complex.go", 4, "gocognit"},    // cognitive complexity 16
		{"e11/flag.go", 4, "revive"},         // flag-parameter
		{"e11/long.go", 69, "funlen"},        // 61-line body; the 60-line body is not reported
		{"e13/errs.go", 15, "errcheck"},      // unchecked os.Remove
		{"e13/errs.go", 19, "errorlint"},     // err == ErrX
		{"e13/errs.go", 22, "errorlint"},     // %v instead of %w
		{"e21/clock.go", 7, "forbidigo"},     // time.Now
		{"e21/clock.go", 10, "forbidigo"},    // time.Since
		{"e28/ctx.go", 7, "revive"},          // context-as-argument
		{"e28/ctx.go", 10, "forbidigo"},      // context.Background
		{"e28/ctx.go", 13, "forbidigo"},      // context.TODO
		{"e36/secret.go", 6, "gosec"},        // G101
		{"e37/doc.go", 4, "revive"},          // exported without a doc comment
		{"e37/doc.go", 5, "gocritic"},        // commentedOutCode
		{"e40/format.go", 2, "gofmt"},        // not gofmt-formatted
		{"e40/format.go", 7, "nolintlint"},   // bare //nolint
		{"e40/imports.go", 5, "goimports"},   // local import not in its own group
		{"e40/imports.go", 14, "nolintlint"}, // //nolint:errcheck without a reason
	}
}

// lintRules returns the IDs of the rules whose "Enforced by" column in
// docs/ENGINEERING.md lists lint.
func lintRules(t *testing.T) []int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, "docs", "ENGINEERING.md"))
	if err != nil {
		t.Fatalf("read ENGINEERING.md: %v", err)
	}
	rowRe := regexp.MustCompile(`^\| E(\d+) \|.*\| ([^|]+) \|$`)
	var ids []int
	for line := range strings.Lines(string(data)) {
		m := rowRe.FindStringSubmatch(strings.TrimRight(line, "\r\n"))
		if m == nil {
			continue
		}
		for by := range strings.SplitSeq(m[2], ",") {
			if strings.TrimSpace(by) == "lint" {
				id, _ := strconv.Atoi(m[1]) // \d+ always parses
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// report is one issue with the linter's text.
type report struct {
	issue
	text string
}

// runLint runs golangci-lint on the fixtures from the repository root.
func runLint(t *testing.T) []report {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "go", "tool", "-modfile=tools.mod", "golangci-lint", "run",
		"--output.json.path=stdout", "--show-stats=false", "./"+fixtureDir+"/...")
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	if err != nil && (!errors.As(err, &exitErr) || exitErr.ExitCode() != 1) {
		t.Fatalf("golangci-lint: %v\n%s", err, stderr.Bytes())
	}
	var parsed struct {
		Issues []struct {
			FromLinter string
			Text       string
			Pos        struct {
				Filename string
				Line     int
			}
		}
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("parse golangci-lint output: %v\n%s", err, out)
	}
	reports := make([]report, 0, len(parsed.Issues))
	for _, i := range parsed.Issues {
		file := strings.TrimPrefix(filepath.ToSlash(i.Pos.Filename), fixtureDir+"/")
		reports = append(reports, report{issue{file, i.Pos.Line, i.FromLinter}, i.Text})
	}
	return reports
}

// ruleOf returns the rule ID a fixture file belongs to: e08/money.go → E8.
func ruleOf(t *testing.T, file string) string {
	t.Helper()
	folder, _, _ := strings.Cut(file, "/")
	n, err := strconv.Atoi(strings.TrimPrefix(folder, "e"))
	if err != nil {
		t.Fatalf("%s is not in a rule folder eNN", file)
	}
	return "E" + strconv.Itoa(n)
}

// diff returns the issues only in want and the issues only in got.
func diff(want, got []issue) (missing, unexpected []issue) {
	order := func(a, b issue) int {
		return cmp.Or(cmp.Compare(a.file, b.file), cmp.Compare(a.line, b.line), cmp.Compare(a.linter, b.linter))
	}
	want, got = slices.Clone(want), slices.Clone(got)
	slices.SortFunc(want, order)
	slices.SortFunc(got, order)
	for len(want) > 0 || len(got) > 0 {
		switch {
		case len(got) == 0 || len(want) > 0 && order(want[0], got[0]) < 0:
			missing, want = append(missing, want[0]), want[1:]
		case len(want) == 0 || order(want[0], got[0]) > 0:
			unexpected, got = append(unexpected, got[0]), got[1:]
		default:
			want, got = want[1:], got[1:]
		}
	}
	return missing, unexpected
}
