// Command auditor runs the per-company and cross-company checks; it never writes. Not implemented yet.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "auditor: not implemented yet")
	os.Exit(1)
}
