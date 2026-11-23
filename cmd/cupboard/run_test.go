package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/The-Pulse-Engine/API-Cupboard/internal/store"
)

const helperSrc = `package main

import (
	"fmt"
	"os"
	"strconv"
)

// Prints the env var named by the first argument and exits with the second.
func main() {
	fmt.Print(os.Getenv(os.Args[1]))
	code, _ := strconv.Atoi(os.Args[2])
	os.Exit(code)
}
`

// buildHelper compiles helperSrc and returns the path of the program.
func buildHelper(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "helper.go")
	if err := os.WriteFile(src, []byte(helperSrc), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "helper")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", bin, src).CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v\n%s", err, out)
	}
	return bin
}

// setupProject writes a cupboard.toml, moves into it and stores the key.
func setupProject(t *testing.T) {
	t.Helper()
	setup(t)
	dir := t.TempDir()
	toml := "[env]\nCUPBOARD_TEST_KEY = \"test\"\n"
	if err := os.WriteFile(filepath.Join(dir, "cupboard.toml"), []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	if err := store.SaveKey("test", "sk-run"); err != nil {
		t.Fatal(err)
	}
}

func TestRun(t *testing.T) {
	setupProject(t)
	stdout, stderr, code := runCLI(t, "", "run", "--", buildHelper(t), "CUPBOARD_TEST_KEY", "3")
	if code != 3 || stdout != "sk-run" {
		t.Fatalf("code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if strings.Contains(stderr, "sk-run") {
		t.Fatalf("stderr shows the key: %q", stderr)
	}
}

func TestRunCmdFile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("npm is a .cmd file only on Windows")
	}
	setupProject(t)
	script := filepath.Join(t.TempDir(), "helper.cmd")
	body := "@echo off\r\necho %CUPBOARD_TEST_KEY%\r\nexit /b 4\r\n"
	if err := os.WriteFile(script, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runCLI(t, "", "run", "--", script)
	if code != 4 || strings.TrimSpace(stdout) != "sk-run" {
		t.Fatalf("code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestRunUsage(t *testing.T) {
	for _, args := range [][]string{{"run"}, {"run", "--"}, {"run", "node", "app.js"}} {
		if _, stderr, code := runCLI(t, "", args...); code != 2 || !strings.Contains(stderr, "usage") {
			t.Errorf("%v: code %d, stderr %q", args, code, stderr)
		}
	}
}

func TestRunMissingKey(t *testing.T) {
	setupProject(t)
	store.DeleteKey("test")
	_, stderr, code := runCLI(t, "", "run", "--", "go", "version")
	if code != 1 || !strings.Contains(stderr, `CUPBOARD_TEST_KEY: shelf "test" has no key`) {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}
