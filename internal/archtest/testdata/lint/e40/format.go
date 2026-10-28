// Package e40 violates E40: formatting, import grouping and bare nolint directives.
package e40
import "strings"

// Upper upper-cases s.
func Upper(s string) string {
	return strings.ToUpper(s) //nolint
}
