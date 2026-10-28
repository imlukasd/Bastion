package main

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestBaseline_Steps(t *testing.T) {
	want := []step{
		{"build", []string{"go", "build", "./..."}},
		{"vet", []string{"go", "vet", "./..."}},
		{"lint", []string{"go", "tool", "-modfile=tools.mod", "golangci-lint", "run", "./..."}},
		{"vuln", []string{"go", "tool", "-modfile=tools.mod", "govulncheck", "./..."}},
		{"unit", []string{"go", "test", "-short", "./..."}},
		{"plan", []string{"go", "run", "scripts/brief/main.go", "-check"}},
	}
	if got := baseline(); !reflect.DeepEqual(got, want) {
		t.Errorf("baseline() = %q\nwant %q", got, want)
	}
}

func TestRun_PassingStepsPrintOnlyCheckOK(t *testing.T) {
	var ran []string
	fake := func(_ context.Context, s step) ([]byte, error) {
		ran = append(ran, s.name)
		return []byte("output of " + s.name + "\n"), nil
	}
	var out bytes.Buffer

	if err := run(t.Context(), &out, baseline(), fake); err != nil {
		t.Fatalf("run: %v", err)
	}
	if want := names(baseline()); !slices.Equal(ran, want) {
		t.Errorf("ran %q, want %q", ran, want)
	}
	if got := out.String(); got != "check OK\n" {
		t.Errorf("output %q, want %q", got, "check OK\n")
	}
}

func TestRun_FailingStepPrintsItsOutputAndStops(t *testing.T) {
	errVet := errors.New("exit status 1")
	var ran []string
	fake := func(_ context.Context, s step) ([]byte, error) {
		ran = append(ran, s.name)
		if s.name == "vet" {
			return []byte("x.go:3:2: unreachable code"), errVet
		}
		return []byte("output of " + s.name + "\n"), nil
	}
	var out bytes.Buffer

	err := run(t.Context(), &out, baseline(), fake)

	if !errors.Is(err, errVet) || !strings.HasPrefix(err.Error(), "vet: ") {
		t.Errorf("error %v, want vet: wrapping %v", err, errVet)
	}
	if want := []string{"build", "vet"}; !slices.Equal(ran, want) {
		t.Errorf("ran %q, want %q", ran, want)
	}
	if got, want := out.String(), "FAIL vet: go vet ./...\nx.go:3:2: unreachable code\n"; got != want {
		t.Errorf("output %q, want %q", got, want)
	}
}

func names(steps []step) []string {
	var n []string
	for _, s := range steps {
		n = append(n, s.name)
	}
	return n
}
