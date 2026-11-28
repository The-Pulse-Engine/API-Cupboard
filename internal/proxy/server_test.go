package proxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

// start runs h behind the Host check on a free port and returns the port.
func start(t *testing.T, h http.Handler) int {
	t.Helper()
	l, err := Listen(0)
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(l, h)
	go srv.Serve(l)
	t.Cleanup(func() { srv.Close() })
	return l.Addr().(*net.TCPAddr).Port
}

// get sends GET path to the server on port with the given Host header.
func get(t *testing.T, port int, host, path string) (int, string) {
	t.Helper()
	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d%s", port, path), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = host
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestListenOnlyOnLoopback(t *testing.T) {
	l, err := Listen(0)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if ip := l.Addr().(*net.TCPAddr).IP; !ip.Equal(net.IPv4(127, 0, 0, 1)) {
		t.Fatalf("listening on %v, want 127.0.0.1", ip)
	}
}

func TestListenPortInUse(t *testing.T) {
	l, err := Listen(0)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	_, err = Listen(l.Addr().(*net.TCPAddr).Port)
	if err == nil || !strings.Contains(err.Error(), "another cupboard serve") {
		t.Fatalf("err = %v", err)
	}
}

func TestHostCheck(t *testing.T) {
	port := start(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }))
	p := fmt.Sprint(port)
	for host, want := range map[string]int{
		"127.0.0.1:" + p:    200,
		"localhost:" + p:    200,
		"LOCALHOST:" + p:    200,
		"evil.example:" + p: 421,
		"127.0.0.1":         421, // no port
		"localhost":         421,
		"127.0.0.1:1":       421, // another port
		"[::1]:" + p:        421,
	} {
		if code, _ := get(t, port, host, "/anything"); code != want {
			t.Errorf("Host %q: status %d, want %d", host, code, want)
		}
	}
}
