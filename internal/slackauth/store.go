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

// ErrAppOverridden reports a login whose app a repository's own file would
// override with another app's client ID, so the secrets kept would be refreshed
// for the wrong app there.
var ErrAppOverridden = errors.New("the repository's own configuration file names another Slack app")

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
	login func(ctx context.Context, clientID string, credentials Credentials) error
}

// NewStore is a Store that keeps credentials through load and save, described
// as where. It names no app: a login keeps the credentials alone.
func NewStore(
	where string, load func(context.Context) (Credentials, error), save func(context.Context, Credentials) error,
) Store {
	return Store{
		where: where, load: load, save: save,
		login: func(ctx context.Context, _ string, credentials Credentials) error { return save(ctx, credentials) },
	}
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
	return s.keepTrying(func() error { return s.Save(ctx, renewed) })
}

// KeepLogin keeps the pair a login's first refresh gave, as Keep does, with
// the app it is for, clientID, and Slack as the kind, named in the
// configuration file the secrets are read from: in the same write where that
// file keeps the pair, and just before the keychain keeps it where it does,
// taken back if the keychain refuses. So the secrets are never kept without
// the app they are for, and every directory that reads them reads the app too.
func (s Store) KeepLogin(ctx context.Context, clientID string, renewed Credentials) error {
	return s.keepTrying(func() error { return s.login(ctx, clientID, renewed) })
}

// Where says where the credentials are kept, for doctor and the login to say.
func (s Store) Where() string {
	return s.where
}

// keepTrying runs save until it succeeds, keepTries times at most.
func (s Store) keepTrying(save func() error) error {
	var err error

	for range keepTries {
		err = save()
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("%w (in %s): %w", ErrNotKept, s.where, err)
}

// Choose is where the configuration keeps a user token's credentials: in the
// configuration file when it already holds them, or on any system but macOS,
// which has no keychain workflow drives; otherwise in item, the macOS keychain.
func Choose(goos string, cfg config.Config, item keychain.Item) Store {
	if goos != "darwin" || cfg.Messaging.HoldsUserTokenSecrets() {
		return FileStore(cfg.Layers())
	}

	return KeychainStore(item, cfg.Layers())
}

// FileStore keeps the credentials in one of the configuration files: the one
// that already holds them, or else the home directory's, so a first login
// puts no secret in a file in a working tree that can do without one. It
// rewrites only their fields and keeps every other setting as it is.
func FileStore(files config.Files) Store {
	path := secretsFile(files)

	return Store{
		where: "the configuration file " + path,
		load:  func(context.Context) (Credentials, error) { return loadFile(path) },
		save: func(_ context.Context, credentials Credentials) error {
			return saveFile(files, path, holding(credentials))
		},
		login: func(_ context.Context, clientID string, credentials Credentials) error {
			return saveFile(files, path, func(messaging config.Messaging) config.Messaging {
				return naming(clientID)(holding(credentials)(messaging))
			})
		},
	}
}

// RefuseAppOverride refuses, with ErrAppOverridden, a login for the app
// clientID that would name it in the home file, beside the secrets or the
// keychain that keeps them, while the repository's own file names another app,
// whose client ID wins wherever that file is read. A login checks this before
// spending the refresh token typed.
func RefuseAppOverride(files config.Files, clientID string) error {
	named := secretsFile(files)
	if files.Repo == "" || named == files.Repo {
		return nil
	}

	own, err := readSecrets(files.Repo)
	if err != nil {
		return err
	}

	if own.ClientID == "" || own.ClientID == clientID {
		return nil
	}

	return fmt.Errorf("%w: %s sets messaging.client_id to %q, which wins there over the app %q a login names in %s; "+
		"remove it from that file, or set it to %q, then run the login again", ErrAppOverridden, files.Repo, own.ClientID,
		clientID, named, clientID)
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

// saveFile writes the messaging settings change makes into path, one of
// files, over the revision it read, so an edit landing between the read and
// the write is refused rather than lost. The repository's file is written as
// the layer it is; the home file is written whole, as itself.
func saveFile(files config.Files, path string, change func(config.Messaging) config.Messaging) error {
	layers := files
	if path == files.Home {
		layers = config.Files{Home: path}
	}

	cfg, revision, err := config.LoadLayersAt(layers)
	if err != nil {
		return err
	}

	cfg.Messaging = change(cfg.Messaging)

	_, err = config.SaveLayers(layers, cfg, revision)

	return err
}

// holding is messaging settings holding credentials in place of their own.
func holding(credentials Credentials) func(config.Messaging) config.Messaging {
	return func(messaging config.Messaging) config.Messaging {
		messaging.ClientSecret, messaging.RefreshToken = credentials.ClientSecret, credentials.RefreshToken
		messaging.AccessToken, messaging.ExpiresAt = credentials.AccessToken, stamp(credentials.ExpiresAt)

		return messaging
	}
}

// naming is messaging settings naming the Slack app clientID as the one its
// user token is for.
func naming(clientID string) func(config.Messaging) config.Messaging {
	return func(messaging config.Messaging) config.Messaging {
		messaging.Kind, messaging.ClientID = config.KindSlack, clientID

		return messaging
	}
}

// renaming is messaging settings naming the kind and app before named, as
// they were before a login named its own.
func renaming(before config.Messaging) func(config.Messaging) config.Messaging {
	return func(messaging config.Messaging) config.Messaging {
		messaging.Kind, messaging.ClientID = before.Kind, before.ClientID

		return messaging
	}
}

// stored is the credentials as the keychain holds them: one line of JSON.
type stored struct {
	ClientSecret string `json:"client_secret"`
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
	ExpiresAt    string `json:"expires_at"`
}

// KeychainStore keeps the credentials in item, one line of JSON in the macOS
// keychain, and a login's app in the one of files the secrets would be read
// from were the file to keep them: the home directory's where there is one,
// which every directory reads, as every directory reads the keychain.
func KeychainStore(item keychain.Item, files config.Files) Store {
	return Store{
		where: "the macOS keychain (" + KeychainService + ")",
		load:  func(ctx context.Context) (Credentials, error) { return loadKeychain(ctx, item) },
		save:  func(ctx context.Context, credentials Credentials) error { return saveKeychain(ctx, item, credentials) },
		login: keychainLogin(item, files),
	}
}

// keychainLogin is a login that names its app in the one of files the secrets
// would be read from, then keeps its credentials in item, and takes the naming
// back when the keychain refuses them: so a pair is never kept without its
// app, and an app is left named only for the pair it is for.
func keychainLogin(item keychain.Item, files config.Files) func(context.Context, string, Credentials) error {
	return func(ctx context.Context, clientID string, credentials Credentials) error {
		path := secretsFile(files)

		var before config.Messaging

		err := saveFile(files, path, func(messaging config.Messaging) config.Messaging {
			before = messaging

			return naming(clientID)(messaging)
		})
		if err != nil {
			return err
		}

		err = saveKeychain(ctx, item, credentials)
		if err != nil {
			return errors.Join(err, saveFile(files, path, renaming(before)))
		}

		return nil
	}
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
