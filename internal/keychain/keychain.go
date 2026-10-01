// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package keychain stores a secret in the operating system's keychain and reads
// it back, so a token need never sit in the configuration file. It links
// nothing: it drives the platform's own tool, which is what keeps the build pure
// Go.
package keychain

import (
	"context"
	"errors"
	"fmt"
	"os/user"
	"strings"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// ErrSecretNotOneLine reports a secret that cannot travel on one line of
// security's input: one holding a line break or a NUL byte, or one longer than
// the line security reads.
var ErrSecretNotOneLine = errors.New("the secret cannot be handed to security on one line")

// ErrNotStored reports a keychain that holds no secret under the service asked
// for.
var ErrNotStored = errors.New("the keychain holds no secret under that name")

// ErrNoAccount reports that neither the current user's login name nor $USER
// gives the account name security needs to store a secret under.
var ErrNoAccount = errors.New("no account name to store the secret under: neither the current user nor $USER names one")

// jiraService is the name the Jira token is stored under. A token_command
// already saved in a configuration file names it, so it stays as it is.
const jiraService = "workflow-jira"

// lookupCommand is the token_command that reads the Jira token back out of the
// keychain, printing it and nothing else. It names no account, so it finds the
// token whichever login name it was stored under.
const lookupCommand = "security find-generic-password -s " + jiraService + " -w"

// itemNotFound is the status security exits with when the keychain holds no
// such item (errSecItemNotFound).
const itemNotFound = 44

// maxLine is the longest command line security reads whole from its input, its
// newline included; it would read the rest of a longer one as a second command.
const maxLine = 4095

// Runner runs a program with input on its standard input and returns what it
// wrote to standard output. proc.Capture satisfies it.
type Runner func(ctx context.Context, program proc.Command, input []byte) ([]byte, error)

// UserLookup returns the user running this program. user.Current satisfies it.
type UserLookup func() (*user.User, error)

// Item is a secret the macOS keychain holds under service, for the user running
// this program.
type Item struct {
	service     string
	run         Runner
	currentUser UserLookup
	getenv      func(string) string
}

// Open is the keychain item for service on goos, and false where no keychain is
// wired: everywhere but macOS, whose built-in `security` it drives.
func Open(goos, service string, run Runner, currentUser UserLookup, getenv func(string) string) (Item, bool) {
	if goos != "darwin" {
		return Item{}, false
	}

	return Item{service: service, run: run, currentUser: currentUser, getenv: getenv}, true
}

// Store saves secret, replacing any the item held. security runs with -i,
// reading the command that stores the secret from its standard input, so the
// secret is never one of the program's arguments. It requires an account name,
// and the secret is stored under the login name of the user running this
// program: currentUser's, or getenv's $USER where that lookup fails or gives
// none.
func (i Item) Store(ctx context.Context, secret string) error {
	account, err := accountName(i.currentUser, i.getenv)
	if err != nil {
		return err
	}

	line, err := storeLine(i.service, account, secret)
	if err != nil {
		return err
	}

	_, err = i.run(ctx, proc.Command{Name: "security", Args: []string{"-i"}}, []byte(line))

	return err
}

// Read is the secret the item holds, or ErrNotStored when there is none.
func (i Item) Read(ctx context.Context) (string, error) {
	account, err := accountName(i.currentUser, i.getenv)
	if err != nil {
		return "", err
	}

	args := []string{"find-generic-password", "-a", account, "-s", i.service, "-w"}

	output, err := i.run(ctx, proc.Command{Name: "security", Args: args}, nil)
	if code, _, exited := proc.Failure(err); exited && code == itemNotFound {
		return "", fmt.Errorf("%w: %s", ErrNotStored, i.service)
	}

	if err != nil {
		return "", err
	}

	return strings.TrimRight(string(output), "\n"), nil
}

// Storer is how config init keeps the Jira token in the keychain on goos: a
// function that stores a secret through run and returns the token_command that
// reads it back. It is nil where storing is not wired for goos, so the guided
// flow keeps the token in the file there. Reading a secret back through a
// token_command works more widely — anywhere its own tool is installed — but
// storing is wired for macOS here, through the built-in `security`.
func Storer(
	goos string, run Runner, currentUser UserLookup, getenv func(string) string,
) func(secret string) (string, error) {
	item, ok := Open(goos, jiraService, run, currentUser, getenv)
	if !ok {
		return nil
	}

	return func(secret string) (string, error) {
		// Bounded as proc.Run bounds a quick read, so a security that never
		// answers cannot hold config init open.
		ctx, cancel := context.WithTimeout(context.Background(), proc.DefaultRunTimeout)
		defer cancel()

		err := item.Store(ctx, secret)
		if err != nil {
			return "", err
		}

		return lookupCommand, nil
	}
}

// accountName is the login name of the user running this program: the one
// currentUser finds, or $USER where that lookup fails or finds no name.
func accountName(currentUser UserLookup, getenv func(string) string) (string, error) {
	current, err := currentUser()
	if err == nil && current.Username != "" {
		return current.Username, nil
	}

	if name := getenv("USER"); name != "" {
		return name, nil
	}

	if err != nil {
		return "", fmt.Errorf("%w (looking up the current user: %w)", ErrNoAccount, err)
	}

	return "", ErrNoAccount
}

// storeLine is security's command line that saves secret for account under
// service, updating an existing entry (-U) rather than adding a second one.
func storeLine(service, account, secret string) (string, error) {
	if strings.ContainsAny(secret, "\n\x00") {
		return "", fmt.Errorf("%w: it holds a line break or a NUL byte", ErrSecretNotOneLine)
	}

	line := "add-generic-password -U -a " + quoted(account) + " -s " + service + " -w " + quoted(secret) + "\n"
	if len(line) > maxLine {
		return "", fmt.Errorf("%w: it is longer than the %d bytes security reads", ErrSecretNotOneLine, maxLine)
	}

	return line, nil
}

// quoted is value as one argument on security's command line: in double
// quotes, with each backslash and double quote escaped by a backslash, which
// security's own parser removes again.
func quoted(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}
