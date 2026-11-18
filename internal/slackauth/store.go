// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slackauth

import (
	"context"
	"crypto/rand"
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
// token it replaced is spent, so a login with it would be refused too. Only a
// fresh refresh token, from reinstalling the app, and a login with it bring
// posting back.
var ErrNotKept = errors.New("slack gave a new token that could not be kept, and the refresh token it " +
	"replaced is spent; reinstall the app for a fresh refresh token, then run `workflow slack login`")

// ErrKeepRefused reports a place the Slack token is kept that would not take
// a write, found before a refresh, so no refresh token was spent and the pair
// held still works. What to do about it follows the cause: see Store.refused.
var ErrKeepRefused = errors.New("the place the Slack token is kept would not take a write, so no refresh " +
	"token was spent")

// unwritableAdvice is what to do about a keychain that would not store, or a
// directory that takes no new file; unparsedAdvice about a configuration file
// that does not parse, which no keychain or directory answers.
const (
	unwritableAdvice = "unlock the keychain or make the configuration file's directory writable, then try again"
	unparsedAdvice   = "fix the file, as workflow doctor says, then try again"
)

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

// probeService is the keychain item a keychain store proves itself by,
// stored with a fresh value and deleted again, so the item the credentials
// are kept in is never touched by a prove.
const probeService = "workflow-slack-probe"

// Store is where a user token's credentials are kept: the macOS keychain, or the
// configuration file.
type Store struct {
	where      string
	load       func(ctx context.Context) (Credentials, error)
	save       func(ctx context.Context, credentials Credentials) error
	login      func(ctx context.Context, clientID string, credentials Credentials) error
	prove      func(ctx context.Context) error
	proveLogin func(ctx context.Context) error
}

// NewStore is a Store that keeps credentials through load and save, and
// proves it would take a write through prove, described as where. It names
// no app: a login keeps the credentials alone, and proves as a refresh does.
func NewStore(
	where string, load func(context.Context) (Credentials, error), save func(context.Context, Credentials) error,
	prove func(context.Context) error,
) Store {
	return Store{
		where: where, load: load, save: save,
		login: func(ctx context.Context, _ string, credentials Credentials) error { return save(ctx, credentials) },
		prove: prove, proveLogin: prove,
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

// Prove reports, as ErrKeepRefused, a store that would not take a write now,
// found without touching what it holds, so a refresh, which spends the
// refresh token as Slack answers, is made only where its pair can be kept.
func (s Store) Prove(ctx context.Context) error {
	return s.refused(s.prove(ctx))
}

// ProveLogin is Prove for a login, which also names its app in a
// configuration file: that file is proved to take a write too.
func (s Store) ProveLogin(ctx context.Context) error {
	return s.refused(s.proveLogin(ctx))
}

// Where says where the credentials are kept, for doctor and the login to say.
func (s Store) Where() string {
	return s.where
}

// refused is a prove's failure as ErrKeepRefused, naming where, and then
// what answers it: fixing a file that does not parse, or else unlocking the
// keychain or the directory.
func (s Store) refused(err error) error {
	if err == nil {
		return nil
	}

	advice := unwritableAdvice
	if errors.Is(err, config.ErrInvalid) {
		advice = unparsedAdvice
	}

	return fmt.Errorf("%w (in %s): %w; %s", ErrKeepRefused, s.where, err, advice)
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
func Choose(goos string, cfg config.Config, item keychain.Item, locks config.SaveLocks) Store {
	if goos != "darwin" || cfg.Messaging.HoldsUserTokenSecrets() {
		return FileStore(locks, cfg.Layers())
	}

	return KeychainStore(item, locks, cfg.Layers())
}

// FileStore keeps the credentials in one of the configuration files: the one
// that already holds them, or else the home directory's, so a first login
// puts no secret in a file in a working tree that can do without one. It
// rewrites only their fields and keeps every other setting as it is, under
// locks, as config.SaveLayers holds them.
func FileStore(locks config.SaveLocks, files config.Files) Store {
	path := secretsFile(files)
	prove := func(context.Context) error { return proveFile(files, path) }

	return Store{
		where: "the configuration file " + path,
		load:  func(context.Context) (Credentials, error) { return loadFile(path) },
		save: func(_ context.Context, credentials Credentials) error {
			return saveFile(locks, files, path, holding(credentials))
		},
		login: func(_ context.Context, clientID string, credentials Credentials) error {
			return saveFile(locks, files, path, func(messaging config.Messaging) config.Messaging {
				return naming(clientID)(holding(credentials)(messaging))
			})
		},
		prove: prove, proveLogin: prove,
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
// the layer it is; the home file is written whole, as itself. The write holds
// locks, as config.SaveLayers does.
func saveFile(
	locks config.SaveLocks, files config.Files, path string, change func(config.Messaging) config.Messaging,
) error {
	layers := savedLayers(files, path)

	cfg, revision, err := config.LoadLayersAt(layers)
	if err != nil {
		return err
	}

	cfg.Messaging = change(cfg.Messaging)

	_, err = config.SaveLayers(locks, layers, cfg, revision)

	return err
}

// savedLayers are the layers a save into path, one of files, reads and
// writes: the repository's file as the layer it is over the home file, and
// the home file whole, as itself.
func savedLayers(files config.Files, path string) config.Files {
	if path == files.Home {
		return config.Files{Home: path}
	}

	return files
}

// proveFile proves that path, one of files, would take the save of a pair:
// the layers its save reads parse, as the save would refuse them otherwise,
// and a file can be made beside it. The file itself is left as it is.
func proveFile(files config.Files, path string) error {
	_, _, err := config.LoadLayersAt(savedLayers(files, path))
	if err != nil {
		return err
	}

	return config.ProveWritable(path)
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
// which every directory reads, as every directory reads the keychain. A
// login's naming holds locks, as config.SaveLayers does.
func KeychainStore(item keychain.Item, locks config.SaveLocks, files config.Files) Store {
	probe := item.Under(probeService)

	return Store{
		where: "the macOS keychain (" + KeychainService + ")",
		load:  func(ctx context.Context) (Credentials, error) { return loadKeychain(ctx, item) },
		save:  func(ctx context.Context, credentials Credentials) error { return saveKeychain(ctx, item, credentials) },
		login: keychainLogin(item, locks, files),
		prove: func(ctx context.Context) error { return proveKeychain(ctx, probe) },
		proveLogin: func(ctx context.Context) error {
			err := proveFile(files, secretsFile(files))
			if err != nil {
				return err
			}

			return proveKeychain(ctx, probe)
		},
	}
}

// proveKeychain proves the keychain would keep a pair by storing a fresh
// value in probe, which reads it back as every store does, then deleting it.
// A keychain is locked or unlocked as a whole, so the probe meets what a
// keep would, whether or not a pair is held; and a fresh value cannot pass
// the read back on a probe an earlier run left.
func proveKeychain(ctx context.Context, probe keychain.Item) error {
	err := probe.Store(ctx, rand.Text())
	if err != nil {
		return err
	}

	return probe.Delete(ctx)
}

// keychainLogin is a login that names its app in the one of files the secrets
// would be read from, then keeps its credentials in item, and takes the naming
// back when the keychain refuses them: so a pair is never kept without its
// app, and an app is left named only for the pair it is for.
func keychainLogin(
	item keychain.Item, locks config.SaveLocks, files config.Files,
) func(context.Context, string, Credentials) error {
	return func(ctx context.Context, clientID string, credentials Credentials) error {
		path := secretsFile(files)

		var before config.Messaging

		err := saveFile(locks, files, path, func(messaging config.Messaging) config.Messaging {
			before = messaging

			return naming(clientID)(messaging)
		})
		if err != nil {
			return err
		}

		err = saveKeychain(ctx, item, credentials)
		if err != nil {
			return errors.Join(err, saveFile(locks, files, path, renaming(before)))
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
