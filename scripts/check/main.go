// Command check runs the baseline behind `task check`: build, vet,
// golangci-lint, govulncheck, unit tests and the plan check. Steps run in
// order and stop at the first failure; only a failure prints its output, and
// a green run prints "check OK".
//
//	go run ./scripts/check
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
)

// step is one baseline command; args[0] is the program.
type step struct {
	name string
	args []string
}

// execFunc runs one step and returns its combined stdout and stderr.
type execFunc func(ctx context.Context, s step) ([]byte, error)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Stdout, baseline(), execStep)
	stop()
	if err != nil {
		os.Exit(1)
	}
}

// baseline returns the steps of task check, in order.
func baseline() []step {
	return []step{
		{"build", []string{"go", "build", "./..."}},
		{"vet", []string{"go", "vet", "./..."}},
		{"lint", []string{"go", "tool", "-modfile=tools.mod", "golangci-lint", "run", "./..."}},
		{"vuln", []string{"go", "tool", "-modfile=tools.mod", "govulncheck", "./..."}},
		{"unit", []string{"go", "test", "-short", "./..."}},
		{"plan", []string{"go", "run", "scripts/brief/main.go", "-check"}},
	}
}

// run executes steps in order and stops at the first failure.
func run(ctx context.Context, w io.Writer, steps []step, execute execFunc) error {
	for _, s := range steps {
		out, err := execute(ctx, s)
		if err == nil {
			continue
		}
		msg := fmt.Sprintf("FAIL %s: %s\n%s", s.name, strings.Join(s.args, " "), out)
		if len(out) > 0 && !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		if _, werr := io.WriteString(w, msg); werr != nil {
			return fmt.Errorf("write %s output: %w", s.name, werr)
		}
		return fmt.Errorf("%s: %w", s.name, err)
	}
	if _, err := io.WriteString(w, "check OK\n"); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}

// execStep runs s with exec.CommandContext in the current directory.
func execStep(ctx context.Context, s step) ([]byte, error) {
	out, err := exec.CommandContext(ctx, s.args[0], s.args[1:]...).CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("run %s: %w", s.args[0], err)
	}
	return out, nil
}
