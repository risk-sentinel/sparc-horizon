// Command sparcpull copies one SPARC instance's OSCAL documents (SSPs,
// assessment plans, assessment results, POA&Ms) into a folder that the mock
// API server's -dir and the container read (#124).
//
//	sparcpull -url https://sparc.example.gov -token-file ~/.config/sparc/token -out ./my-oscal
//
// It is a fetch, not ingestion: nothing here verifies a signature, so what it
// writes is demo input for the local mock, never TM-1's ingestion path (P2).
//
// The folder is sparcpull's: the first run marks it, a later run refuses an
// unmarked folder that holds anything, and a clean run removes the documents
// SPARC no longer lists. A run with any failure writes what succeeded, removes
// nothing, and exits 1.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/risk-sentinel/sparc-horizon/internal/sparc"
)

type options struct {
	url, tokenFile, out, cache string
	timeout                    time.Duration
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stderr io.Writer) int {
	opts, err := parseFlags(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		_, _ = fmt.Fprintln(stderr, "sparcpull:", err)
		return 2
	}
	c, err := client(opts)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "sparcpull:", err)
		return 2
	}
	return sparc.Pull(ctx, c, opts.out, stderr)
}

func parseFlags(args []string, stderr io.Writer) (options, error) {
	fs := flag.NewFlagSet("sparcpull", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var o options
	fs.StringVar(&o.url, "url", os.Getenv("HORIZON_SPARC_URL"), "SPARC base URL, https only (env HORIZON_SPARC_URL)")
	fs.StringVar(&o.tokenFile, "token-file", os.Getenv("HORIZON_SPARC_TOKEN_FILE"),
		"file holding the SPARC API token, owner-only (env HORIZON_SPARC_TOKEN_FILE)")
	fs.StringVar(&o.out, "out", "", "folder to write the documents into (required)")
	fs.StringVar(&o.cache, "cache", "", "ETag cache folder (default: the user cache dir)")
	fs.DurationVar(&o.timeout, "timeout", sparc.DefaultTimeout, "per-request timeout")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	switch {
	case fs.NArg() > 0:
		return o, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	case o.url == "":
		return o, errors.New("-url is required")
	case o.tokenFile == "":
		return o, errors.New("-token-file is required; the token is never taken on the command line")
	case o.out == "":
		return o, errors.New("-out is required")
	}
	return o, nil
}

func client(o options) (*sparc.Client, error) {
	tok, err := readToken(o.tokenFile)
	if err != nil {
		return nil, err
	}
	dir := o.cache
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("no cache directory: %w; pass -cache", err)
		}
		dir = filepath.Join(base, "sparc-horizon", "sparcpull")
	}
	cache, err := sparc.OpenCache(dir)
	if err != nil {
		return nil, err
	}
	return sparc.New(sparc.Config{BaseURL: o.url, Token: tok, Cache: cache, Timeout: o.timeout})
}

// readToken reads the token from a file only the owner can read.
//
// NIST: IA-5 — the authenticator is never a flag (visible in the process
// list and shell history) and is refused from a file others can read, the
// rule ssh applies to a private key.
//
// The path is checked before it is opened, because opening a FIFO blocks until
// something writes to it, and a symlink is refused rather than followed. The
// open handle is then checked again, so the mode that was checked is the mode
// of the file that was read.
func readToken(path string) (string, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return "", fmt.Errorf("token file: %w", err)
	}
	defer func() { _ = root.Close() }()
	name := filepath.Base(path)
	st, err := root.Lstat(name)
	if err != nil {
		return "", fmt.Errorf("token file: %w", err)
	}
	if err := ownerOnlyFile(path, st); err != nil {
		return "", err
	}
	f, err := root.Open(name)
	if err != nil {
		return "", fmt.Errorf("token file: %w", err)
	}
	defer func() { _ = f.Close() }()
	if st, err = f.Stat(); err != nil {
		return "", fmt.Errorf("token file: %w", err)
	}
	if err := ownerOnlyFile(path, st); err != nil {
		return "", err
	}
	b, err := io.ReadAll(io.LimitReader(f, 64<<10))
	if err != nil {
		return "", fmt.Errorf("token file: %w", err)
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}

func ownerOnlyFile(path string, st os.FileInfo) error {
	if !st.Mode().IsRegular() {
		return fmt.Errorf("token file %s is not a regular file", path)
	}
	if st.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("token file %s is %v; it must be readable by its owner only (chmod 600)", path, st.Mode().Perm())
	}
	return nil
}
