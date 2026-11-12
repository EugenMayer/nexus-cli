package keychain

import (
	"errors"
	"slices"
	"strings"
)

// The OS keyrings can't enumerate their entries, so a program that stores several secrets of the
// same kind (e.g. one token per organization) keeps their names in a list entry. A list is stored
// like a secret, as sorted, newline-separated names, but holds no secret itself.

// List returns the names stored in the list at path, sorted. A missing list is empty.
func (k *Keychain) List(path string) ([]string, error) {
	v, err := k.Get(path)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseList(v), nil
}

// AddToList adds name to the list at path, if it isn't in it yet.
func (k *Keychain) AddToList(path, name string) error {
	names, err := k.List(path)
	if err != nil {
		return err
	}
	if slices.Contains(names, name) {
		return nil
	}
	return k.Set(path, strings.Join(parseList(strings.Join(append(names, name), "\n")), "\n"))
}

// RemoveFromList removes name from the list at path. An empty list is deleted.
func (k *Keychain) RemoveFromList(path, name string) error {
	names, err := k.List(path)
	if err != nil {
		return err
	}
	names = slices.DeleteFunc(names, func(n string) bool { return n == name })
	if len(names) == 0 {
		_, err := k.Delete(path)
		return err
	}
	return k.Set(path, strings.Join(names, "\n"))
}

func parseList(v string) []string {
	var names []string
	for _, n := range strings.Split(v, "\n") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}
