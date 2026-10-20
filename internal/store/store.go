// Package store persists download records in a versioned meta.json file.
package store

import (
	"cmp"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/jannes-dailidow/hrc-exporter/internal/domain"
)

// FileName is the conventional name of the metadata file.
const FileName = "meta.json"

// Version is the current meta.json schema version.
const Version = 1

var ErrUnsupportedVersion = errors.New("unsupported meta.json version")

type meta struct {
	Version   int              `json:"version"`
	Documents map[string]entry `json:"documents"`
}

type entry struct {
	ID           string    `json:"id"`
	Year         int       `json:"year"`
	Date         string    `json:"date"` // YYYY-MM-DD
	Type         string    `json:"type"`
	File         string    `json:"file"`
	SHA256       string    `json:"sha256"`
	Size         int64     `json:"size"`
	DownloadedAt time.Time `json:"downloaded_at"`
}

// JSONStore keeps records in memory and writes them to a JSON file.
type JSONStore struct {
	path string
	mu   sync.Mutex
	docs map[string]domain.Record
}

// Open loads the store at path. A missing file yields an empty store.
func Open(path string) (*JSONStore, error) {
	s := &JSONStore{path: path, docs: map[string]domain.Record{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var m meta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if m.Version < 1 || m.Version > Version {
		return nil, fmt.Errorf("%s: %w %d (supported: 1..%d)", path, ErrUnsupportedVersion, m.Version, Version)
	}
	for id, e := range m.Documents {
		r, err := e.record()
		if err != nil {
			return nil, fmt.Errorf("%s: document %s: %w", path, id, err)
		}
		s.docs[id] = r
	}
	return s, nil
}

// Path returns the file location.
func (s *JSONStore) Path() string { return s.path }

// Get returns the record for a document id.
func (s *JSONStore) Get(id string) (domain.Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.docs[id]
	return r, ok
}

// Put adds or replaces a record. Call Save to persist.
func (s *JSONStore) Put(r domain.Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docs[r.ID] = r
}

// Records returns all records sorted by date, then id.
func (s *JSONStore) Records() []domain.Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.SortedFunc(maps.Values(s.docs), func(a, b domain.Record) int {
		if c := a.Date.Compare(b.Date); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
}

// Save atomically writes the store to disk.
func (s *JSONStore) Save() error {
	s.mu.Lock()
	m := meta{Version: Version, Documents: make(map[string]entry, len(s.docs))}
	for id, r := range s.docs {
		m.Documents[id] = newEntry(r)
	}
	s.mu.Unlock()

	data, err := json.Marshal(m, json.Deterministic(true), jsontext.WithIndent("  "))
	if err != nil {
		return err
	}
	return writeFileAtomic(s.path, append(data, '\n'))
}

func newEntry(r domain.Record) entry {
	return entry{
		ID: r.ID, Year: r.Year, Date: r.Date.Format(time.DateOnly), Type: r.Type,
		File: r.File, SHA256: r.SHA256, Size: r.Size, DownloadedAt: r.DownloadedAt,
	}
}

func (e entry) record() (domain.Record, error) {
	date, err := time.Parse(time.DateOnly, e.Date)
	if err != nil {
		return domain.Record{}, fmt.Errorf("date: %w", err)
	}
	return domain.Record{
		ID: e.ID, Year: e.Year, Date: date, Type: e.Type,
		File: e.File, SHA256: e.SHA256, Size: e.Size, DownloadedAt: e.DownloadedAt,
	}, nil
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
