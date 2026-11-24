package store

import (
	"errors"
	"regexp"

	"github.com/zalando/go-keyring"
)

const service = "api-cupboard"

var (
	ErrBadName = errors.New("shelf name must use only a-z, 0-9 and -, and must not start with -")
	// A leading - would make the name look like a flag on the command line.
	validName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

func ValidName(name string) bool { return validName.MatchString(name) }

func SaveKey(shelf, key string) error {
	if !ValidName(shelf) {
		return ErrBadName
	}
	return keyring.Set(service, shelf, key)
}

func ReadKey(shelf string) (string, error) {
	if !ValidName(shelf) {
		return "", ErrBadName
	}
	return keyring.Get(service, shelf)
}

func DeleteKey(shelf string) error {
	if !ValidName(shelf) {
		return ErrBadName
	}
	return keyring.Delete(service, shelf)
}
