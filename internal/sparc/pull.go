package sparc

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// Pull's folder is its own: the first run marks it, a later run refuses an
// unmarked folder that holds anything, and a clean run removes the documents
// SPARC no longer lists. Without the mark, pruning could delete a tester's
// own files.
const marker = ".sparcpull"

// Fetcher is what Pull needs from a Client.
type Fetcher interface {
	List(ctx context.Context, k Kind) ([]Document, error)
	Export(ctx context.Context, d Document) (Result, error)
}

// Pull lists and exports every kind into out, logging a line per kind and
// per failure, and returns the process exit code: 0 clean, 1 when anything
// failed (what succeeded is written and nothing is removed), 2 when out is
// not a folder Pull may write.
func Pull(ctx context.Context, c Fetcher, out string, stderr io.Writer) int {
	root, err := claim(out)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "sparcpull:", err)
		return 2
	}
	defer func() { _ = root.Close() }()
	p := &puller{c: c, root: root, log: stderr, written: map[string]bool{}}
	for _, k := range Kinds {
		p.kind(ctx, k)
	}
	failures := p.failures
	written := p.written
	if len(failures) > 0 {
		_, _ = fmt.Fprintf(stderr, "sparcpull: %d failure(s); nothing was removed from %s\n", len(failures), out)
		return 1
	}
	removed, err := prune(root, written)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "sparcpull:", err)
		return 1
	}
	for _, r := range removed {
		_, _ = fmt.Fprintf(stderr, "removed %s: SPARC no longer lists it\n", r)
	}
	return 0
}

type puller struct {
	c        Fetcher
	root     *os.Root
	log      io.Writer
	written  map[string]bool
	failures []string
}

func (p *puller) fail(err error) {
	p.failures = append(p.failures, err.Error())
	_, _ = fmt.Fprintln(p.log, "sparcpull:", err)
}

// kind lists one kind and writes each export, recording every failure and
// carrying on, so one bad document does not hide the state of the rest.
func (p *puller) kind(ctx context.Context, k Kind) {
	docs, err := p.c.List(ctx, k)
	if err != nil {
		p.fail(err)
		return
	}
	fetched, unchanged := 0, 0
	for _, d := range docs {
		res, err := p.c.Export(ctx, d)
		if err != nil {
			p.fail(err)
			continue
		}
		name := string(k) + "-" + d.Key() + ".json"
		if err := writeAtomic(p.root, name, res.Body); err != nil {
			p.fail(fmt.Errorf("writing %s: %w", name, err))
			continue
		}
		p.written[name] = true
		if res.Unchanged {
			unchanged++
		} else {
			fetched++
		}
	}
	_, _ = fmt.Fprintf(p.log, "%-4s %d listed, %d transferred, %d unchanged (304)\n", k, len(docs), fetched, unchanged)
}

// claim makes out sparcpull's folder: created owner-only if missing, marked if
// empty, and refused if it holds anything without the mark. Everything after
// goes through the returned os.Root, so no document name reaches outside out.
func claim(out string) (*os.Root, error) {
	if err := os.MkdirAll(out, 0o700); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(out)
	if err != nil {
		return nil, err
	}
	if err := mark(root, out); err != nil {
		_ = root.Close()
		return nil, err
	}
	return root, nil
}

func mark(root *os.Root, out string) error {
	if _, err := root.Stat(marker); err == nil {
		return nil
	}
	entries, err := readDir(root)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s is not empty and was not written by sparcpull; use an empty or new folder", out)
	}
	return root.WriteFile(marker, []byte("written by sparcpull; files here are replaced and removed\n"), 0o600)
}

func readDir(root *os.Root) ([]fs.DirEntry, error) {
	return fs.ReadDir(root.FS(), ".")
}

// prune removes the documents a previous run wrote that this run did not.
// Only names sparcpull itself writes are candidates.
func prune(root *os.Root, keep map[string]bool) ([]string, error) {
	entries, err := readDir(root)
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || keep[name] || !ours(name) {
			continue
		}
		if err := root.Remove(name); err != nil {
			return removed, err
		}
		removed = append(removed, name)
	}
	sort.Strings(removed)
	return removed, nil
}

func ours(name string) bool {
	if !strings.HasSuffix(name, ".json") {
		return false
	}
	for _, k := range Kinds {
		if strings.HasPrefix(name, string(k)+"-") {
			return true
		}
	}
	return false
}
