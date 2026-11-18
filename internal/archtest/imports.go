package archtest

import "errors"

// Rule names; Violation.Rule is one of them.
const (
	RuleProducts = "E2-products" // pay and tickets never import each other
	RuleAudit    = "E2-audit"    // audit imports only platform
	RuleSim      = "E2-sim"      // simulators import only platform
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
func (v Violation) String() string { return "" }

// Scan reads the module path from <root>/go.mod and returns every import of a
// Bastion package in every .go file below root.
func Scan(root string) ([]Import, error) { return nil, errors.New("not implemented") }

// Check returns the imports that break a rule, in input order.
func Check(imports []Import) []Violation { return nil }
