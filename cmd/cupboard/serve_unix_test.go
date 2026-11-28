//go:build !windows

package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// startServe runs cupboard serve on a free port until the test sends a signal.
// It returns the port, the exit code channel and serve's stdout.
func startServe(t *testing.T) (int, chan int, *syncBuffer) {
	t.Helper()
	setup(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	out := &syncBuffer{}
	done := make(chan int, 1)
	go func() {
		done <- cli([]string{"serve", "--port", fmt.Sprint(port)}, strings.NewReader(""), out, io.Discard)
	}()
	// Wait until the port answers; serve registered for signals before it opened the port.
	for start := time.Now(); ; time.Sleep(20 * time.Millisecond) {
		if c, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port)); err == nil {
			c.Close()
			break
		}
		if time.Since(start) > 5*time.Second {
			t.Fatal("serve did not open the port")
		}
	}
	return port, done, out
}

func stopServe(t *testing.T, done chan int) {
	t.Helper()
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("code %d, want 0", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not stop")
	}
}

func TestServeStopsCleanlyOnSignal(t *testing.T) {
	_, done, out := startServe(t)
	stopServe(t, done)
	if !strings.Contains(out.String(), "proxy stopped") {
		t.Fatalf("stdout %q", out.String())
	}
}

// A request that is still open when serve stops must not turn the stop into an error.
func TestServeClosesOpenRequestsOnStop(t *testing.T) {
	old := shutdownWait
	shutdownWait = 200 * time.Millisecond
	t.Cleanup(func() { shutdownWait = old })

	port, done, out := startServe(t)
	c, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	// Half a request: the server waits for the rest, so the connection stays active.
	fmt.Fprintf(c, "GET /x HTTP/1.1\r\nHost: 127.0.0.1:%d\r\n", port)
	time.Sleep(100 * time.Millisecond)
	stopServe(t, done)
	if !strings.Contains(out.String(), "closed requests that were still open") {
		t.Fatalf("stdout %q", out.String())
	}
}

// serve must route to the shelves in the real shelves.json, not only start and stop.
func TestServeUsesShelvesJSON(t *testing.T) {
	port, done, _ := startServe(t)
	defer stopServe(t, done)
	// A closed port: a 502 from the upstream request proves that serve found the shelf.
	if _, stderr, code := runCLI(t, "k", "add", "closed", "--stdin", "--upstream", "https://127.0.0.1:1"); code != 0 {
		t.Fatalf("add: code %d, stderr %q", code, stderr)
	}
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/closed/x", port))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 502 || !strings.Contains(string(body), "the upstream request failed") {
		t.Fatalf("status %d, body %q", resp.StatusCode, body)
	}
}

// syncBuffer is a bytes.Buffer that serve's goroutine and the test can share.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
