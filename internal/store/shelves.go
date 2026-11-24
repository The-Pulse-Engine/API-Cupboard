package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
)

var (
	ErrBadUpstream = errors.New("upstream must be an https:// URL with a host and no user name or password")
	ErrBadInject   = errors.New("inject must be bearer, basic, header (with a valid header name) or query (with a query name)")
	ErrDuplicate   = errors.New("shelf name is used twice")

	// An HTTP header name is a token (RFC 9110, section 5.6.2).
	validHeader = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")
)

type Shelf struct {
	Name         string   `json:"name"`
	Upstream     string   `json:"upstream"`
	Inject       string   `json:"inject"`
	Header       string   `json:"header,omitempty"`
	Query        string   `json:"query,omitempty"`
	AllowOrigins []string `json:"allow_origins,omitempty"`
}

// Validate checks one shelf. Errors name the shelf, never a key.
func (s Shelf) Validate() error {
	if !ValidName(s.Name) {
		return fmt.Errorf("shelf %q: %w", s.Name, ErrBadName)
	}
	u, err := url.Parse(s.Upstream)
	// A user name or password in the URL would be a secret in a plaintext file.
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("shelf %q: %w", s.Name, ErrBadUpstream)
	}
	switch {
	case s.Inject == "bearer", s.Inject == "basic",
		s.Inject == "header" && validHeader.MatchString(s.Header),
		s.Inject == "query" && s.Query != "":
		return nil
	}
	return fmt.Errorf("shelf %q: %w", s.Name, ErrBadInject)
}

func validateAll(shelves []Shelf) error {
	seen := map[string]bool{}
	for _, s := range shelves {
		if err := s.Validate(); err != nil {
			return err
		}
		if seen[s.Name] {
			return fmt.Errorf("shelf %q: %w", s.Name, ErrDuplicate)
		}
		seen[s.Name] = true
	}
	return nil
}

// ShelvesPath is %APPDATA%\api-cupboard\shelves.json on Windows and ~/.config/api-cupboard/shelves.json on Linux.
func ShelvesPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "api-cupboard", "shelves.json"), nil
}

func LoadShelves(path string) ([]Shelf, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var shelves []Shelf
	if err := json.Unmarshal(data, &shelves); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	// Files from v0 have no inject field.
	for i := range shelves {
		if shelves[i].Inject == "" {
			shelves[i].Inject = "bearer"
		}
	}
	// Users can edit this file by hand, and the proxy routes on it.
	if err := validateAll(shelves); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return shelves, nil
}

func SaveShelves(path string, shelves []Shelf) error {
	if err := validateAll(shelves); err != nil {
		return err
	}
	data, err := json.MarshalIndent(shelves, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// Write, flush to disk, then rename, so a crash never leaves a half-written file.
	tmp := path + ".tmp"
	if err := writeSynced(tmp, data); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func writeSynced(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
