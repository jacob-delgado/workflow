// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package keychain_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/keychain"
)

// errKeychainLocked stands in for security refusing to store.
var errKeychainLocked = errors.New("security: exit status 51")

// ran is one program a fake Runner was asked to run.
type ran struct {
	name string
	args []string
}

// recordingRunner is a Runner that records what it is asked to run and answers
// with err.
func recordingRunner(runs *[]ran, err error) keychain.Runner {
	return func(_ context.Context, name string, args ...string) ([]byte, error) {
		*runs = append(*runs, ran{name: name, args: args})

		return nil, err
	}
}

func TestStorerIsWiredForMacOSOnly(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{"darwin": true, "linux": false, "windows": false}

	for goos, wired := range cases {
		t.Run(goos, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var runs []ran

			// Act
			store := keychain.Storer(goos, recordingRunner(&runs, nil))

			// Assert
			if got := store != nil; got != wired {
				t.Errorf("Storer(%q) wired = %t, want %t", goos, got, wired)
			}
		})
	}
}

func TestStorerSavesUnderTheJiraServiceUpdatablyAndNamesTheReader(t *testing.T) {
	t.Parallel()

	// Arrange
	var runs []ran

	store := keychain.Storer("darwin", recordingRunner(&runs, nil))

	// Act
	tokenCommand, err := store("s3cret")

	// Assert
	if err != nil || tokenCommand != "security find-generic-password -s workflow-jira -w" {
		t.Errorf("store = %q, %v; want the find-generic-password reader", tokenCommand, err)
	}

	want := []string{"add-generic-password", "-U", "-s", "workflow-jira", "-w", "s3cret"}
	if len(runs) != 1 || runs[0].name != "security" || !slices.Equal(runs[0].args, want) {
		t.Errorf("store ran %+v, want security %q once", runs, want)
	}
}

func TestStorerReportsARefusedStore(t *testing.T) {
	t.Parallel()

	// Arrange
	var runs []ran

	store := keychain.Storer("darwin", recordingRunner(&runs, errKeychainLocked))

	// Act
	tokenCommand, err := store("s3cret")

	// Assert
	if !errors.Is(err, errKeychainLocked) || tokenCommand != "" {
		t.Errorf("store = %q, %v; want no token_command and security's error", tokenCommand, err)
	}
}
