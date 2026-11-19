package sparc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeFetcher stands in for the client, whose transport is tested in
// internal/ These tests are about the folder.
type fakeFetcher struct {
	docs    map[Kind][]Document
	fail    map[string]error // key → export error
	listErr map[Kind]error
}

func (f *fakeFetcher) List(_ context.Context, k Kind) ([]Document, error) {
	return f.docs[k], f.listErr[k]
}

func (f *fakeFetcher) Export(_ context.Context, d Document) (Result, error) {
	if err := f.fail[d.Key()]; err != nil {
		return Result{}, err
	}
	return Result{Body: []byte(fmt.Sprintf(`{"x":%q}`, d.Key()))}, nil
}

func fetcherWith(keys map[Kind][]string) *fakeFetcher {
	f := &fakeFetcher{docs: map[Kind][]Document{}, fail: map[string]error{}, listErr: map[Kind]error{}}
	for k, ks := range keys {
		for _, key := range ks {
			f.docs[k] = append(f.docs[k], Document{Kind: k, Slug: key})
		}
	}
	return f
}

func ls(t *testing.T, dir string) string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range es {
		names = append(names, e.Name())
	}
	return strings.Join(names, " ")
}

func TestPullWritesEachKindUnderTheMockNames(t *testing.T) {
	out := filepath.Join(t.TempDir(), "oscal")
	f := fetcherWith(map[Kind][]string{SSP: {"alpha"}, SAR: {"alpha-sar"}, POAM: {"7"}})
	var log bytes.Buffer
	if code := Pull(context.Background(), f, out, &log); code != 0 {
		t.Fatalf("exit %d: %s", code, log.String())
	}
	if got := ls(t, out); got != ".sparcpull poam-7.json sar-alpha-sar.json ssp-alpha.json" {
		t.Fatalf("wrote %q", got)
	}
	st, _ := os.Stat(out)
	if st.Mode().Perm()&0o077 != 0 {
		t.Errorf("out is %v, want owner-only", st.Mode().Perm())
	}
	st, _ = os.Stat(filepath.Join(out, "ssp-alpha.json"))
	if st.Mode().Perm()&0o077 != 0 {
		t.Errorf("a document is %v, want owner-only", st.Mode().Perm())
	}
}

func TestAnUnmarkedFolderWithFilesIsRefused(t *testing.T) {
	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "ssp-mine.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	if code := Pull(context.Background(), fetcherWith(nil), out, &log); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if _, err := os.Stat(filepath.Join(out, "ssp-mine.json")); err != nil {
		t.Fatalf("the tester's file was touched: %v", err)
	}
}

func TestACleanRunRemovesWhatSPARCNoLongerLists(t *testing.T) {
	out := filepath.Join(t.TempDir(), "o")
	var log bytes.Buffer
	Pull(context.Background(), fetcherWith(map[Kind][]string{SSP: {"a", "b"}}), out, &log)
	if err := os.WriteFile(filepath.Join(out, "notes.txt"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := Pull(context.Background(), fetcherWith(map[Kind][]string{SSP: {"a"}}), out, &log); code != 0 {
		t.Fatalf("exit %d: %s", code, log.String())
	}
	if got := ls(t, out); got != ".sparcpull notes.txt ssp-a.json" {
		t.Fatalf("after pruning: %q", got)
	}
	if !strings.Contains(log.String(), "removed ssp-b.json") {
		t.Errorf("the removal was not reported: %s", log.String())
	}
}

func TestAFailedRunRemovesNothingAndExitsOne(t *testing.T) {
	out := filepath.Join(t.TempDir(), "o")
	var log bytes.Buffer
	Pull(context.Background(), fetcherWith(map[Kind][]string{SSP: {"a", "b"}}), out, &log)

	f := fetcherWith(map[Kind][]string{SSP: {"a"}, SAR: {"bad"}})
	f.fail["bad"] = ErrNotConformed
	f.listErr[POAM] = errors.New("poam list down")
	log.Reset()
	if code := Pull(context.Background(), f, out, &log); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if got := ls(t, out); got != ".sparcpull ssp-a.json ssp-b.json" {
		t.Fatalf("a failed run changed the folder to %q", got)
	}
	for _, want := range []string{"does not conform", "poam list down", "2 failure(s)"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log lacks %q: %s", want, log.String())
		}
	}
}
