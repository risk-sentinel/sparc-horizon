package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTheTokenFileMustBeOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "token")
	if err := os.WriteFile(p, []byte("sparc_abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tok, err := readToken(p)
	if err != nil || tok != "sparc_abc" {
		t.Fatalf("got %q, %v", tok, err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	// Group-readable: the one bit added is the one the check must catch.
	if err := os.Chmod(p, st.Mode().Perm()|0o040); err != nil {
		t.Fatal(err)
	}
	if _, err := readToken(p); err == nil || !strings.Contains(err.Error(), "chmod 600") {
		t.Fatalf("a world-readable token file was accepted: %v", err)
	}
	if _, err := readToken(dir); err == nil {
		t.Fatal("a directory was accepted as a token file")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(p, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readToken(link); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("a symlink was followed: %v", err)
	}
	if _, err := readToken(filepath.Join(dir, "absent")); err == nil {
		t.Fatal("a missing token file was accepted")
	}
}

func TestFlagsRequireEverythingAndRefuseHTTP(t *testing.T) {
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, []byte("sparc_abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HORIZON_SPARC_URL", "")
	t.Setenv("HORIZON_SPARC_TOKEN_FILE", "")
	cases := map[string][]string{
		"no url":     {"-token-file", tok, "-out", dir},
		"no token":   {"-url", "https://s.example", "-out", dir},
		"no out":     {"-url", "https://s.example", "-token-file", tok},
		"extra arg":  {"-url", "https://s.example", "-token-file", tok, "-out", dir, "x"},
		"plain http": {"-url", "http://127.0.0.1", "-token-file", tok, "-out", dir, "-cache", filepath.Join(dir, "c")},
	}
	for name, args := range cases {
		var log bytes.Buffer
		if code := run(context.Background(), args, &log); code != 2 {
			t.Errorf("%s: exit %d, want 2 (%s)", name, code, log.String())
		}
	}
	var log bytes.Buffer
	if code := run(context.Background(), []string{"-h"}, &log); code != 0 {
		t.Errorf("-h exited %d", code)
	}
}
