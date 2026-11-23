package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/The-Pulse-Engine/API-Cupboard/internal/store"
	"github.com/zalando/go-keyring"
)

// setup points the keychain and shelves.json at test-only places.
func setup(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func shelves(t *testing.T) []store.Shelf {
	t.Helper()
	_, s, err := loadShelves()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func wantKey(t *testing.T, shelf, want string) {
	t.Helper()
	if got, err := store.ReadKey(shelf); err != nil || got != want {
		t.Fatalf("ReadKey(%q) = %q, %v", shelf, got, err)
	}
}

func TestAddPrompts(t *testing.T) {
	setup(t)
	stdout, stderr, code := runCLI(t, "https://api.openai.com\nsk-secret\n", "add", "openai")
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	if strings.Contains(stdout+stderr, "sk-secret") {
		t.Fatal("output shows the key")
	}
	wantKey(t, "openai", "sk-secret")
	want := []store.Shelf{{Name: "openai", Upstream: "https://api.openai.com", Inject: "bearer"}}
	if got := shelves(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("shelves = %v", got)
	}
}

func TestAddStdinAndFlags(t *testing.T) {
	setup(t)
	// Flags both before and after the shelf name.
	_, stderr, code := runCLI(t, "sk-g\r\n", "add", "--inject", "query", "gemini",
		"--query", "key", "--stdin", "--upstream", "https://example.com/v1")
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	wantKey(t, "gemini", "sk-g")
	want := []store.Shelf{{Name: "gemini", Upstream: "https://example.com/v1", Inject: "query", Query: "key"}}
	if got := shelves(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("shelves = %v", got)
	}
}

func TestAddRefuses(t *testing.T) {
	cases := map[string]struct {
		stdin string
		args  []string
	}{
		"http upstream":     {"http://api.openai.com\nk\n", []string{"add", "openai"}},
		"bad name":          {"https://x.com\nk\n", []string{"add", "Open/AI"}},
		"header no name":    {"https://x.com\nk\n", []string{"add", "a", "--inject", "header"}},
		"empty key":         {"https://x.com\n\n", []string{"add", "a"}},
		"stdin no upstream": {"k", []string{"add", "a", "--stdin"}},
	}
	for name, c := range cases {
		setup(t)
		if _, _, code := runCLI(t, c.stdin, c.args...); code != 1 {
			t.Errorf("%s: code %d", name, code)
		}
		if got := shelves(t); got != nil {
			t.Errorf("%s: shelves = %v", name, got)
		}
	}
}

func TestAddExisting(t *testing.T) {
	setup(t)
	runCLI(t, "old", "add", "a", "--stdin", "--upstream", "https://x.com")
	if _, stderr, code := runCLI(t, "new", "add", "a", "--stdin", "--upstream", "https://y.com"); code != 1 || !strings.Contains(stderr, "--force") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	wantKey(t, "a", "old")
	if _, _, code := runCLI(t, "new", "add", "a", "--stdin", "--force", "--upstream", "https://y.com"); code != 0 {
		t.Fatalf("--force: code %d", code)
	}
	wantKey(t, "a", "new")
	if got := shelves(t); len(got) != 1 || got[0].Upstream != "https://y.com" {
		t.Fatalf("shelves = %v", got)
	}
}

func TestAddKeychainFails(t *testing.T) {
	setup(t)
	keyring.MockInitWithError(errors.New("keychain locked"))
	if _, _, code := runCLI(t, "k", "add", "a", "--stdin", "--upstream", "https://x.com"); code != 1 {
		t.Fatalf("code %d", code)
	}
	if got := shelves(t); got != nil {
		t.Fatalf("shelves.json changed: %v", got)
	}
}
