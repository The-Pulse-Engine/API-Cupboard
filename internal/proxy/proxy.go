package proxy

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"

	"github.com/The-Pulse-Engine/API-Cupboard/internal/store"
)

// Handler routes /<shelf>/<path> to the shelf's upstream URL. load is called
// for each request, so a change from cupboard add applies without a restart.
// transport is nil except in tests.
func Handler(load func() ([]store.Shelf, error), transport http.RoundTripper) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Split the escaped path, so /open%2fai/x is the shelf "open%2fai", not "open".
		name, rest, slash := strings.Cut(strings.TrimPrefix(r.URL.EscapedPath(), "/"), "/")
		if !store.ValidName(name) {
			http.Error(w, "cupboard: no such shelf", http.StatusNotFound)
			return
		}
		if slash {
			rest = "/" + rest
		}
		restPath, err := url.PathUnescape(rest)
		if err != nil || hasDotSegment(restPath) {
			http.Error(w, "cupboard: the path has . or .. segments", http.StatusBadRequest)
			return
		}

		shelves, err := load()
		if err != nil {
			http.Error(w, "cupboard: "+err.Error(), http.StatusInternalServerError)
			return
		}
		i := slices.IndexFunc(shelves, func(s store.Shelf) bool { return s.Name == name })
		if i < 0 {
			http.Error(w, fmt.Sprintf("cupboard: no shelf %q, run cupboard add %s", name, name), http.StatusNotFound)
			return
		}
		target, err := url.Parse(shelves[i].Upstream)
		// LoadShelves checks this too, but never send a request in clear text (security requirement 7).
		if err != nil || target.Scheme != "https" || target.Hostname() == "" {
			http.Error(w, fmt.Sprintf("cupboard: shelf %q has no https upstream", name), http.StatusBadGateway)
			return
		}

		rp := &httputil.ReverseProxy{
			// Rewrite, not Director: after Director, Go deletes headers that the client
			// names in Connection, and that would include an injected key header.
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.SetURL(target) // scheme, host, query, and the upstream Host header
				// SetURL would add a / to "/openai", so join the base path and the rest here.
				// Path and RawPath must match, or Go drops RawPath and %2F becomes /.
				pr.Out.URL.Path = strings.TrimSuffix(target.Path, "/") + restPath
				pr.Out.URL.RawPath = strings.TrimSuffix(target.EscapedPath(), "/") + rest
			},
			Transport: transport,
			// The default handler logs the full upstream URL, which can hold a key once
			// query injection lands (security requirement 8). Keep the cause, drop the URL.
			ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
				var ue *url.Error
				if errors.As(err, &ue) {
					err = ue.Err
				}
				http.Error(w, fmt.Sprintf("cupboard: shelf %q: the upstream request failed: %v", name, err), http.StatusBadGateway)
			},
		}
		rp.ServeHTTP(w, r)
	})
}

func hasDotSegment(path string) bool {
	return slices.ContainsFunc(strings.Split(path, "/"), func(s string) bool { return s == "." || s == ".." })
}
