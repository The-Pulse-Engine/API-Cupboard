package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

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
	for _, name := range []string{"", "A", "../x", "a b", "a/b", "-dev", "-"} {
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

func shelvesNamed(prefix string, n int) []Shelf {
	var s []Shelf
	for i := 0; i < n; i++ {
		s = append(s, Shelf{Name: fmt.Sprintf("%s-%d", prefix, i), Upstream: "https://example.com/" + strings.Repeat("a", 50), Inject: "bearer"})
	}
	return s
}

// Two saves at once used to share one temp file and corrupt shelves.json.
func TestConcurrentSavesNeverCorrupt(t *testing.T) {
	for round := 0; round < 50; round++ {
		path := filepath.Join(t.TempDir(), "shelves.json")
		var wg sync.WaitGroup
		for _, n := range []int{1, 6} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				SaveShelves(path, shelvesNamed("s", n)) // a rename can lose the race on Windows; that is fine
			}()
		}
		wg.Wait()
		got, err := LoadShelves(path)
		if err != nil || (len(got) != 1 && len(got) != 6) {
			t.Fatalf("round %d: %d shelves, %v", round, len(got), err)
		}
	}
}

// With the lock, concurrent read-change-write cycles keep every shelf.
func TestLockKeepsEveryShelf(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new-dir", "shelves.json")
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock, err := LockShelves(path)
			if err != nil {
				errs <- err
				return
			}
			defer unlock()
			shelves, err := LoadShelves(path)
			if err == nil {
				err = SaveShelves(path, append(shelves, shelvesNamed(fmt.Sprint("w", i), 1)...))
			}
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if got, err := LoadShelves(path); err != nil || len(got) != 20 {
		t.Fatalf("%d shelves, %v", len(got), err)
	}
	if _, err := os.Stat(path + ".lock"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("lock file left behind: %v", err)
	}
}

// A lock left by a command that died must not block later commands.
func TestLockTakesOverStaleLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shelves.json")
	lock := path + ".lock"
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * staleLock)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	unlock, err := LockShelves(path)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("took %v to take a stale lock", time.Since(start))
	}
	unlock()
	if _, err := os.Stat(lock); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("lock file left behind: %v", err)
	}
}

// On Windows, a reader holding shelves.json open made a save fail with "Access is denied".
func TestSaveWhileAnotherProcessReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shelves.json")
	s := shelvesNamed("s", 1)
	if err := SaveShelves(path, s); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				LoadShelves(path)
			}
		}
	}()
	for i := 0; i < 200; i++ {
		if err := SaveShelves(path, s); err != nil {
			close(stop)
			<-done
			t.Fatalf("save %d: %v", i, err)
		}
	}
	close(stop)
	<-done
}
