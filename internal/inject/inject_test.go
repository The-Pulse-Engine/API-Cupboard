package inject

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/The-Pulse-Engine/API-Cupboard/internal/store"
	"github.com/zalando/go-keyring"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

const valid = `[env]
OPENAI_API_KEY = "openai"
STRIPE_SECRET_KEY = "stripe"
`

func TestFindAndLoad(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, FileName), valid)
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{root, deep} {
		path, err := Find(dir)
		if err != nil || path != filepath.Join(root, FileName) {
			t.Fatalf("Find(%s) = %q, %v", dir, path, err)
		}
	}
	got, err := Load(filepath.Join(root, FileName))
	want := map[string]string{"OPENAI_API_KEY": "openai", "STRIPE_SECRET_KEY": "stripe"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Load = %v, %v", got, err)
	}
}

func TestFindMissing(t *testing.T) {
	if _, err := Find(t.TempDir()); !errors.Is(err, ErrNoConfig) {
		t.Fatalf("Find = %v", err)
	}
}

func TestLoadRejects(t *testing.T) {
	for name, body := range map[string]string{
		"bad env name":   "[env]\n\"1BAD\" = \"openai\"\n",
		"env with dash":  "[env]\n\"MY-KEY\" = \"openai\"\n",
		"bad shelf name": "[env]\nKEY = \"../openai\"\n",
		"unknown table":  "[evn]\nKEY = \"openai\"\n",
		"not toml":       "[env\n",
		"empty env":      "[env]\n",
		"empty file":     "",
	} {
		path := filepath.Join(t.TempDir(), FileName)
		writeFile(t, path, body)
		if _, err := Load(path); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestEnviron(t *testing.T) {
	keyring.MockInit()
	store.SaveKey("openai", "sk-test")
	t.Setenv("OPENAI_API_KEY", "from-shell")
	env, err := Environ(map[string]string{"OPENAI_API_KEY": "openai"})
	if err != nil {
		t.Fatal(err)
	}
	// exec.Cmd uses the last value, so the key must come after the shell value.
	if last := env[len(env)-1]; last != "OPENAI_API_KEY=sk-test" {
		t.Fatalf("last entry = %q", last)
	}
	if len(env) != len(os.Environ())+1 {
		t.Fatalf("env has %d entries, want %d", len(env), len(os.Environ())+1)
	}
}

func TestEnvironMissingKey(t *testing.T) {
	keyring.MockInit()
	_, err := Environ(map[string]string{"OPENAI_API_KEY": "openai"})
	want := `OPENAI_API_KEY: shelf "openai" has no key, run cupboard add openai`
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v", err)
	}
	keyring.MockInitWithError(errors.New("keychain locked"))
	if _, err := Environ(map[string]string{"K": "openai"}); err == nil || !strings.Contains(err.Error(), "keychain locked") {
		t.Fatalf("err = %v", err)
	}
}
