package store

import (
	"errors"
	"path/filepath"
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
	want := []Shelf{{Name: "openai", Upstream: "https://api.openai.com"}}
	if err := SaveShelves(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadShelves(path)
	if err != nil || len(got) != 1 || got[0] != want[0] {
		t.Fatalf("LoadShelves = %v, %v", got, err)
	}
}

func TestBadNames(t *testing.T) {
	keyring.MockInit()
	for _, name := range []string{"", "A", "../x", "a b", "a/b"} {
		if err := SaveKey(name, "k"); err != ErrBadName {
			t.Errorf("SaveKey(%q) = %v", name, err)
		}
		if err := SaveShelves(filepath.Join(t.TempDir(), "s.json"), []Shelf{{Name: name}}); err != ErrBadName {
			t.Errorf("SaveShelves(%q) = %v", name, err)
		}
	}
}
