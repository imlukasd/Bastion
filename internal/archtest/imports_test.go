package archtest

import (
	"errors"
	"fmt"
	"go/scanner"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// archFixtures holds one mini repository per case (brief P1-04 › Contracts).
const archFixtures = "testdata/arch"

// TestImports_RepoFollowsRules: the repository itself breaks no import rule,
// and the scan is not vacuous (it sees a known internal import).
func TestImports_RepoFollowsRules(t *testing.T) {
	imports, err := Scan(repoRoot)
	if err != nil {
		t.Fatalf("Scan(repo): %v", err)
	}
	known := Import{
		File: "internal/platform/db/dbtest/dbtest.go",
		From: "internal/platform/db/dbtest",
		To:   "internal/platform/db/migrate",
	}
	if !slices.ContainsFunc(imports, func(i Import) bool {
		return i.File == known.File && i.From == known.From && i.To == known.To
	}) {
		t.Errorf("Scan(repo) did not report %s importing %s; found %d imports", known.From, known.To, len(imports))
	}
	for _, v := range Check(imports) {
		t.Errorf("violation: %s", v)
	}
}

// TestImports_FixturesReportExpectedViolations: each fixture reports exactly
// its violations, as "file:line rule", in Scan order and without duplicates.
func TestImports_FixturesReportExpectedViolations(t *testing.T) {
	cases := map[string][]string{
		"products": {
			"cmd/pay-api/main.go:7 " + RuleProducts,
			"internal/pay/funding/bridge.go:3 " + RuleProducts, // a package no rule covers
			"internal/pay/funding/funding.go:5 " + RuleProducts,
			"internal/pay/intents/named.go:4 " + RuleProducts, // named import
			"internal/pay/intents/named.go:5 " + RuleProducts, // blank import
			"internal/pay/intents/named.go:6 " + RuleProducts, // dot import
			"internal/pay/limits/limits.go:4 " + RuleProducts, // kitx is not kit
			"internal/pay/limits/limits.go:5 " + RuleProducts, // sim (D15)
			"internal/pay/root.go:3 " + RuleProducts,          // the module root
			"internal/tickets/orders/bridge.go:3 " + RuleProducts, // a package no rule covers
			"internal/tickets/orders/orders_test.go:3 " + RuleProducts,
		},
		"hidden": {
			"internal/pay/p/p.go:4 " + RuleHidden,
			"internal/pay/p/p.go:5 " + RuleHidden,
			"internal/pay/p/p.go:6 " + RuleHidden,
			"test/integration/helpers.go:3 " + RuleHidden,
		},
		"audit": {
			"cmd/auditor/main.go:5 " + RuleAudit,
			"internal/audit/pay/check.go:5 " + RuleAudit,
			"internal/audit/pay/check.go:6 " + RuleAudit,
			"internal/audit/tickets/check.go:3 " + RuleAudit, // platformx is not platform
		},
		"sim": {
			"cmd/pspsim/main.go:4 " + RuleSim,
			"internal/sim/bank/bank.go:4 " + RuleSim,
			"internal/sim/bank/bank.go:6 " + RuleSim,
			"internal/sim/psp/card.go:3 " + RuleSim, // simulators is not sim
			"internal/sim/psp/psp.go:3 " + RuleSim,
		},
		"layers": {
			"internal/kit/fsm/fsm.go:3 " + RuleKit, // kitx is not kit
			"internal/kit/ledger/ledger.go:4 " + RuleKit,
			"internal/kit/outbox/outbox.go:3 " + RuleKit,
			"internal/platform/db/db.go:3 " + RulePlatform,
			"internal/platform/ids/ids.go:3 " + RulePlatform, // platformx is not platform
		},
		"clean": nil,
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			imports, err := Scan(filepath.Join(archFixtures, name))
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			got := make([]string, 0)
			for _, v := range Check(imports) {
				got = append(got, fmt.Sprintf("%s:%d %s", v.File, v.Line, v.Rule))
			}
			if !slices.Equal(got, want) {
				t.Errorf("violations, in order:\n got %q\nwant %q", got, want)
			}
		})
	}
}

// TestScan_Clean: the clean fixture yields exactly its 27 Bastion imports:
// none from the standard library, third parties or skipped folders, and the
// file excluded by a build tag is read.
func TestScan_Clean(t *testing.T) {
	imports, err := Scan(filepath.Join(archFixtures, "clean"))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(imports) != 27 {
		t.Errorf("Scan found %d imports, want 27: %v", len(imports), imports)
	}
	first := Import{File: "internal/pay/funding/funding.go", Line: 8, From: "internal/pay/funding", To: "internal/kit/ledger"}
	if !slices.Contains(imports, first) {
		t.Errorf("Scan did not report %+v", first)
	}
	tagged := Import{File: "internal/pay/tools.go", Line: 5, From: "internal/pay", To: "internal/platform/ids"}
	if !slices.Contains(imports, tagged) {
		t.Errorf("Scan did not read the //go:build ignore file: want %+v", tagged)
	}
	for _, i := range imports {
		for _, skipped := range []string{"/testdata/", "/.hidden/", "/_old/"} {
			if strings.Contains(i.File, skipped) {
				t.Errorf("Scan read a skipped folder: %+v", i)
			}
		}
	}
}

// TestScan_Errors: a missing go.mod, a go.mod without a module line and a
// file that does not parse are errors; errors wrap their cause and the parse
// error names the file.
func TestScan_Errors(t *testing.T) {
	if _, err := Scan(t.TempDir()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Scan without go.mod: error %v, want one wrapping fs.ErrNotExist", err)
	}

	noModule := t.TempDir()
	if err := os.WriteFile(filepath.Join(noModule, "go.mod"), []byte("go 1.27.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(noModule); err == nil {
		t.Error("Scan with a go.mod without a module line: no error")
	}

	_, err := Scan(filepath.Join(archFixtures, "broken"))
	if err == nil {
		t.Fatal("Scan(broken): no error")
	}
	var parseErrs scanner.ErrorList
	if !errors.As(err, &parseErrs) {
		t.Errorf("Scan(broken) error %v does not wrap the parser's scanner.ErrorList", err)
	}
	if !strings.Contains(filepath.ToSlash(err.Error()), "internal/x/x.go") {
		t.Errorf("Scan(broken) error %q does not name internal/x/x.go", err)
	}
}

// TestViolation_String: the message names file, line, both packages, the rule
// and its reason.
func TestViolation_String(t *testing.T) {
	v := Violation{
		Import: Import{File: "internal/pay/funding/funding.go", Line: 5, From: "internal/pay/funding", To: "internal/tickets/orders"},
		Rule:   RuleProducts,
	}
	want := "internal/pay/funding/funding.go:5: internal/pay/funding imports internal/tickets/orders (E2-products: products import only their own product, platform and kit)"
	if got := v.String(); got != want {
		t.Errorf("String:\n got %q\nwant %q", got, want)
	}

	reasons := map[string]string{
		RuleHidden:   "no package imports a folder the scan skips",
		RuleProducts: "products import only their own product, platform and kit",
		RuleAudit:    "audit imports only platform",
		RuleSim:      "simulators import only platform",
		RulePlatform: "platform imports only platform",
		RuleKit:      "kit imports only platform and kit",
	}
	for rule, reason := range reasons {
		v.Rule = rule
		if got := v.String(); !strings.HasSuffix(got, "("+rule+": "+reason+")") {
			t.Errorf("String for %s = %q, want the suffix (%s: %s)", rule, got, rule, reason)
		}
	}
}
