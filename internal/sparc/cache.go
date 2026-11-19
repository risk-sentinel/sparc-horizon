package sparc

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Cache keeps each export's ETag and bytes on disk, so a document SPARC has
// not changed is answered by a 304 and not transferred again.
//
// The cached bytes are checked against the digest recorded with them before
// they are used. A cache file edited or truncated on disk is a miss, never a
// document served as SPARC's.
//
// Every read and write goes through an os.Root, so a cache key can name
// nothing outside the cache directory even if checkKey were bypassed.
type Cache struct{ root *os.Root }

// OpenCache uses dir, creating it owner-only. The bodies are authorization
// documents, so the directory is not readable by anyone else.
func OpenCache(dir string) (*Cache, error) {
	if dir == "" {
		return nil, errors.New("sparc: no cache directory")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("sparc: cache: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("sparc: cache: %w", err)
	}
	return &Cache{root: root}, nil
}

type entry struct {
	ETag   string `json:"etag"`
	SHA256 string `json:"sha256"`
	Body   []byte `json:"-"`
}

func paths(k Kind, key string) (meta, body string) {
	base := string(k) + "-" + key
	return base + ".etag.json", base + ".json"
}

func (c *Cache) load(k Kind, key string) (*entry, bool) {
	metaPath, bodyPath := paths(k, key)
	m, err := c.root.ReadFile(metaPath)
	if err != nil {
		return nil, false
	}
	var e entry
	if json.Unmarshal(m, &e) != nil || e.ETag == "" {
		return nil, false
	}
	b, err := c.root.ReadFile(bodyPath)
	if err != nil {
		return nil, false
	}
	if digest(b) != e.SHA256 {
		return nil, false
	}
	e.Body = b
	return &e, true
}

func (c *Cache) store(k Kind, key, etag string, body []byte) error {
	metaPath, bodyPath := paths(k, key)
	m, err := json.Marshal(entry{ETag: etag, SHA256: digest(body)})
	if err != nil {
		return err
	}
	// Body first: a crash between the two leaves a digest that does not
	// match, which load treats as a miss.
	if err := writeAtomic(c.root, bodyPath, body); err != nil {
		return fmt.Errorf("sparc: cache: %w", err)
	}
	if err := writeAtomic(c.root, metaPath, m); err != nil {
		return fmt.Errorf("sparc: cache: %w", err)
	}
	return nil
}

func digest(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// writeAtomic writes name inside root, owner-only, through a temporary file
// and a rename, so a reader never sees half a document.
func writeAtomic(root *os.Root, name string, data []byte) error {
	var rnd [8]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return err
	}
	tmp := ".tmp-" + hex.EncodeToString(rnd[:])
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(tmp) }()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return root.Rename(tmp, name)
}
