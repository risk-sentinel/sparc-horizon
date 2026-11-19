//go:build unix

package main

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A directory fails at the read whether or not the regular-file check exists,
// so it cannot show the check working. A FIFO can: without the check, reading
// it blocks until something writes, and the pull hangs.
func TestATokenFIFOIsRefusedWithoutBlocking(t *testing.T) {
	p := filepath.Join(t.TempDir(), "token")
	if err := syscall.Mkfifo(p, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := readToken(p)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("got %v, want a refusal", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("readToken blocked on a FIFO")
	}
}
