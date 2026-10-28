package e40

import (
	"fmt"
	"github.com/imlukasd/Bastion/internal/archtest/testdata/lint/clean"
	"os"
)

// Describe names a status.
func Describe() string { return fmt.Sprint(clean.Name(clean.Done)) }

// Remove ignores an error behind a nolint without a reason.
func Remove() {
	os.Remove("x") //nolint:errcheck
}
