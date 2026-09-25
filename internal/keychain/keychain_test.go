// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package keychain_test

import (
	"context"
	"errors"
	"os/user"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/keychain"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// errKeychainLocked stands in for security refusing to store.
var errKeychainLocked = errors.New("security: exit status 51")

// errUnknownUser stands in for the OS failing to look up the current user.
var errUnknownUser = errors.New("user: unknown userid 501")

// ran is one program a fake Runner was asked to run, and what it was handed on
// its standard input.
type ran struct {
	program proc.Command
	input   string
}

// recordingRunner is a Runner that records what it is asked to run and answers
// with err.
func recordingRunner(runs *[]ran, err error) keychain.Runner {
	return func(_ context.Context, program proc.Command, input []byte) ([]byte, error) {
		*runs = append(*runs, ran{program: program, input: string(input)})

		return nil, err
	}
}

// loggedInAs is a user lookup that finds a user whose login name is name.
func loggedInAs(name string) keychain.UserLookup {
	return func() (*user.User, error) {
		return &user.User{Username: name}, nil
	}
}

// failedLookup is a user lookup the OS cannot answer.
func failedLookup() (*user.User, error) {
	return nil, errUnknownUser
}

// userIs is a getenv in which $USER is name and nothing else is set.
func userIs(name string) func(string) string {
	return func(key string) string {
		if key == "USER" {
			return name
		}

		return ""
	}
}

// storerFor is the macOS store for a user logged in as jacob, running
// security through a recordingRunner that answers with err.
func storerFor(runs *[]ran, err error) func(secret string) (string, error) {
	return keychain.Storer("darwin", recordingRunner(runs, err), loggedInAs("jacob"), userIs(""))
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
			store := keychain.Storer(goos, recordingRunner(&runs, nil), loggedInAs("jacob"), userIs(""))

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

	store := storerFor(&runs, nil)

	// Act
	tokenCommand, err := store("s3cret")

	// Assert
	if err != nil || tokenCommand != "security find-generic-password -s workflow-jira -w" {
		t.Errorf("store = %q, %v; want the find-generic-password reader", tokenCommand, err)
	}

	want := ran{
		program: proc.Command{Name: "security", Args: []string{"-i"}},
		input:   `add-generic-password -U -a "jacob" -s workflow-jira -w "s3cret"` + "\n",
	}
	if len(runs) != 1 || runs[0].program.Name != want.program.Name ||
		!slices.Equal(runs[0].program.Args, want.program.Args) || runs[0].input != want.input {
		t.Errorf("store ran %+v, want %+v once", runs, want)
	}
}

func TestStorerKeepsTheSecretOutOfTheArguments(t *testing.T) {
	t.Parallel()

	// Arrange
	var runs []ran

	store := storerFor(&runs, nil)

	// Act
	_, err := store("s3cret")

	// Assert
	if err != nil || len(runs) != 1 {
		t.Fatalf("store ran %d programs, %v; want one", len(runs), err)
	}

	for _, arg := range append([]string{runs[0].program.Name}, runs[0].program.Args...) {
		if strings.Contains(arg, "s3cret") {
			t.Errorf("the secret is in the command's arguments: %q", runs[0].program.Args)
		}
	}

	if !strings.Contains(runs[0].input, "s3cret") {
		t.Errorf("the secret is not on the command's input: %q", runs[0].input)
	}
}

func TestStorerQuotesTheSecretForSecurity(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		plain, quoted string
	}{
		"spaces":         {plain: "two words", quoted: `"two words"`},
		"a double quote": {plain: `say "hi"`, quoted: `"say \"hi\""`},
		"a backslash":    {plain: `back\slash\`, quoted: `"back\\slash\\"`},
		"shell words":    {plain: `it's $HOME; #x`, quoted: `"it's $HOME; #x"`},
		"empty":          {plain: "", quoted: `""`},
	}

	for name, secret := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var runs []ran

			store := storerFor(&runs, nil)

			// Act
			_, err := store(secret.plain)

			// Assert
			want := `add-generic-password -U -a "jacob" -s workflow-jira -w ` + secret.quoted + "\n"
			if err != nil || len(runs) != 1 || runs[0].input != want {
				t.Errorf("store(%q) sent %+v, %v; want the line %q", secret.plain, runs, err, want)
			}
		})
	}
}

func TestStorerRefusesASecretThatCannotBeOneLine(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"a line break": "first\nsecond",
		"a NUL byte":   "cut\x00short",
		"too long":     strings.Repeat("x", 4096),
	}

	for name, secret := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var runs []ran

			store := storerFor(&runs, nil)

			// Act
			tokenCommand, err := store(secret)

			// Assert
			if !errors.Is(err, keychain.ErrSecretNotOneLine) || tokenCommand != "" {
				t.Errorf("store = %q, %v; want ErrSecretNotOneLine", tokenCommand, err)
			}

			if len(runs) != 0 {
				t.Errorf("store ran %d programs for a refused secret, want none", len(runs))
			}
		})
	}
}

func TestStorerReportsARefusedStore(t *testing.T) {
	t.Parallel()

	// Arrange
	var runs []ran

	store := storerFor(&runs, errKeychainLocked)

	// Act
	tokenCommand, err := store("s3cret")

	// Assert
	if !errors.Is(err, errKeychainLocked) || tokenCommand != "" {
		t.Errorf("store = %q, %v; want no token_command and security's error", tokenCommand, err)
	}
}

// A security that never answers must not hold config init open, so the store
// runs it under proc.DefaultRunTimeout, the bound proc.Run gives a quick read.
func TestStorerBoundsSecurityByTheDefaultRunTimeout(t *testing.T) {
	t.Parallel()

	// Arrange
	var (
		deadline time.Time
		bounded  bool
	)

	runner := func(ctx context.Context, _ proc.Command, _ []byte) ([]byte, error) {
		deadline, bounded = ctx.Deadline()

		return nil, nil
	}
	store := keychain.Storer("darwin", runner, loggedInAs("jacob"), userIs(""))
	before := time.Now()

	// Act
	_, err := store("s3cret")
	after := time.Now()

	// Assert
	if err != nil || !bounded {
		t.Fatalf("store = %v, with security given a deadline %t; want it run under one", err, bounded)
	}

	if deadline.Before(before.Add(proc.DefaultRunTimeout)) || deadline.After(after.Add(proc.DefaultRunTimeout)) {
		t.Errorf("security's deadline is %s after the store began, want %s", deadline.Sub(before), proc.DefaultRunTimeout)
	}
}

func TestStorerNamesTheAccountAfterTheUser(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		lookup  keychain.UserLookup
		user    string
		account string
	}{
		"the current user's login name": {lookup: loggedInAs("jacob"), user: "someone-else", account: `"jacob"`},
		"$USER when the lookup fails":   {lookup: failedLookup, user: "fallback", account: `"fallback"`},
		"$USER when the user has no login name": {
			lookup: loggedInAs(""), user: "fallback", account: `"fallback"`,
		},
		"a name quoted for security": {lookup: failedLookup, user: `two "words"`, account: `"two \"words\""`},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var runs []ran

			store := keychain.Storer("darwin", recordingRunner(&runs, nil), tt.lookup, userIs(tt.user))

			// Act
			_, err := store("s3cret")

			// Assert
			want := "add-generic-password -U -a " + tt.account + ` -s workflow-jira -w "s3cret"` + "\n"
			if err != nil || len(runs) != 1 || runs[0].input != want {
				t.Errorf("store sent %+v, %v; want the line %q", runs, err, want)
			}
		})
	}
}

func TestStorerRefusesWithoutAnAccountName(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		lookup keychain.UserLookup
		causes []error
	}{
		"the lookup fails": {lookup: failedLookup, causes: []error{keychain.ErrNoAccount, errUnknownUser}},
		"no login name":    {lookup: loggedInAs(""), causes: []error{keychain.ErrNoAccount}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var runs []ran

			store := keychain.Storer("darwin", recordingRunner(&runs, nil), tt.lookup, userIs(""))

			// Act
			tokenCommand, err := store("s3cret")

			// Assert
			for _, cause := range tt.causes {
				if !errors.Is(err, cause) {
					t.Errorf("store = %q, %v; want it to wrap %v", tokenCommand, err, cause)
				}
			}

			if tokenCommand != "" || len(runs) != 0 {
				t.Errorf("store = %q and ran %d programs without an account, want neither", tokenCommand, len(runs))
			}
		})
	}
}
