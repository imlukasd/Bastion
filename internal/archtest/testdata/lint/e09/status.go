// Package e09 violates E9: switches over a closed set that miss a constant.
package e09

// Status is a closed set.
type Status int

// The statuses.
const (
	Pending Status = iota
	Done
	Failed
)

// Name misses Failed.
func Name(s Status) string {
	switch s {
	case Pending:
		return "pending"
	case Done:
		return "done"
	}
	return ""
}

// Label misses Failed behind a default.
func Label(s Status) string {
	switch s {
	case Pending:
		return "Pending"
	case Done:
		return "Done"
	default:
		return ""
	}
}
