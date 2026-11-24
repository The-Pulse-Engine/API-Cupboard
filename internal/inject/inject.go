// Package inject reads cupboard.toml and builds the environment for cupboard run.
package inject

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/BurntSushi/toml"
	"github.com/The-Pulse-Engine/API-Cupboard/internal/store"
	"github.com/zalando/go-keyring"
)

const FileName = "cupboard.toml"

var (
	ErrNoConfig = errors.New("no " + FileName + " in this directory or any parent")
	validEnv    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// Find returns the path of cupboard.toml in dir or the nearest parent.
func Find(dir string) (string, error) {
	for {
		path := filepath.Join(dir, FileName)
		_, err := os.Stat(path)
		if err == nil {
			return path, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNoConfig
		}
		dir = parent
	}
}

// Load returns the [env] table: env var name to shelf name.
func Load(path string) (map[string]string, error) {
	var cfg struct {
		Env map[string]string `toml:"env"`
	}
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return nil, err
	}
	if extra := md.Undecoded(); len(extra) > 0 {
		return nil, fmt.Errorf("%s: unknown key %q, only [env] is allowed", path, extra[0].String())
	}
	if len(cfg.Env) == 0 {
		return nil, fmt.Errorf("%s: [env] has no entries, so no keys would be passed", path)
	}
	for name, shelf := range cfg.Env {
		if !validEnv.MatchString(name) {
			return nil, fmt.Errorf("%s: %q is not a valid env var name", path, name)
		}
		if !store.ValidName(shelf) {
			return nil, fmt.Errorf("%s: %s: %w", path, name, store.ErrBadName)
		}
	}
	return cfg.Env, nil
}

// Environ returns the current environment plus one variable per entry, with the key from the keychain.
// Errors name the env var and the shelf, never the key.
func Environ(vars map[string]string) ([]string, error) {
	env := os.Environ()
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		shelf := vars[name]
		key, err := store.ReadKey(shelf)
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, fmt.Errorf("%s: shelf %q has no key, run cupboard add %s", name, shelf, shelf)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: shelf %q: %w", name, shelf, err)
		}
		// Later entries win in exec.Cmd, so this overrides any value from the shell.
		env = append(env, name+"="+key)
	}
	return env, nil
}
