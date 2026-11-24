package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/The-Pulse-Engine/API-Cupboard/internal/store"
)

const helperSrc = `package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Prints the env var named by the first argument and exits with the second.
// "kill" kills itself, and "wait" prints a line and sleeps.
func main() {
	fmt.Print(os.Getenv(os.Args[1]))
	switch os.Args[2] {
	case "kill":
		p, _ := os.FindProcess(os.Getpid())
		p.Kill()
		time.Sleep(10 * time.Second)
	case "wait":
		fmt.Println(" ready")
		time.Sleep(10 * time.Second)
	}
	code, _ := strconv.Atoi(os.Args[2])
	os.Exit(code)
}
`

var helper struct {
	once     sync.Once
	dir, bin string
	err      error
}

func TestMain(m *testing.M) {
	code := m.Run()
	if helper.dir != "" {
		os.RemoveAll(helper.dir)
	}
	os.Exit(code)
}

// buildHelper compiles helperSrc once per test run and returns the path of the program.
func buildHelper(t *testing.T) string {
	t.Helper()
	helper.once.Do(func() {
		if helper.dir, helper.err = os.MkdirTemp("", "cupboard-helper-"); helper.err != nil {
			return
		}
		src := filepath.Join(helper.dir, "helper.go")
		if helper.err = os.WriteFile(src, []byte(helperSrc), 0o600); helper.err != nil {
			return
		}
		helper.bin = filepath.Join(helper.dir, "helper")
		if runtime.GOOS == "windows" {
			helper.bin += ".exe"
		}
		if out, err := exec.Command("go", "build", "-o", helper.bin, src).CombinedOutput(); err != nil {
			helper.err = fmt.Errorf("build helper: %v\n%s", err, out)
		}
	})
	if helper.err != nil {
		t.Fatal(helper.err)
	}
	return helper.bin
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
	// Test stderr is a buffer, not a console, so the "from shelf" note must stay out of piped output.
	if stderr != "" {
		t.Fatalf("stderr is not empty when it is not a console: %q", stderr)
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
