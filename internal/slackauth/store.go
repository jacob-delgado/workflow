// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slackauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/keychain"
)

// ErrNotLoggedIn reports that no Slack user token has been set up where the
// configuration says it is kept; `workflow slack login` sets one up.
var ErrNotLoggedIn = errors.New("no Slack user token is set up; run `workflow slack login`")

// ErrNotKept reports a new pair Slack gave that could not be kept: the refresh
// token it replaced is spent, so only a fresh login brings posting back.
var ErrNotKept = errors.New("slack gave a new token that could not be kept; run `workflow slack login` again")

// keepTries is how many times a new pair is saved before it is given up: the
// file's save is refused when an edit lands between its read and its write,
// and a try after that reads the file again.
const keepTries = 3

// KeychainService is the name the credentials are kept under in the keychain.
const KeychainService = "workflow-slack"

// Store is where a user token's credentials are kept: the macOS keychain, or the
// configuration file.
type Store struct {
	where string
	load  func(ctx context.Context) (Credentials, error)
	save  func(ctx context.Context, credentials Credentials) error
}

// NewStore is a Store that keeps credentials through load and save, described
// as where.
func NewStore(
	where string, load func(context.Context) (Credentials, error), save func(context.Context, Credentials) error,
) Store {
	return Store{where: where, load: load, save: save}
}

// Load reads the credentials, or ErrNotLoggedIn when none are kept.
func (s Store) Load(ctx context.Context) (Credentials, error) {
	return s.load(ctx)
}

// Save keeps credentials in place of any kept before.
func (s Store) Save(ctx context.Context, credentials Credentials) error {
	return s.save(ctx, credentials)
}

// Keep saves a pair Slack just gave, trying again where a save fails, since
// the refresh token it replaced is already spent; ErrNotKept says it could not.
func (s Store) Keep(ctx context.Context, renewed Credentials) error {
	var err error

	for range keepTries {
		err = s.Save(ctx, renewed)
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("%w (in %s): %w", ErrNotKept, s.where, err)
}

// Where says where the credentials are kept, for doctor and the login to say.
func (s Store) Where() string {
	return s.where
}

// Choose is where the configuration keeps a user token's credentials: in the
// configuration file when it already holds them, or on any system but macOS,
// which has no keychain workflow drives; otherwise in item, the macOS keychain.
func Choose(goos string, cfg config.Config, item keychain.Item) Store {
	if goos != "darwin" || cfg.Messaging.HoldsUserTokenSecrets() {
		return FileStore(cfg.Layers())
	}

	return KeychainStore(item)
}

// FileStore keeps the credentials in one of the configuration files: the one
// that already holds them, or else the home directory's, so a first login
// puts no secret in a file in a working tree that can do without one. It
// rewrites only their fields and keeps every other setting as it is.
func FileStore(files config.Files) Store {
	path := secretsFile(files)

	return NewStore("the configuration file "+path,
		func(context.Context) (Credentials, error) { return loadFile(path) },
		func(_ context.Context, credentials Credentials) error { return saveFile(files, path, credentials) })
}

// secretsFile is the file of files the credentials are kept in.
func secretsFile(files config.Files) string {
	for _, path := range []string{files.Repo, files.Home} {
		held, err := readSecrets(path)
		if err == nil && held.ClientSecret+held.RefreshToken != "" {
			return path
		}
	}

	if files.Home != "" {
		return files.Home
	}

	return files.Repo
}

// readSecrets reads the user token's fields from the one file at path, on
// their own: a repository's file can be a few settings over the home
// directory's, and need not be a configuration by itself.
func readSecrets(path string) (config.Messaging, error) {
	//nolint:gosec // the path is the user's own config file, by design
	contents, err := os.ReadFile(path)
	if err != nil {
		return config.Messaging{}, fmt.Errorf("reading %s: %w", path, err)
	}

	var held struct {
		Messaging config.Messaging `json:"messaging"`
	}

	err = json.Unmarshal(contents, &held)
	if err != nil {
		return config.Messaging{}, fmt.Errorf("reading %s: %w", path, err)
	}

	return held.Messaging, nil
}

// loadFile reads the credentials the configuration file at path holds.
func loadFile(path string) (Credentials, error) {
	messaging, err := readSecrets(path)
	if err != nil {
		return Credentials{}, err
	}

	if messaging.RefreshToken == "" || messaging.ClientSecret == "" {
		return Credentials{}, ErrNotLoggedIn
	}

	expires, err := expiry(messaging.ExpiresAt)
	if err != nil {
		return Credentials{}, err
	}

	return Credentials{
		ClientSecret: messaging.ClientSecret, RefreshToken: messaging.RefreshToken,
		AccessToken: messaging.AccessToken, ExpiresAt: expires,
	}, nil
}

// saveFile writes credentials into path, one of files, over the revision it
// read, so an edit landing between the read and the write is refused rather
// than lost. The repository's file is written as the layer it is; the home
// file is written whole, as itself.
func saveFile(files config.Files, path string, credentials Credentials) error {
	layers := files
	if path == files.Home {
		layers = config.Files{Home: path}
	}

	cfg, revision, err := config.LoadLayersAt(layers)
	if err != nil {
		return err
	}

	cfg.Messaging.ClientSecret = credentials.ClientSecret
	cfg.Messaging.RefreshToken = credentials.RefreshToken
	cfg.Messaging.AccessToken = credentials.AccessToken
	cfg.Messaging.ExpiresAt = stamp(credentials.ExpiresAt)

	_, err = config.SaveLayers(layers, cfg, revision)

	return err
}

// stored is the credentials as the keychain holds them: one line of JSON.
type stored struct {
	ClientSecret string `json:"client_secret"`
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
	ExpiresAt    string `json:"expires_at"`
}

// KeychainStore keeps the credentials in item, one line of JSON in the macOS
// keychain.
func KeychainStore(item keychain.Item) Store {
	return NewStore("the macOS keychain ("+KeychainService+")",
		func(ctx context.Context) (Credentials, error) { return loadKeychain(ctx, item) },
		func(ctx context.Context, credentials Credentials) error { return saveKeychain(ctx, item, credentials) })
}

// loadKeychain reads the credentials item holds.
func loadKeychain(ctx context.Context, item keychain.Item) (Credentials, error) {
	line, err := item.Read(ctx)
	if errors.Is(err, keychain.ErrNotStored) {
		return Credentials{}, ErrNotLoggedIn
	}

	if err != nil {
		return Credentials{}, err
	}

	var held stored

	err = json.Unmarshal([]byte(line), &held)
	if err != nil || held.RefreshToken == "" || held.ClientSecret == "" {
		return Credentials{}, fmt.Errorf("the keychain's %s entry is not workflow's: %w", KeychainService, ErrNotLoggedIn)
	}

	expires, err := expiry(held.ExpiresAt)
	if err != nil {
		return Credentials{}, err
	}

	return Credentials{
		ClientSecret: config.Secret(held.ClientSecret), RefreshToken: config.Secret(held.RefreshToken),
		AccessToken: config.Secret(held.AccessToken), ExpiresAt: expires,
	}, nil
}

// saveKeychain keeps credentials in item.
func saveKeychain(ctx context.Context, item keychain.Item, credentials Credentials) error {
	// Trade-off TRADE-13: stored holds only strings, which always encode.
	line, err := json.Marshal(stored{
		ClientSecret: credentials.ClientSecret.Reveal(), RefreshToken: credentials.RefreshToken.Reveal(),
		AccessToken: credentials.AccessToken.Reveal(), ExpiresAt: stamp(credentials.ExpiresAt),
	})
	if err != nil {
		return fmt.Errorf("encoding the credentials: %w", err)
	}

	return item.Store(ctx, string(line))
}

// expiry reads a stored expiry; none is the zero time, which reads as expired.
func expiry(stampedAt string) (time.Time, error) {
	if stampedAt == "" {
		return time.Time{}, nil
	}

	at, err := time.Parse(time.RFC3339, stampedAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("the stored expiry %q is not a time: %w", stampedAt, ErrNotLoggedIn)
	}

	return at.UTC(), nil
}

// stamp writes an expiry as RFC 3339 UTC; the zero time is none.
func stamp(at time.Time) string {
	if at.IsZero() {
		return ""
	}

	return at.UTC().Format(time.RFC3339)
}
