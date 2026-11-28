package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/The-Pulse-Engine/API-Cupboard/internal/store"
)

// seen is what the test upstream received.
type seen struct {
	method, uri, host, body string
}

// upstream starts a TLS test server that records each request.
func upstream(t *testing.T) (*httptest.Server, *[]seen) {
	t.Helper()
	var got []seen
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = append(got, seen{r.Method, r.URL.RequestURI(), r.Host, string(body)})
		io.WriteString(w, "from upstream")
	}))
	t.Cleanup(ts.Close)
	return ts, &got
}

func shelvesFor(ts *httptest.Server) func() ([]store.Shelf, error) {
	return func() ([]store.Shelf, error) {
		return []store.Shelf{
			{Name: "openai", Upstream: ts.URL + "/v1", Inject: "bearer"},
			{Name: "root", Upstream: ts.URL, Inject: "bearer"},
		}, nil
	}
}

func send(h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, strings.NewReader(body)))
	return rec
}

func TestRoutesToUpstream(t *testing.T) {
	ts, got := upstream(t)
	h := Handler(shelvesFor(ts), ts.Client().Transport)
	host := strings.TrimPrefix(ts.URL, "https://")
	cases := []struct{ method, target, body, wantURI string }{
		{"POST", "/openai/chat?limit=2&q=a%20b", `{"x":1}`, "/v1/chat?limit=2&q=a%20b"},
		{"GET", "/openai/models", "", "/v1/models"},
		{"GET", "/openai", "", "/v1"},
		{"GET", "/openai/", "", "/v1/"},
		{"GET", "/openai/a%2Fb", "", "/v1/a%2Fb"}, // an encoded slash stays encoded
		{"GET", "/root/x", "", "/x"},
	}
	for _, c := range cases {
		rec := send(h, c.method, c.target, c.body)
		if rec.Code != 200 || rec.Body.String() != "from upstream" {
			t.Fatalf("%s: status %d, body %q", c.target, rec.Code, rec.Body)
		}
		last := (*got)[len(*got)-1]
		want := seen{c.method, c.wantURI, host, c.body}
		if last != want {
			t.Errorf("%s: upstream got %+v, want %+v", c.target, last, want)
		}
	}
}

func TestRefusesBadPaths(t *testing.T) {
	ts, got := upstream(t)
	h := Handler(shelvesFor(ts), ts.Client().Transport)
	for target, want := range map[string]int{
		"/unknown/x":             404,
		"/OpenAI/x":              404,
		"/%6fpenai/x":            404, // an encoded shelf name
		"/open%2fai/x":           404,
		"/%2e%2e/x":              404,
		"/":                      404,
		"/-x/y":                  404,
		"/openai/..%2f..%2fx":    400,
		"/openai/../root/x":      400,
		"/openai/./x":            400,
		"/openai/a/%2e%2e/b":     400,
		"/openai/..%2f%2e%2e%2f": 400,
	} {
		if rec := send(h, "GET", target, ""); rec.Code != want {
			t.Errorf("%s: status %d, want %d", target, rec.Code, want)
		}
	}
	if len(*got) != 0 {
		t.Fatalf("upstream got %d requests, want 0: %+v", len(*got), *got)
	}
}

// A hand-edited shelves.json with an http:// upstream must never be used.
func TestNeverForwardsInClearText(t *testing.T) {
	var hits atomic.Int32
	plain := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer plain.Close()

	path := filepath.Join(t.TempDir(), "shelves.json")
	raw := `[{"name": "openai", "upstream": "` + plain.URL + `", "inject": "bearer"}]`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	fromFile := Handler(func() ([]store.Shelf, error) { return store.LoadShelves(path) }, nil)
	if rec := send(fromFile, "GET", "/openai/x", ""); rec.Code != 500 {
		t.Errorf("from shelves.json: status %d, want 500", rec.Code)
	}
	// The check in the handler itself, for a loader that skipped validation.
	unchecked := Handler(func() ([]store.Shelf, error) {
		return []store.Shelf{{Name: "openai", Upstream: plain.URL, Inject: "bearer"}}, nil
	}, nil)
	if rec := send(unchecked, "GET", "/openai/x", ""); rec.Code != 502 {
		t.Errorf("unchecked: status %d, want 502", rec.Code)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("the http upstream got %d requests", n)
	}
}

func TestUpstreamDownHidesTheURL(t *testing.T) {
	ts, _ := upstream(t)
	h := Handler(func() ([]store.Shelf, error) {
		return []store.Shelf{{Name: "openai", Upstream: ts.URL + "/v1?key=sk-secret", Inject: "bearer"}}, nil
	}, ts.Client().Transport)
	ts.Close()
	rec := send(h, "GET", "/openai/x", "")
	body := rec.Body.String()
	// The cause (a refused connection) is shown, so the user can tell what went wrong.
	if rec.Code != 502 || !strings.Contains(body, `shelf "openai"`) || !strings.Contains(body, "dial tcp") {
		t.Fatalf("status %d, body %q", rec.Code, body)
	}
	if strings.Contains(body, "sk-secret") || strings.Contains(body, "/v1") {
		t.Fatalf("the error shows the upstream URL: %q", body)
	}
}
