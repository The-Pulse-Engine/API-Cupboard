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

func TestAddNeedsConsole(t *testing.T) {
	setup(t)
	// Test stdin is a pipe, as in Git Bash, so add must not prompt for the key.
	stdout, stderr, code := runCLI(t, "https://api.openai.com\nsk-secret\n", "add", "openai")
	if code != 1 || !strings.Contains(stderr, "--stdin") || strings.Contains(stderr, "Key:") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	if strings.Contains(stdout+stderr, "sk-secret") {
		t.Fatal("output shows the key")
	}
	if _, err := store.ReadKey("openai"); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("key saved: %v", err)
	}
	if got := shelves(t); got != nil {
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
		"http upstream":     {"k", []string{"add", "openai", "--stdin", "--upstream", "http://api.openai.com"}},
		"bad name":          {"k", []string{"add", "Open/AI", "--stdin", "--upstream", "https://x.com"}},
		"header no name":    {"k", []string{"add", "a", "--stdin", "--upstream", "https://x.com", "--inject", "header"}},
		"empty key":         {"\r\n", []string{"add", "a", "--stdin", "--upstream", "https://x.com"}},
		"stdin no upstream": {"k", []string{"add", "a", "--stdin"}},
		"two-line key":      {"line1\nline2\n", []string{"add", "a", "--stdin", "--upstream", "https://x.com"}},
		"key with NUL":      {"sk\x00x", []string{"add", "a", "--stdin", "--upstream", "https://x.com"}},
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

func TestList(t *testing.T) {
	setup(t)
	if stdout, _, code := runCLI(t, "", "list"); code != 0 || !strings.Contains(stdout, "cupboard add") {
		t.Fatalf("empty: code %d, stdout %q", code, stdout)
	}
	runCLI(t, "sk-one", "add", "openai", "--stdin", "--upstream", "https://api.openai.com")
	runCLI(t, "sk-two", "add", "anthropic", "--stdin", "--upstream", "https://api.anthropic.com",
		"--inject", "header", "--header", "x-api-key")
	stdout, _, code := runCLI(t, "", "list")
	if code != 0 {
		t.Fatalf("code %d", code)
	}
	for _, want := range []string{"openai", "https://api.openai.com", "bearer", "anthropic", "header x-api-key"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("list misses %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "sk-") {
		t.Errorf("list shows a key:\n%s", stdout)
	}
}

func TestRemove(t *testing.T) {
	setup(t)
	runCLI(t, "k1", "add", "a", "--stdin", "--upstream", "https://x.com")
	runCLI(t, "k2", "add", "b", "--stdin", "--upstream", "https://y.com")
	if _, stderr, code := runCLI(t, "", "remove", "a"); code != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	if _, err := store.ReadKey("a"); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("key still there: %v", err)
	}
	if got := shelves(t); len(got) != 1 || got[0].Name != "b" {
		t.Fatalf("shelves = %v", got)
	}
	wantKey(t, "b", "k2")
}

func TestRemoveMissing(t *testing.T) {
	setup(t)
	if _, stderr, code := runCLI(t, "", "remove", "nope"); code != 1 || !strings.Contains(stderr, `"nope" does not exist`) {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	// The key is gone from the keychain but the shelf is still listed.
	runCLI(t, "k", "add", "a", "--stdin", "--upstream", "https://x.com")
	store.DeleteKey("a")
	if _, stderr, code := runCLI(t, "", "remove", "a"); code != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	if got := shelves(t); len(got) != 0 {
		t.Fatalf("shelves = %v", got)
	}
}

// failWith makes the named hook fail for one test.
func failWith[T any](t *testing.T, hook *T, fake T) {
	old := *hook
	*hook = fake
	t.Cleanup(func() { *hook = old })
}

func TestAddShelvesWriteFails(t *testing.T) {
	setup(t)
	runCLI(t, "old", "add", "b", "--stdin", "--upstream", "https://x.com")
	failWith(t, &saveShelves, func(string, []store.Shelf) error { return errors.New("disk full") })

	if _, _, code := runCLI(t, "k", "add", "a", "--stdin", "--upstream", "https://x.com"); code != 1 {
		t.Fatalf("new shelf: code %d", code)
	}
	if _, err := store.ReadKey("a"); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("new shelf: key left in keychain: %v", err)
	}
	if _, _, code := runCLI(t, "new", "add", "b", "--stdin", "--force", "--upstream", "https://y.com"); code != 1 {
		t.Fatalf("replace: code %d", code)
	}
	wantKey(t, "b", "old")
}

func TestAddForceStopsWhenOldKeyUnreadable(t *testing.T) {
	setup(t)
	runCLI(t, "old", "add", "b", "--stdin", "--upstream", "https://x.com")
	failWith(t, &readOldKey, func(string) (string, error) { return "", errors.New("keychain locked") })
	if _, stderr, code := runCLI(t, "new", "add", "b", "--stdin", "--force", "--upstream", "https://y.com"); code != 1 || !strings.Contains(stderr, "keychain locked") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	wantKey(t, "b", "old")
	if got := shelves(t); got[0].Upstream != "https://x.com" {
		t.Fatalf("shelves = %v", got)
	}
}

func TestAddKeyInUpstream(t *testing.T) {
	setup(t)
	_, stderr, code := runCLI(t, "AIzaSecret123", "add", "gemini", "--stdin", "--upstream", "https://example.com/v1?key=AIzaSecret123")
	if code != 1 || !strings.Contains(stderr, "contains the key") || strings.Contains(stderr, "AIzaSecret123") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	if got := shelves(t); got != nil {
		t.Fatalf("shelves = %v", got)
	}
}

func TestRemoveKeepsShelfWhenKeychainFails(t *testing.T) {
	setup(t)
	runCLI(t, "k", "add", "a", "--stdin", "--upstream", "https://x.com")
	keyring.MockInitWithError(errors.New("keychain locked"))
	if _, stderr, code := runCLI(t, "", "remove", "a"); code != 1 || !strings.Contains(stderr, "keychain locked") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	if got := shelves(t); len(got) != 1 {
		t.Fatalf("shelf removed although its key was not: %v", got)
	}
}
