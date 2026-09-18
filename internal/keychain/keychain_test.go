// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package keychain_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/keychain"
)

func TestSupportedIsWiredForMacOS(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{"darwin": true, "linux": false, "windows": false}

	for goos, want := range cases {
		t.Run(goos, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := keychain.Supported(goos); got != want {
				t.Errorf("Supported(%q) = %t, want %t", goos, got, want)
			}
		})
	}
}

func TestStoreCommandSavesUnderTheServiceUpdatably(t *testing.T) {
	t.Parallel()

	// Act
	cmd := keychain.StoreCommand("workflow-jira", "s3cret")

	// Assert
	want := []string{"add-generic-password", "-U", "-s", "workflow-jira", "-w", "s3cret"}
	if cmd.Name != "security" || !slices.Equal(cmd.Args, want) {
		t.Errorf("StoreCommand = %s %q, want security %q", cmd.Name, cmd.Args, want)
	}
}

func TestLookupCommandReadsBackByService(t *testing.T) {
	t.Parallel()

	// Act
	got := keychain.LookupCommand("workflow-jira")

	// Assert
	if got != "security find-generic-password -s workflow-jira -w" {
		t.Errorf("LookupCommand = %q, want the find-generic-password reader", got)
	}
}
