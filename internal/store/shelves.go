package store

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type Shelf struct {
	Name     string `json:"name"`
	Upstream string `json:"upstream"`
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
	err = json.Unmarshal(data, &shelves)
	return shelves, err
}

func SaveShelves(path string, shelves []Shelf) error {
	for _, s := range shelves {
		if !ValidName(s.Name) {
			return ErrBadName
		}
	}
	data, err := json.MarshalIndent(shelves, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// Write then rename, so a crash never leaves a half-written file.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
