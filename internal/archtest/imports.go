package archtest

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Rule names; Violation.Rule is one of them.
const (
	RuleHidden   = "E2-hidden"   // no package imports a folder the scan skips
	RuleProducts = "E2-products" // products import only their own product, platform and kit
	RuleAudit    = "E2-audit"    // audit imports only platform and audit
	RuleSim      = "E2-sim"      // simulators import only platform and sim
	RulePlatform = "E1-platform" // platform imports only platform
	RuleKit      = "E1-kit"      // kit imports only platform and kit
)

// Import is one import of a Bastion package by a Go file under the scanned root.
type Import struct {
	File string // file path relative to the root, slash-separated: "internal/pay/funding/funding.go"
	Line int    // line of the import spec
	From string // folder of File relative to the root: "internal/pay/funding"
	To   string // imported path relative to the module: "internal/tickets/orders" ("" for the module root)
}

// Violation is an Import that breaks Rule.
type Violation struct {
	Import

	Rule string
}

// String formats "<File>:<Line>: <From> imports <To> (<Rule>: <reason>)".
func (v Violation) String() string {
	return fmt.Sprintf("%s:%d: %s imports %s (%s: %s)", v.File, v.Line, v.From, v.To, v.Rule, reason(v.Rule))
}

// reason explains a rule in the words of its constant's comment.
func reason(rule string) string {
	switch rule {
	case RuleHidden:
		return "no package imports a folder the scan skips"
	case RuleProducts:
		return "products import only their own product, platform and kit"
	case RuleAudit:
		return "audit imports only platform and audit"
	case RuleSim:
		return "simulators import only platform and sim"
	case RulePlatform:
		return "platform imports only platform"
	case RuleKit:
		return "kit imports only platform and kit"
	default:
		return "unknown rule"
	}
}

// Scan reads the module path from <root>/go.mod and returns every import of a
// path equal to the module or under "<module>/" in every .go file below root,
// including _test.go files and files excluded by build tags, in file-walk
// order (lexical) then source order. Folders below root named testdata or
// starting with "." or "_" are skipped, as ./... skips them; RuleHidden
// forbids importing them.
// Errors: go.mod missing, unreadable or without a module line; a file that
// does not parse (the error names the file). Errors wrap the cause (%w).
func Scan(root string) ([]Import, error) {
	module, err := modulePath(root)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var imports []Import
	walk := func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		found, err := fileImports(fset, root, p, module)
		if err != nil {
			return err
		}
		imports = append(imports, found...)
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		return nil, fmt.Errorf("scan %s: %w", root, err)
	}
	return imports, nil
}

// modulePath returns the module path of <root>/go.mod.
func modulePath(root string) (string, error) {
	name := filepath.Join(root, "go.mod")
	data, err := os.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("read module path: %w", err)
	}
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "module" {
			continue
		}
		if unquoted, err := strconv.Unquote(fields[1]); err == nil {
			return unquoted, nil
		}
		return fields[1], nil
	}
	return "", fmt.Errorf("%s has no module line", name)
}

// skipDir reports whether the go command ignores a folder of this name.
func skipDir(name string) bool {
	return name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

// fileImports returns the imports of module packages in the file at p.
func fileImports(fset *token.FileSet, root, p, module string) ([]Import, error) {
	f, err := parser.ParseFile(fset, p, nil, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return nil, fmt.Errorf("relative path of %s: %w", p, err)
	}
	file := filepath.ToSlash(rel)
	var imports []Import
	for _, spec := range f.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("%s: import %s: %w", file, spec.Path.Value, err)
		}
		to, ok := modulePackage(imported, module)
		if !ok {
			continue
		}
		imports = append(imports, Import{
			File: file,
			Line: fset.Position(spec.Path.Pos()).Line,
			From: path.Dir(file),
			To:   to,
		})
	}
	return imports, nil
}

// modulePackage returns the imported path relative to the module, and false
// for a path outside the module (standard library, third parties).
func modulePackage(imported, module string) (string, bool) {
	if imported == module {
		return "", true
	}
	return strings.CutPrefix(imported, module+"/")
}

// Check returns the imports that break a rule, in input order.
func Check(imports []Import) []Violation {
	var violations []Violation
	for _, i := range imports {
		if rule, broken := ruleFor(i.From, i.To); broken {
			violations = append(violations, Violation{Import: i, Rule: rule})
		}
	}
	return violations
}

// ruleFor returns the rule that governs importing to from the folder from,
// and whether the import breaks it (brief P1-04 › Contracts › Rules).
func ruleFor(from, to string) (string, bool) {
	if slices.ContainsFunc(strings.Split(to, "/"), skipDir) {
		return RuleHidden, true
	}
	rule, allowed := allowList(from)
	if rule == "" {
		return "", false
	}
	return rule, !slices.ContainsFunc(allowed, func(x string) bool { return under(to, x) })
}

// allowList returns the rule of the importer folder from and the folders it
// may import, or "" for a folder no rule covers.
func allowList(from string) (string, []string) {
	switch {
	case under(from, "internal/pay") || strings.HasPrefix(from, "cmd/pay-"):
		return RuleProducts, []string{"internal/pay", "internal/platform", "internal/kit"}
	case under(from, "internal/tickets") || strings.HasPrefix(from, "cmd/tickets-"):
		return RuleProducts, []string{"internal/tickets", "internal/platform", "internal/kit"}
	case under(from, "internal/audit") || under(from, "cmd/auditor"):
		return RuleAudit, []string{"internal/platform", "internal/audit"}
	case under(from, "internal/sim") || under(from, "cmd/pspsim") || under(from, "cmd/banksim"):
		return RuleSim, []string{"internal/platform", "internal/sim"}
	case under(from, "internal/platform"):
		return RulePlatform, []string{"internal/platform"}
	case under(from, "internal/kit"):
		return RuleKit, []string{"internal/platform", "internal/kit"}
	default:
		return "", nil
	}
}

// under reports whether p is the folder x or below it.
func under(p, x string) bool {
	return p == x || strings.HasPrefix(p, x+"/")
}
