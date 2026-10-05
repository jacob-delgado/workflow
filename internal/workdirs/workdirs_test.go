// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package workdirs_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// realDir is the directory a case links to, and anaAPI where a case is.
const (
	realDir = "real"
	anaAPI  = "/home/ana/src/api"
)

// tree makes dirs and files under a new directory, which it returns, every
// path written relative to it.
func tree(t *testing.T, dirs []string, files []string) string {
	t.Helper()

	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	for _, dir := range dirs {
		err = os.MkdirAll(filepath.Join(root, dir), 0o750)
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, file := range files {
		err = os.WriteFile(filepath.Join(root, file), nil, 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	return root
}

// link makes name in root a link to target, also in root.
func link(t *testing.T, root, target, name string) {
	t.Helper()

	err := os.Symlink(filepath.Join(root, target), filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
}

// names are the names of a listing's entries, in its order.
func names(listing workdirs.Listing) []string {
	listed := make([]string, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		listed = append(listed, entry.Name)
	}

	return listed
}

func TestAListingIsTheDirectoriesInItByName(t *testing.T) {
	t.Parallel()

	// Arrange
	// A file is not somewhere to work, and a hidden directory is reached by
	// typing its name rather than listed with the rest.
	root := tree(t, []string{"zeta", "api", "Docs", ".cache"}, []string{"notes.md"})

	// Act
	listing, err := workdirs.List(root, "")

	// Assert
	if want := []string{"api", "Docs", "zeta"}; err != nil || !slices.Equal(names(listing), want) {
		t.Errorf("List = %v, %v; want %v", names(listing), err, want)
	}
}

func TestAListingSaysWhichDirectoriesAreRepositories(t *testing.T) {
	t.Parallel()

	// Arrange
	// A worktree or a submodule has a .git file rather than a directory.
	root := tree(t, []string{"api/.git", "linked", "plain"}, []string{"linked/.git"})

	// Act
	listing, err := workdirs.List(root, "")

	// Assert
	repositories := map[string]bool{}
	for _, entry := range listing.Entries {
		repositories[entry.Name] = entry.Repository
	}

	if err != nil || !repositories["api"] || !repositories["linked"] || repositories["plain"] {
		t.Errorf("List = %+v, %v; want api and linked marked repositories, plain not", listing.Entries, err)
	}
}

func TestADirectoryLinkedToIsListedLikeOneInIt(t *testing.T) {
	t.Parallel()

	// Arrange
	root := tree(t, []string{realDir}, []string{"file"})
	link(t, root, realDir, "link")
	link(t, root, "file", "filelink")

	// Act
	listing, err := workdirs.List(root, "")

	// Assert
	if want := []string{"link", realDir}; err != nil || !slices.Equal(names(listing), want) {
		t.Errorf("List = %v, %v; want %v", names(listing), err, want)
	}
}

func TestAListingStopsAtItsLimitAndSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	dirs := make([]string, workdirs.ListLimit+1)
	for index := range dirs {
		dirs[index] = "d" + strconv.Itoa(index)
	}

	root := tree(t, dirs, nil)

	// Act
	listing, err := workdirs.List(root, "")

	// Assert
	if err != nil || len(listing.Entries) != workdirs.ListLimit || !listing.Truncated {
		t.Errorf("List = %d entries (more: %v), %v; want %d and that there were more",
			len(listing.Entries), listing.Truncated, err, workdirs.ListLimit)
	}
}

func TestADirectoryIsCheckedBeforeItIsUsed(t *testing.T) {
	t.Parallel()

	root := tree(t, []string{"here"}, []string{"file"})
	cases := map[string]struct {
		dir  string
		want error
	}{
		"a directory":     {dir: filepath.Join(root, "here"), want: nil},
		"nothing there":   {dir: filepath.Join(root, "gone"), want: workdirs.ErrNotFound},
		"a file":          {dir: filepath.Join(root, "file"), want: workdirs.ErrNotADirectory},
		"a relative path": {dir: "here", want: workdirs.ErrNotAbsolute},
	}

	for name, check := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := workdirs.Check(check.dir)

			// Assert
			if !errors.Is(err, check.want) || (check.want == nil && err != nil) {
				t.Errorf("Check(%q) = %v, want %v", check.dir, err, check.want)
			}
		})
	}
}

func TestATypedPathIsReadFromWhereYouAreAndYourHome(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ typed, want string }{
		"absolute":        {typed: "/srv/api", want: "/srv/api"},
		"relative":        {typed: "../web", want: "/home/ana/src/web"},
		"your home":       {typed: "~", want: "/home/ana"},
		"under your home": {typed: "~/notes/", want: "/home/ana/notes"},
		"blank":           {typed: "  ", want: anaAPI},
	}

	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := workdirs.Resolve(path.typed, anaAPI, "/home/ana"); got != path.want {
				t.Errorf("Resolve(%q) = %q, want %q", path.typed, got, path.want)
			}
		})
	}
}

func TestADirectoryUnderYourHomeIsShownFromIt(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ home, want string }{
		"under it":           {home: "/home/ana", want: "~/src/api"},
		"no home":            {home: "", want: anaAPI},
		"a name it prefixes": {home: "/home/an", want: anaAPI},
	}

	for name, shown := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := workdirs.Shown(anaAPI, shown.home); got != shown.want {
				t.Errorf("Shown = %q, want %q", got, shown.want)
			}
		})
	}
}

func TestHomeItselfIsShownAsATilde(t *testing.T) {
	t.Parallel()

	// Act & Assert
	if got := workdirs.Shown("/home/ana", "/home/ana"); got != "~" {
		t.Errorf("Shown = %q, want ~", got)
	}
}

func TestTwoNamesForOneDirectoryAreTheSame(t *testing.T) {
	t.Parallel()

	root := tree(t, []string{realDir, "other"}, nil)
	link(t, root, realDir, "link")

	cases := map[string]struct {
		one, other string
		want       bool
	}{
		"a link and where it leads": {one: "link", other: realDir, want: true},
		"two directories":           {one: "other", other: realDir, want: false},
		"nothing there":             {one: "gone", other: "gone", want: false},
	}

	for name, pair := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := workdirs.Same(filepath.Join(root, pair.one), filepath.Join(root, pair.other)); got != pair.want {
				t.Errorf("Same(%s, %s) = %v, want %v", pair.one, pair.other, got, pair.want)
			}
		})
	}
}

func TestAListingOfThoseStartingWithAPrefixReachesPastTheLimit(t *testing.T) {
	t.Parallel()

	// Arrange
	// Tab completes from the directories that start as typed, so a name past
	// the first thousand is still found.
	dirs := make([]string, 0, workdirs.ListLimit+4)
	for index := range workdirs.ListLimit + 1 {
		dirs = append(dirs, "d"+strconv.Itoa(index))
	}

	root := tree(t, append(dirs, "zebra", "Zulu", "alpha"), nil)

	// Act
	listing, err := workdirs.List(root, "z")

	// Assert
	if want := []string{"zebra"}; err != nil || !slices.Equal(names(listing), want) || listing.Truncated {
		t.Errorf("List = %v (more: %v), %v; want %v alone", names(listing), listing.Truncated, err, want)
	}
}

func TestAListingPastTheLimitKeepsTheFirstByName(t *testing.T) {
	t.Parallel()

	// Arrange
	// Read in byte order, every capitalized name comes first; the listing is
	// the first thousand by name whatever their case.
	dirs := make([]string, 0, workdirs.ListLimit+1)
	for index := range workdirs.ListLimit {
		dirs = append(dirs, "B"+strconv.Itoa(index))
	}

	root := tree(t, append(dirs, "a"), nil)

	// Act
	listing, err := workdirs.List(root, "")

	// Assert
	if err != nil || len(listing.Entries) != workdirs.ListLimit || listing.Entries[0].Name != "a" {
		t.Errorf("List starts %v, %v; want a first", names(listing)[:1], err)
	}
}

func TestAHiddenDirectoryIsListedOnceItsDotIsTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	root := tree(t, []string{".config", "configs"}, nil)

	// Act
	listing, err := workdirs.List(root, ".c")

	// Assert
	if want := []string{".config"}; err != nil || !slices.Equal(names(listing), want) {
		t.Errorf("List = %v, %v; want %v", names(listing), err, want)
	}
}

func TestAFileIsNotListed(t *testing.T) {
	t.Parallel()

	// Arrange
	root := tree(t, nil, []string{"notes.md"})

	// Act
	_, err := workdirs.List(filepath.Join(root, "notes.md"), "")

	// Assert
	if !errors.Is(err, workdirs.ErrNotADirectory) {
		t.Errorf("List = %v, want ErrNotADirectory", err)
	}
}

func TestADirectoryOutsideYourHomeIsShownAsItIs(t *testing.T) {
	t.Parallel()

	// Act & Assert
	if got := workdirs.Shown("/opt/tools", "/home/ana"); got != "/opt/tools" {
		t.Errorf("Shown = %q, want /opt/tools", got)
	}
}
