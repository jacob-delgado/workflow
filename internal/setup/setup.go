// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package setup is the first run every surface shares: where the
// configuration file goes, Jira's address and token checked against Jira
// before they are kept, the OS keychain offered for the token, a messaging
// webhook, and the file written. The command line's `config init` asks its
// questions at a prompt; the terminal and the web ask the same ones in a form.
package setup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// Errors a setup reports. Callers tell them apart with errors.Is.
var (
	// ErrExists reports a file setup would have written over: it holds
	// credentials that are not recoverable once overwritten.
	ErrExists = errors.New("configuration file already exists")
	// ErrNoKeychain reports the token asked to be kept in a keychain where
	// none is wired, or under a dry run.
	ErrNoKeychain = errors.New("there is no keychain here to keep the token in")
	// ErrNoHome reports a file asked for in a home directory where none is
	// known.
	ErrNoHome = errors.New("there is no home directory here to keep the file in")
	// ErrKeychainAtHome reports the token asked to be kept in the keychain
	// for a file other than the home directory's: the file reads it back with
	// a jira.token_command, which only the home file may set.
	ErrKeychainAtHome = errors.New("the keychain can keep the token only for the home directory's file; " +
		"write the file there, or keep the token in the file")
)

// Place is where setup writes the file.
type Place string

// The places a file may go.
const (
	// Repository is the repository's root, so every subdirectory sees it, or
	// the working directory outside a repository.
	Repository Place = "repository"
	// Home is the home directory, where every directory sees it.
	Home Place = "home"
)

// Where is where setup runs: the working directory and the home directory.
type Where struct {
	WorkDir string
	HomeDir string
}

// Path is the file setup writes for place.
func (w Where) Path(place Place) string {
	if place == Home {
		return filepath.Join(w.HomeDir, config.FileName)
	}

	return filepath.Join(config.RepoRoot(w.WorkDir), config.FileName)
}

// IsHomeFile reports whether the file for place is the home directory's: the
// home place, or the repository's where the repository's root is the home
// directory.
func (w Where) IsHomeFile(place Place) bool {
	return w.HomeDir != "" && w.Path(place) == w.Path(Home)
}

// Places are where the file may go here: the repository, and the home
// directory where one is known.
func (w Where) Places() []Place {
	if w.HomeDir == "" {
		return []Place{Repository}
	}

	return []Place{Repository, Home}
}

// Layers are the home file the file for place lies over, and that file; empty
// when it stands alone: written to the home directory, where the repository's
// root is the home directory, with no home directory known, or with no home
// file to lie over.
func (w Where) Layers(place Place) config.Files {
	path, home := w.Path(place), w.Path(Home)
	if place == Home || w.HomeDir == "" || home == path {
		return config.Files{}
	}

	_, err := os.Stat(home)
	if err != nil {
		return config.Files{}
	}

	return config.Files{Home: home, Repo: path}
}

// RefuseExisting refuses anything already at path: a file, or a link, even
// one to nothing, which a write would follow to wherever it points.
func RefuseExisting(path string) error {
	_, err := os.Lstat(path)
	if err == nil {
		return fmt.Errorf("%w: %s", ErrExists, path)
	}

	return nil
}

// Answers are what setup asks: Jira's address and token, and a Slack incoming
// webhook. A blank address or webhook skips that question.
type Answers struct {
	Jira    config.Jira
	Webhook config.Secret
}

// Over is cfg with the answers in place of its own, and as it was where a
// question was left blank, so a blank answer over a home file keeps the home
// file's.
func (a Answers) Over(cfg config.Config) config.Config {
	cfg.Jira = withJira(cfg.Jira, a.Jira)
	cfg.Messaging = withWebhook(cfg.Messaging, a.Webhook)

	return cfg
}

// withJira is settings with the address and token asked for in place of its
// own, or settings as they were when the question was left blank.
func withJira(settings, asked config.Jira) config.Jira {
	if asked.BaseURL == "" {
		return settings
	}

	settings.BaseURL, settings.Token, settings.TokenCommand, settings.TokenEnv =
		asked.BaseURL, asked.Token, asked.TokenCommand, ""

	return settings
}

// withWebhook is messaging posting to Slack through webhook, or messaging as
// it was when the question was left blank. A webhook stands alone, so a user
// token it replaces goes with it.
func withWebhook(messaging config.Messaging, webhook config.Secret) config.Messaging {
	if webhook == "" {
		return messaging
	}

	messaging.Kind, messaging.WebhookURL, messaging.ClientID = config.KindSlack, webhook, ""
	messaging.ClientSecret, messaging.RefreshToken, messaging.AccessToken, messaging.ExpiresAt = "", "", "", ""

	return messaging
}

// Check asks Jira over doer who the token in settings authenticates as, and
// names them.
func Check(ctx context.Context, doer jira.Doer, settings config.Jira) (string, error) {
	user, err := jira.New(doer, settings).Myself(ctx)
	if err != nil {
		return "", fmt.Errorf("checking the token with Jira: %w", err)
	}

	return Identify(user), nil
}

// Keepable reports whether a check that failed with err may be kept anyway:
// a Jira that is unreachable or refuses the token now may answer later, but an
// address that is not one never will, and one carrying a username and password
// must never reach the file.
func Keepable(err error) bool {
	return !errors.Is(err, config.ErrInvalidBaseURL) && !errors.Is(err, config.ErrCredentialInBaseURL)
}

// Identify names a user, falling back to the login when an instance is
// configured to withhold display names.
func Identify(user jira.User) string {
	if user.DisplayName == "" {
		return user.Name
	}

	return user.DisplayName + " (" + user.Name + ")"
}

// Keep moves the token in settings into the OS keychain through store, so the
// file holds the token_command that reads it back rather than the secret.
func Keep(store func(secret string) (string, error), settings config.Jira) (config.Jira, error) {
	if store == nil {
		return settings, ErrNoKeychain
	}

	tokenCommand, err := store(settings.Token.Reveal())
	if err != nil {
		return settings, fmt.Errorf("storing the token in the keychain: %w", err)
	}

	settings.Token, settings.TokenCommand = "", tokenCommand

	return settings, nil
}

// Beneath is the configuration a new file starts from — the home file's, when
// it lies over one, or the defaults — at the revision of the pair, which the
// save must still find them at.
func Beneath(layers config.Files) (config.Config, config.Revision, error) {
	over, err := config.RevisionOfLayers(layers)
	if err != nil {
		return config.Default(), config.Revision{}, err
	}

	if layers.Home == "" {
		return config.Default(), over, nil
	}

	beneath, _, err := config.LoadLayersAt(config.Files{Home: layers.Home})
	if err != nil {
		return config.Default(), config.Revision{}, err
	}

	return beneath, over, nil
}

// Save writes cfg to path: a file of its own, or, over a home file, the layer
// that differs from it.
func Save(path string, layers config.Files, cfg config.Config, over config.Revision) error {
	if layers.Home == "" {
		return config.Save(path, cfg)
	}

	_, err := config.SaveLayers(layers, cfg, over)

	return err
}

// Create writes cfg to path as Save does, but only as a new file: anything
// already there, a link included, is refused with ErrExists and nothing is
// written, so a file made between RefuseExisting and the write is never
// replaced and a link is never followed.
func Create(path string, layers config.Files, cfg config.Config, over config.Revision) error {
	var err error
	if layers.Home == "" {
		err = config.Create(path, cfg)
	} else {
		err = config.CreateLayers(layers, cfg, over)
	}

	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%w: %s", ErrExists, path)
	}

	return err
}

// NotIgnored reports a file inside a repository that git does not ignore,
// which, holding credentials, could be committed. Outside a repository there
// is nothing to commit it to.
func NotIgnored(ctx context.Context, path string) bool {
	ignored, err := gitrepo.At(proc.Run, filepath.Dir(path)).CheckIgnored(ctx, path)

	return err == nil && !ignored
}
