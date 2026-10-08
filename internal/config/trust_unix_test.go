// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package config_test

// A configuration file is taken only when no one but the user could have
// written it: it is the user's own, and neither others nor a group the user
// does not have to themselves may write it. Windows keeps no such bits, so
// these hold on Unix alone.

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// nobody is the user and group id of the account that owns nothing.
const nobody = 65534

// withMode is a configuration file holding `{}` at mode, made so whatever the
// umask.
func withMode(t *testing.T, mode os.FileMode) string {
	t.Helper()

	path := write(t, t.TempDir(), `{}`)

	err := os.Chmod(path, mode)
	if err != nil {
		t.Fatalf("setting the mode: %v", err)
	}

	return path
}

// ownedByAnotherUser is a configuration file the user running the test does
// not own: one it gives away when it can, being root, or else one the system
// keeps.
func ownedByAnotherUser(t *testing.T) string {
	t.Helper()

	if os.Geteuid() == 0 {
		path := write(t, t.TempDir(), `{}`)

		err := os.Chown(path, nobody, nobody)
		if err != nil {
			t.Fatalf("giving the file away: %v", err)
		}

		return path
	}

	path := "/etc/passwd"

	info, err := os.Stat(path)
	if err != nil {
		t.Skipf("no file another user owns: %v", err)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) == os.Geteuid() {
		t.Skip("no file another user owns")
	}

	return path
}

// inAnotherGroup is a configuration file whose group is not the user's own
// private one, where the test can make one: a group it belongs to whose id is
// not its user id, or any group, being root.
func inAnotherGroup(t *testing.T) string {
	t.Helper()

	path := withMode(t, 0o660)

	groups, err := os.Getgroups()
	if err != nil {
		t.Fatalf("listing the groups: %v", err)
	}

	other := slices.IndexFunc(groups, func(group int) bool { return group != os.Geteuid() })

	switch {
	case os.Geteuid() == 0:
		err = os.Chown(path, -1, nobody)
	case other >= 0:
		err = os.Chown(path, -1, groups[other])
	default:
		t.Skip("the user belongs to no group but its own")
	}

	if err != nil {
		t.Fatalf("handing the file to another group: %v", err)
	}

	return path
}

func TestAConfigurationFileOthersCanWriteIsRefused(t *testing.T) {
	t.Parallel()

	for name, mode := range map[string]os.FileMode{"others may write": 0o602, "anyone may write": 0o666} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			files := config.Files{Home: withMode(t, mode)}

			// Act
			_, _, err := config.LoadLayersAt(files)

			// Assert
			if !errors.Is(err, config.ErrUntrustedFile) {
				t.Errorf("LoadLayersAt of a %#o file = %v; want ErrUntrustedFile", mode, err)
			}
		})
	}
}

func TestARepositoryFileOthersCanWriteIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	files := config.Files{Home: withMode(t, 0o600), Repo: withMode(t, 0o646)}

	// Act
	_, _, err := config.LoadLayersAt(files)

	// Assert
	if !errors.Is(err, config.ErrUntrustedFile) {
		t.Errorf("LoadLayersAt = %v; want the repository's file refused with ErrUntrustedFile", err)
	}
}

func TestAConfigurationFileAnotherUserOwnsIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	files := config.Files{Home: ownedByAnotherUser(t)}

	// Act
	_, _, err := config.LoadLayersAt(files)

	// Assert
	if !errors.Is(err, config.ErrUntrustedFile) {
		t.Errorf("LoadLayersAt = %v; want a file another user owns refused with ErrUntrustedFile", err)
	}
}

func TestAConfigurationFileAnotherGroupCanWriteIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	files := config.Files{Home: inAnotherGroup(t)}

	// Act
	_, _, err := config.LoadLayersAt(files)

	// Assert
	if !errors.Is(err, config.ErrUntrustedFile) {
		t.Errorf("LoadLayersAt = %v; want a file another group may write refused with ErrUntrustedFile", err)
	}
}

func TestAConfigurationFileOnlyItsOwnerCanWriteIsTaken(t *testing.T) {
	t.Parallel()

	for name, mode := range map[string]os.FileMode{"private": 0o600, "readable by all": 0o644} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			files := config.Files{Home: withMode(t, mode)}

			// Act
			_, _, err := config.LoadLayersAt(files)
			// Assert
			if err != nil {
				t.Errorf("LoadLayersAt of a %#o file = %v; want it taken", mode, err)
			}
		})
	}
}

func TestALinkedHomeFileIsJudgedByTheFileItNames(t *testing.T) {
	t.Parallel()

	// Arrange
	target := withMode(t, 0o666)
	link := filepath.Join(t.TempDir(), config.FileName)

	err := os.Symlink(target, link)
	if err != nil {
		t.Fatalf("linking the file: %v", err)
	}

	// Act
	_, _, err = config.LoadLayersAt(config.Files{Home: link})

	// Assert
	if !errors.Is(err, config.ErrUntrustedFile) {
		t.Errorf("LoadLayersAt through a link = %v; want the file it names refused with ErrUntrustedFile", err)
	}
}

func TestAConfigurationFileItsOwnersOwnGroupCanWriteIsTaken(t *testing.T) {
	t.Parallel()

	// Arrange
	// A user's own group has the user's id, where the system gives each user
	// one; there, a umask of 002 leaves every file the user makes group
	// writable, and no one else is in the group.
	path := withMode(t, 0o660)

	err := os.Chown(path, -1, os.Geteuid())
	if err != nil {
		t.Skipf("the user has no group of its own: %v", err)
	}

	// Act
	_, _, err = config.LoadLayersAt(config.Files{Home: path})
	// Assert
	if err != nil {
		t.Errorf("LoadLayersAt = %v; want a file only its owner's own group may write taken", err)
	}
}
