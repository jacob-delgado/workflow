// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package keychain_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/synctest"
	"time"

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

// exitedWith is the error proc.Capture returns for a security run that
// exited with code.
func exitedWith(code int) error {
	return &proc.ExitError{
		Program: securityProgram, Code: code, Stderr: "the specified item could not be found in the keychain",
	}
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

	item := itemFor(t, recordingRunner(&runs, nil))

	// Act
	err := item.Store(t.Context(), `{"refresh_token":"xoxe-1-secret"}`)

	// Assert
	if err != nil || len(runs) != 2 {
		t.Fatalf("Store = %v after %d runs, want the store and its read back, and no error", err, len(runs))
	}

	if args := strings.Join(runs[0].program.Args, " "); args != "-i" || strings.Contains(args, "xoxe") {
		t.Errorf("security ran with %q, want only -i, the secret kept off the arguments", runs[0].program.Args)
	}

	want := `add-generic-password -U -a "jacob" -s "workflow-slack" -w "{\"refresh_token\":\"xoxe-1-secret\"}"` + "\n"
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

	item := itemFor(t, answeringRunner(&runs, "", exitedWith(44)))

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

// What security prints as a read fails is told by its exit status alone, as a
// store's is.
func TestReadTellsAFailureWithoutWhatSecurityPrinted(t *testing.T) {
	t.Parallel()

	// Arrange
	var runs []ran

	printed := &proc.ExitError{Program: securityProgram, Code: 51, Stderr: "SECRET-VALUE nearby"}
	item := itemFor(t, answeringRunner(&runs, "", printed))

	// Act
	_, err := item.Read(t.Context())

	// Assert
	if !errors.Is(err, keychain.ErrNotRead) || strings.Contains(err.Error(), "SECRET-VALUE") {
		t.Errorf("Read = %v, want ErrNotRead told without what security printed", err)
	}
}

func TestReadRefusesWithoutAnAccountName(t *testing.T) {
	t.Parallel()

	// Arrange
	var runs []ran

	item, ok := keychain.Open("darwin", slackService, answeringRunner(&runs, "stored-secret\n", nil),
		failedLookup, userIs(""))
	if !ok {
		t.Fatal("Open on darwin gave no item")
	}

	// Act
	secret, err := item.Read(t.Context())

	// Assert
	if !errors.Is(err, keychain.ErrNoAccount) || secret != "" {
		t.Errorf("Read = %q, %v; want %v and no secret", secret, err, keychain.ErrNoAccount)
	}

	if len(runs) != 0 {
		t.Errorf("Read ran %d programs without an account, want none", len(runs))
	}
}

// A store or a read may be asked under a context that never ends, one detached
// from a caller who has left, so each security run carries its own bound: one
// waiting on a locked keychain's unlock dialog cannot hold what waits on it
// without end.
func TestItemBoundsEachSecurityRunByTheDefaultRunTimeout(t *testing.T) {
	t.Parallel()

	cases := map[string]func(context.Context, keychain.Item) error{
		"a store": func(ctx context.Context, item keychain.Item) error {
			return item.Store(ctx, "s3cret")
		},
		"a read": func(ctx context.Context, item keychain.Item) error {
			_, err := item.Read(ctx)

			return err
		},
	}

	for name, use := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var (
				deadline time.Time
				bounded  bool
			)

			// The store's read back, and the read, find the secret stored.
			item := itemFor(t, func(ctx context.Context, _ proc.Command, _ []byte) ([]byte, error) {
				deadline, bounded = ctx.Deadline()

				return []byte("s3cret\n"), nil
			})
			before := time.Now()

			// Act
			err := use(context.WithoutCancel(t.Context()), item)
			after := time.Now()

			// Assert
			if err != nil || !bounded {
				t.Fatalf("%s = %v, with security given a deadline %t; want it run under one", name, err, bounded)
			}

			if deadline.Before(before.Add(proc.DefaultRunTimeout)) || deadline.After(after.Add(proc.DefaultRunTimeout)) {
				t.Errorf("security's deadline is %s after %s began, want %s",
					deadline.Sub(before), name, proc.DefaultRunTimeout)
			}
		})
	}
}

// A security run past the item's own bound answers to proc.ErrTimedOut, as one
// proc.Run stops at its bound does, so the words for a program that did not
// answer in time are the ones shown; a run its caller's own deadline ended
// first does not. The runner reports a run its context ended as proc.Capture
// does, by that context's cause.
func TestOnlyASecurityRunPastTheItemsBoundIsTimedOut(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		callerDeadline time.Duration
		wantTimedOut   bool
	}{
		"the item's bound ends it":          {callerDeadline: 2 * proc.DefaultRunTimeout, wantTimedOut: true},
		"the caller's own deadline ends it": {callerDeadline: proc.DefaultRunTimeout / 2, wantTimedOut: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				// Arrange
				item := itemFor(t, func(ctx context.Context, program proc.Command, _ []byte) ([]byte, error) {
					<-ctx.Done()

					return nil, fmt.Errorf("%s: %w", program.Name, context.Cause(ctx))
				})

				ctx, cancel := context.WithTimeout(t.Context(), tt.callerDeadline)
				defer cancel()

				// Act
				_, err := item.Read(ctx)

				// Assert
				if errors.Is(err, proc.ErrTimedOut) != tt.wantTimedOut {
					t.Errorf("Read = %v; answering to proc.ErrTimedOut is %t, want %t",
						err, errors.Is(err, proc.ErrTimedOut), tt.wantTimedOut)
				}
			})
		})
	}
}

func TestAStoreThatSecurityAcceptsButDoesNotKeepIsNotKept(t *testing.T) {
	t.Parallel()

	// Arrange
	// security exits 0 for the store, yet the item still holds what it held.
	runner := func(_ context.Context, program proc.Command, _ []byte) ([]byte, error) {
		if program.Args[0] == "find-generic-password" {
			return []byte("old-secret\n"), nil
		}

		return nil, nil
	}
	item := itemFor(t, runner)

	// Act
	err := item.Store(t.Context(), "new-secret")

	// Assert
	if !errors.Is(err, keychain.ErrNotKept) || strings.Contains(err.Error(), "new-secret") ||
		strings.Contains(err.Error(), "old-secret") {
		t.Errorf("Store = %v, want %v naming neither secret", err, keychain.ErrNotKept)
	}
}

func TestAStoreSecurityCannotReadBackIsNotKept(t *testing.T) {
	t.Parallel()

	// Arrange
	// security exits 0 for the store, then cannot read the item back.
	runner := func(_ context.Context, program proc.Command, _ []byte) ([]byte, error) {
		if program.Args[0] == "find-generic-password" {
			return nil, exitedWith(51)
		}

		return nil, nil
	}
	item := itemFor(t, runner)

	// Act
	err := item.Store(t.Context(), "new-secret")

	// Assert
	if !errors.Is(err, keychain.ErrNotKept) || errors.Is(err, keychain.ErrNotRead) ||
		!strings.Contains(err.Error(), "status 51") || strings.Contains(err.Error(), "new-secret") {
		t.Errorf("Store = %v, want %v alone, told by the read back's exit status and naming no secret",
			err, keychain.ErrNotKept)
	}
}

func TestUnderIsTheItemForAnotherServiceAsTheSameUser(t *testing.T) {
	t.Parallel()

	// Arrange
	var runs []ran

	item := itemFor(t, answeringRunner(&runs, "probe-secret\n", nil))

	// Act
	_, err := item.Under("workflow-slack-probe").Read(t.Context())

	// Assert
	if got := strings.Join(runs[0].program.Args, " "); err != nil ||
		got != "find-generic-password -a jacob -s workflow-slack-probe -w" {
		t.Errorf("security ran with %q, %v; want the other service read for the same user", got, err)
	}
}

func TestDeleteRemovesTheServicesSecret(t *testing.T) {
	t.Parallel()

	// Arrange
	var runs []ran

	item := itemFor(t, recordingRunner(&runs, nil))

	err := item.Store(t.Context(), "probe-secret")
	if err != nil {
		t.Fatalf("Store = %v", err)
	}

	// Act
	err = item.Delete(t.Context())
	// Assert
	if err != nil {
		t.Fatalf("Delete = %v, want the secret deleted", err)
	}

	got := strings.Join(runs[len(runs)-1].program.Args, " ")
	if got != "delete-generic-password -a jacob -s workflow-slack" {
		t.Errorf("security ran with %q, want the service's secret deleted for its user", got)
	}

	_, err = item.Read(t.Context())
	if !errors.Is(err, keychain.ErrNotStored) {
		t.Errorf("Read after Delete = %v, want %v", err, keychain.ErrNotStored)
	}
}

func TestDeletingWhatIsNotThereIsDone(t *testing.T) {
	t.Parallel()

	// Arrange
	var runs []ran

	item := itemFor(t, answeringRunner(&runs, "", exitedWith(44)))

	// Act
	err := item.Delete(t.Context())

	// Assert
	if err != nil || len(runs) != 1 {
		t.Errorf("Delete = %v after %d runs, want one run and nothing to delete taken as done", err, len(runs))
	}
}

func TestADeleteSecurityRefusesIsNotDeleted(t *testing.T) {
	t.Parallel()

	cases := map[string]error{
		"an exit status":    &proc.ExitError{Program: securityProgram, Code: 51, Stderr: "SECRET-VALUE nearby"},
		"a run that failed": errKeychainLocked,
	}

	for name, refusal := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var runs []ran

			item := itemFor(t, answeringRunner(&runs, "", refusal))

			// Act
			err := item.Delete(t.Context())

			// Assert
			if !errors.Is(err, keychain.ErrNotDeleted) || strings.Contains(err.Error(), "SECRET-VALUE") {
				t.Errorf("Delete = %v, want %v told without what security printed", err, keychain.ErrNotDeleted)
			}
		})
	}
}

func TestDeleteRefusesWithoutAnAccountName(t *testing.T) {
	t.Parallel()

	// Arrange
	var runs []ran

	item, ok := keychain.Open("darwin", slackService, answeringRunner(&runs, "", nil), failedLookup, userIs(""))
	if !ok {
		t.Fatal("Open on darwin gave no item")
	}

	// Act
	err := item.Delete(t.Context())

	// Assert
	if !errors.Is(err, keychain.ErrNoAccount) || len(runs) != 0 {
		t.Errorf("Delete = %v after %d runs, want %v and nothing run", err, len(runs), keychain.ErrNoAccount)
	}
}
