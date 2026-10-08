// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/keychain"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// Token source descriptions doctor reports, so the reader knows where a
// credential came from without the credential being shown.
const (
	sourceFile    = "the configuration file"
	sourceCommand = "token_command"
	sourceNone    = "no token configured"
)

// errNoToken is a token source that is set but gives nothing to send.
var errNoToken = errors.New("no token")

// errTokenCommandFailed is a token command that ran and gave no token.
var errTokenCommandFailed = errors.New("the token command failed")

// ResolveToken finds the Jira token settings name — the file's own, the
// keychain item kept for its address, read through system, an environment
// variable, or a command, in that order — and names where it came from, so a
// token need never be written into the file. The keychain item is the one for
// the address the token goes to, so a token kept for another address is never
// read for this one.
//
// A command is split on spaces and run as a program, no shell, so it stays pure
// Go and works on every platform; wrap a pipeline in a script if one is needed.
func ResolveToken(ctx context.Context, settings config.Jira, system Keychain) (config.Secret, string, error) {
	switch {
	case settings.Token != "":
		return settings.Token, sourceFile, nil
	case settings.Keychain:
		return system.jiraToken(ctx, settings)
	case settings.TokenEnv != "":
		variable := settings.TokenEnv

		return config.Secret(strings.TrimSpace(os.Getenv(variable))), "the " + variable + " environment variable", nil
	case settings.TokenCommand != "":
		return fromCommand(ctx, settings.TokenCommand)
	default:
		return "", sourceNone, nil
	}
}

// JiraTokenKeeper keeps a Jira token typed into Settings in k, under the item
// named, bounded as a quick read is so a security that never answers cannot
// hold the save open; nil where workflow drives no keychain, so the file keeps
// it.
func (k Keychain) JiraTokenKeeper(ctx context.Context) func(service, secret string) error {
	if !keychain.Wired(k.GOOS) {
		return nil
	}

	return func(service, secret string) error {
		item, _ := keychain.Open(k.GOOS, service, k.Run, user.Current, os.Getenv)

		bounded, cancel := context.WithTimeout(ctx, proc.DefaultRunTimeout)
		defer cancel()

		return item.Store(bounded, secret)
	}
}

// jiraToken reads the token the keychain keeps for settings' address, naming
// the item it read as its source.
func (k Keychain) jiraToken(ctx context.Context, settings config.Jira) (config.Secret, string, error) {
	service := settings.KeychainService()
	source := "the keychain item " + service

	item, wired := keychain.Open(k.GOOS, service, k.Run, user.Current, os.Getenv)
	if !wired {
		return "", source, fmt.Errorf("%w: jira.keychain reads none here", keychain.ErrNotWired)
	}

	token, err := item.Read(ctx)
	if err != nil {
		return "", source, fmt.Errorf("reading the Jira token: %w", err)
	}

	return config.Secret(strings.TrimSpace(token)), source, nil
}

// fromCommand runs the token command and returns its trimmed output.
func fromCommand(ctx context.Context, command string) (config.Secret, string, error) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", sourceCommand, nil
	}

	out, err := proc.Run(ctx, fields[0], fields[1:]...)
	if err != nil {
		return "", sourceCommand, commandFailure(fields[0], err)
	}

	return config.Secret(strings.TrimSpace(string(out))), sourceCommand, nil
}

// commandFailure is why the token command gave no token, told by the kind of
// failure alone and never in proc's words, which carry what the program wrote
// to standard error: a command that reads a secret can print it, or what leads
// to it, as it fails.
func commandFailure(program string, err error) error {
	for _, kind := range []error{proc.ErrNotFound, proc.ErrTimedOut} {
		if errors.Is(err, kind) {
			return fmt.Errorf("%w: %s: %w", errTokenCommandFailed, program, kind)
		}
	}

	return fmt.Errorf("%w: %s; run it yourself to see why", errTokenCommandFailed, program)
}

// resolveSetToken finds the token a source the configuration sets gives, and
// refuses one that gives nothing: sent, an empty credential would only be
// turned away, for a reason the user could not act on.
func resolveSetToken(ctx context.Context, settings config.Jira, system Keychain) (config.Secret, error) {
	token, source, err := ResolveToken(ctx, settings, system)
	if err != nil {
		return "", err
	}

	if token == "" {
		return "", fmt.Errorf("%w from %s", errNoToken, source)
	}

	return token, nil
}
