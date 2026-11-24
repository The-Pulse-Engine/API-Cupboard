package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestKeys(t *testing.T) {
	keyring.MockInit()
	if err := SaveKey("openai", "sk-test"); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadKey("openai"); err != nil || got != "sk-test" {
		t.Fatalf("ReadKey = %q, %v", got, err)
	}
	if err := DeleteKey("openai"); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadKey("openai"); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestShelves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "shelves.json")
	if got, err := LoadShelves(path); err != nil || got != nil {
		t.Fatalf("missing file: %v, %v", got, err)
	}
	want := []Shelf{
		{Name: "openai", Upstream: "https://api.openai.com", Inject: "bearer", AllowOrigins: []string{"http://localhost:3000"}},
		{Name: "basic", Upstream: "https://example.com", Inject: "basic"},
		{Name: "anthropic", Upstream: "https://api.anthropic.com", Inject: "header", Header: "x-api-key"},
		{Name: "gemini", Upstream: "https://example.com/v1", Inject: "query", Query: "key"},
	}
	if err := SaveShelves(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadShelves(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadShelves = %v, %v", got, err)
	}
}

func TestOldFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shelves.json")
	old := `[{"name": "openai", "upstream": "https://api.openai.com"}]`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadShelves(path)
	want := []Shelf{{Name: "openai", Upstream: "https://api.openai.com", Inject: "bearer"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadShelves = %v, %v", got, err)
	}
}

func TestRejectedShelves(t *testing.T) {
	ok := Shelf{Name: "a", Upstream: "https://example.com", Inject: "bearer"}
	with := func(f func(*Shelf)) []Shelf { s := ok; f(&s); return []Shelf{s} }
	cases := []struct {
		name    string
		shelves []Shelf
		want    error
	}{
		{"bad name", with(func(s *Shelf) { s.Name = "../x" }), ErrBadName},
		{"duplicate", []Shelf{ok, ok}, ErrDuplicate},
		{"http upstream", with(func(s *Shelf) { s.Upstream = "http://example.com" }), ErrBadUpstream},
		{"no host", with(func(s *Shelf) { s.Upstream = "https://" }), ErrBadUpstream},
		{"empty upstream", with(func(s *Shelf) { s.Upstream = "" }), ErrBadUpstream},
		{"port only", with(func(s *Shelf) { s.Upstream = "https://:443" }), ErrBadUpstream},
		{"user in upstream", with(func(s *Shelf) { s.Upstream = "https://user:pass@example.com" }), ErrBadUpstream},
		{"unknown inject", with(func(s *Shelf) { s.Inject = "cookie" }), ErrBadInject},
		{"header without name", with(func(s *Shelf) { s.Inject = "header" }), ErrBadInject},
		{"header with line break", with(func(s *Shelf) { s.Inject, s.Header = "header", "x-key\r\nX-Evil: 1" }), ErrBadInject},
		{"header with space", with(func(s *Shelf) { s.Inject, s.Header = "header", "x key" }), ErrBadInject},
		{"query without name", with(func(s *Shelf) { s.Inject = "query" }), ErrBadInject},
	}
	for _, c := range cases {
		dir := t.TempDir()
		if err := SaveShelves(filepath.Join(dir, "save.json"), c.shelves); !errors.Is(err, c.want) {
			t.Errorf("%s: SaveShelves = %v", c.name, err)
		}
		// Write the file by hand, as a user could, and load it.
		path := filepath.Join(dir, "load.json")
		writeJSON(t, path, c.shelves)
		if _, err := LoadShelves(path); !errors.Is(err, c.want) {
			t.Errorf("%s: LoadShelves = %v", c.name, err)
		}
	}
}

func TestLoadBadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shelves.json")
	if err := os.WriteFile(path, []byte("[{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadShelves(path); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("err = %v, want the file path in it", err)
	}
}

func TestBadNames(t *testing.T) {
	keyring.MockInit()
	for _, name := range []string{"", "A", "../x", "a b", "a/b"} {
		if err := SaveKey(name, "k"); err != ErrBadName {
			t.Errorf("SaveKey(%q) = %v", name, err)
		}
		s := Shelf{Name: name, Upstream: "https://example.com", Inject: "bearer"}
		if err := SaveShelves(filepath.Join(t.TempDir(), "s.json"), []Shelf{s}); !errors.Is(err, ErrBadName) {
			t.Errorf("SaveShelves(%q) = %v", name, err)
		}
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
