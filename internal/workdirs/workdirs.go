// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package workdirs reads the directories workflow can be pointed at: what is in
// one, whether it is there, and how a typed path or a directory is written. It
// lists names and asks whether a name is a directory; it never reads a file.
package workdirs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ListLimit bounds how many directories a listing holds, so a directory of
// thousands answers with what fits on a screen and says there was more.
const ListLimit = 1000

var (
	// ErrNotFound is a directory that is not there.
	ErrNotFound = errors.New("there is no such directory")
	// ErrNotADirectory is a path that names something other than a directory.
	ErrNotADirectory = errors.New("that is not a directory")
	// ErrNotAbsolute is a path not written from the root.
	ErrNotAbsolute = errors.New("the path is not absolute")
	// ErrUnreadable is a directory that is there but cannot be listed.
	ErrUnreadable = errors.New("the directory cannot be read")
)

// Entry is a directory in another: its name, and whether it is a repository's
// root.
type Entry struct {
	Name       string
	Repository bool
}

// Listing is the directories in one, by name, and whether there were more than
// ListLimit.
type Listing struct {
	Entries   []Entry
	Truncated bool
}

// List is the directories in dir whose names start with prefix — all of
// them for "" — a directory linked to among them, sorted by name whatever
// its case, the first ListLimit kept. A hidden one is left out unless prefix
// starts with a dot: it is reached by typing its name.
func List(dir, prefix string) (Listing, error) {
	err := Check(dir)
	if err != nil {
		return Listing{}, err
	}

	found, err := os.ReadDir(dir)
	if err != nil {
		return Listing{}, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}

	var listing Listing

	for _, item := range found {
		if offered(item.Name(), prefix) && directoryEntry(dir, item) {
			listing.Entries = append(listing.Entries, Entry{Name: item.Name(), Repository: false})
		}
	}

	slices.SortFunc(listing.Entries, func(one, other Entry) int {
		return strings.Compare(strings.ToLower(one.Name), strings.ToLower(other.Name))
	})

	if len(listing.Entries) > ListLimit {
		listing.Entries, listing.Truncated = listing.Entries[:ListLimit], true
	}

	// Asked only of the entries kept, so a directory of thousands costs a
	// question for each one shown, not for each one there.
	for index, entry := range listing.Entries {
		listing.Entries[index].Repository = exists(filepath.Join(dir, entry.Name, ".git"))
	}

	return listing, nil
}

// offered reports a name a listing for prefix offers: one starting with it,
// and not hidden unless prefix starts with the dot.
func offered(name, prefix string) bool {
	return strings.HasPrefix(name, prefix) && (!strings.HasPrefix(name, ".") || strings.HasPrefix(prefix, "."))
}

// directoryEntry reports an entry of dir a directory or a link to one. Only a
// link is looked up: what the listing read already says what anything else is.
func directoryEntry(dir string, item fs.DirEntry) bool {
	if item.Type()&fs.ModeSymlink != 0 {
		return isDirectory(filepath.Join(dir, item.Name()))
	}

	return item.IsDir()
}

// Check reports dir an absolute path to a directory that is there.
func Check(dir string) error {
	if !filepath.IsAbs(dir) {
		return ErrNotAbsolute
	}

	info, err := os.Stat(dir)

	switch {
	case errors.Is(err, fs.ErrNotExist):
		return ErrNotFound
	case err != nil:
		return fmt.Errorf("%w: %w", ErrUnreadable, err)
	case !info.IsDir():
		return ErrNotADirectory
	}

	return nil
}

// Resolve is the directory typed names: from the root as written, from your
// home after a ~, and from base otherwise; blank, it is base. A ~ is followed
// by either separator, so ~/src reads the same on Windows.
func Resolve(typed, base, home string) string {
	typed = strings.TrimSpace(typed)

	switch {
	case typed == "~":
		return filepath.Clean(home)
	case strings.HasPrefix(typed, "~/") || strings.HasPrefix(typed, "~"+string(filepath.Separator)):
		return filepath.Join(home, typed[2:])
	case filepath.IsAbs(typed):
		return filepath.Clean(typed)
	}

	return filepath.Join(base, typed)
}

// Shown is dir written from your home, ~/src/api, when it is under it, and as
// it is otherwise. The part under your home is written with forward slashes,
// as ~ itself is a Unix spelling.
func Shown(dir, home string) string {
	if home == "" {
		return dir
	}

	within, err := filepath.Rel(home, dir)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		return dir
	}

	if within == "." {
		return "~"
	}

	return "~/" + filepath.ToSlash(within)
}

// Same reports one and other the same directory however each is written, a
// link and where it leads included; a path that is not there is never the
// same as anything.
func Same(one, other string) bool {
	oneInfo, err := os.Stat(one)
	if err != nil {
		return false
	}

	otherInfo, err := os.Stat(other)
	if err != nil {
		return false
	}

	return os.SameFile(oneInfo, otherInfo)
}

// isDirectory reports path a directory, following a link to one.
func isDirectory(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}

// exists reports something at path, whatever it is.
func exists(path string) bool {
	_, err := os.Lstat(path)

	return err == nil
}
