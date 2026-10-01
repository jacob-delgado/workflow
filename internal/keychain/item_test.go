// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package keychain_test

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/keychain"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// slackService is a service other than the Jira token's.
const slackService = "workflow-slack"

// answeringRunner is a Runner that records what it is asked to run and answers
// with output and err.
func answeringRunner(runs *[]ran, output string, err error) keychain.Runner {
	return func(_ context.Context, program proc.Command, input []byte) ([]byte, error) {
		*runs = append(*runs, ran{program: program, input: string(input)})

		return []byte(output), err
	}
}

// exitedWith is the error a program that exited with code leaves, shaped as
// proc.Capture wraps it.
func exitedWith(t *testing.T, code int) error {
	t.Helper()

	err := exec.CommandContext(t.Context(), "sh", "-c", fmt.Sprintf("exit %d", code)).Run()
	if err == nil {
		t.Fatalf("sh exited 0, want %d", code)
	}

	return fmt.Errorf("security: %w: %s", err, "the specified item could not be found in the keychain")
}

// itemFor is the macOS keychain item for the Slack service, for a user logged in
// as jacob, run through runner.
func itemFor(t *testing.T, runner keychain.Runner) keychain.Item {
	t.Helper()

	item, ok := keychain.Open("darwin", slackService, runner, loggedInAs("jacob"), userIs(""))
	if !ok {
		t.Fatal("Open on darwin gave no item")
	}

	return item
}

func TestOpenGivesAnItemOnMacOSOnly(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{"darwin": true, "linux": false, "windows": false}

	for goos, want := range cases {
		t.Run(goos, func(t *testing.T) {
			t.Parallel()

			// Act
			_, ok := keychain.Open(goos, slackService, nil, loggedInAs("jacob"), userIs(""))

			// Assert
			if ok != want {
				t.Errorf("Open(%q) = %t, want %t", goos, ok, want)
			}
		})
	}
}

func TestStoreHandsTheSecretToSecurityOnItsInputUnderItsService(t *testing.T) {
	t.Parallel()

	// Arrange
	var runs []ran

	item := itemFor(t, answeringRunner(&runs, "", nil))

	// Act
	err := item.Store(t.Context(), `{"refresh_token":"xoxe-1-secret"}`)

	// Assert
	if err != nil || len(runs) != 1 {
		t.Fatalf("Store = %v after %d runs, want one run and no error", err, len(runs))
	}

	if args := strings.Join(runs[0].program.Args, " "); args != "-i" || strings.Contains(args, "xoxe") {
		t.Errorf("security ran with %q, want only -i, the secret kept off the arguments", runs[0].program.Args)
	}

	want := `add-generic-password -U -a "jacob" -s workflow-slack -w "{\"refresh_token\":\"xoxe-1-secret\"}"` + "\n"
	if runs[0].input != want {
		t.Errorf("security read %q, want %q", runs[0].input, want)
	}
}

func TestReadAsksSecurityForTheServicesSecret(t *testing.T) {
	t.Parallel()

	// Arrange
	var runs []ran

	item := itemFor(t, answeringRunner(&runs, "stored-secret\n", nil))

	// Act
	secret, err := item.Read(t.Context())

	// Assert
	if err != nil || secret != "stored-secret" {
		t.Errorf("Read = %q, %v; want the stored secret without its newline", secret, err)
	}

	if got := strings.Join(runs[0].program.Args, " "); got != "find-generic-password -a jacob -s workflow-slack -w" {
		t.Errorf("security ran with %q, want the service and account looked up", got)
	}
}

func TestReadSaysWhenNothingIsStored(t *testing.T) {
	t.Parallel()

	// Arrange
	var runs []ran

	item := itemFor(t, answeringRunner(&runs, "", exitedWith(t, 44)))

	// Act
	_, err := item.Read(t.Context())

	// Assert
	if !errors.Is(err, keychain.ErrNotStored) {
		t.Errorf("Read = %v, want %v for security's item-not-found", err, keychain.ErrNotStored)
	}
}

func TestReadReportsAnyOtherFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	var runs []ran

	item := itemFor(t, answeringRunner(&runs, "", errKeychainLocked))

	// Act
	_, err := item.Read(t.Context())

	// Assert
	if !errors.Is(err, errKeychainLocked) || errors.Is(err, keychain.ErrNotStored) {
		t.Errorf("Read = %v, want security's own failure", err)
	}
}
