package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// run is exercised from a temporary working directory, because it resolves
// both paths inside the working directory and nowhere else.
func inTemp(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
}

func TestRunWritesTheIgnoreFile(t *testing.T) {
	inTemp(t)
	if err := os.WriteFile("b.yml", []byte("findings: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run("b.yml", "out.trivyignore", time.Now()); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := os.ReadFile("out.trivyignore")
	if err != nil || !strings.HasPrefix(string(got), "# Generated from b.yml") {
		t.Fatalf("ignore file = %q, %v", got, err)
	}
}

func TestRunFailsOnAnInvalidBaseline(t *testing.T) {
	inTemp(t)
	if err := os.WriteFile("b.yml", []byte("findings:\n  - cve_id: nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run("b.yml", "", time.Now()); err == nil || !strings.Contains(err.Error(), "problem(s)") {
		t.Fatalf("run = %v, want a problem count", err)
	}
}

func TestRunRefusesPathsOutsideTheWorkingDirectory(t *testing.T) {
	inTemp(t)
	if err := os.WriteFile("b.yml", []byte("findings: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run("../../etc/passwd", "", time.Now()); err == nil {
		t.Fatal("read outside the working directory")
	}
	if err := run("b.yml", "../escape.trivyignore", time.Now()); err == nil {
		t.Fatal("wrote outside the working directory")
	}
	if err := run("missing.yml", "", time.Now()); err == nil {
		t.Fatal("a missing baseline passed")
	}
}
