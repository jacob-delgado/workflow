// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/keychain"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/slackauth"
	"github.com/jacob-delgado/workflow/internal/store"
)

// slackLockName is the file a Slack token refresh is made under, beside the
// store, so every workflow running on the machine takes turns.
const slackLockName = "slack-refresh.lock"

// liveMessaging is the messaging settings every post reads: those workflow
// started with, until the web's Settings saves others.
type liveMessaging struct {
	lock     sync.Mutex
	settings config.Messaging
}

// current is the settings in effect now.
func (l *liveMessaging) current() config.Messaging {
	l.lock.Lock()
	defer l.lock.Unlock()

	return l.settings
}

// replace puts settings in effect for every post after it.
func (l *liveMessaging) replace(settings config.Messaging) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.settings = settings
}

// messagingSetup is what a post is made from: the settings in effect, the
// configuration file a user token's credentials may be kept in, and the
// transport and request log.
type messagingSetup struct {
	settings      func() config.Messaging
	path          string
	httpTransport httpx.Doer
	log           *RequestLog
}

// messagingDeps is what a surface asks of the messaging service. Each post is
// made by a client built from the settings in effect, whose user token is
// asked for anew, since a rotating token can run out between two posts. The
// Slack directory is read through the session's cache, under a Slack user
// token only.
func messagingDeps(ctx context.Context, setup messagingSetup) seams.Messaging {
	client := func() messaging.Client { return messagingClient(setup) }
	bound := seams.Messaging{Post: func(channel, text string) error {
		return client().Post(ctx, channel, text)
	}}

	return withDirectory(ctx, bound, slackDirectoryFor(setup.settings(), client))
}

// messagingClient builds the client a post is made with: over the webhook the
// settings name, whose path is its credential and is never logged, or with
// the Slack user token the settings set up.
func messagingClient(setup messagingSetup) messaging.Client {
	settings := setup.settings()
	service := strings.ToLower(settings.Service())

	if settings.Mode() != config.MessagingUser {
		//nolint:bodyclose // wrap only relays the response; the messaging client reads and closes its body.
		return messaging.New(setup.log.wrapWebhook(service, setup.httpTransport), messaging.APIBase, settings)
	}

	//nolint:bodyclose // Wrap only relays the response; the client reads and closes its body.
	do := setup.log.Wrap(service, setup.httpTransport)

	return messaging.New(do, messaging.APIBase, settings).
		WithToken(SlackToken(config.Config{Messaging: settings, Path: setup.path}, do))
}

// SlackStore is where cfg keeps its Slack user token's credentials: the macOS
// keychain, or the configuration file. doctor, the login and every post go
// through it, so they cannot come to look in different places.
func SlackStore(cfg config.Config) slackauth.Store {
	item, _ := keychain.Open(runtime.GOOS, slackauth.KeychainService, proc.Capture, user.Current, os.Getenv)

	return slackauth.Choose(runtime.GOOS, cfg, item)
}

// SlackRefresher refreshes the Slack user token of the app clientID names,
// through do.
func SlackRefresher(clientID string, do httpx.Doer) slackauth.Refresher {
	return slackauth.Refresher{Do: do, Base: messaging.APIBase, ClientID: clientID, Now: time.Now}
}

// SlackToken hands out cfg's Slack user token, refreshing it through do when it
// is about to run out, under a lock every workflow on the machine shares. Its
// failures read as the messaging client's: none set up is no credential, a
// Slack not reached is unreachable, and a refresh Slack refused is a
// credential it would not accept.
func SlackToken(cfg config.Config, do httpx.Doer) messaging.TokenSource {
	source := slackauth.Source{
		Store: SlackStore(cfg), Refresher: SlackRefresher(cfg.Messaging.ClientID, do),
		Lock: slackauth.FileLock(slackLockPath(cfg.Path)), Now: time.Now,
	}

	return func(ctx context.Context, expired config.Secret) (config.Secret, error) {
		token, err := source.Token(ctx, expired)

		return token, asMessagingError(err)
	}
}

// asMessagingError is a token source's failure as the messaging client's error.
// A lock held elsewhere is no refusal of the token, so it keeps its own words.
func asMessagingError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, slackauth.ErrNotLoggedIn), errors.Is(err, slackauth.ErrNotKept):
		return fmt.Errorf("%w: %w", messaging.ErrNoCredential, err)
	case errors.Is(err, slackauth.ErrLocked):
		return err
	case errors.Is(err, slackauth.ErrUnreachable):
		return fmt.Errorf("%w: %w", messaging.ErrUnreachable, err)
	default:
		return fmt.Errorf("%w: %w", messaging.ErrRejected, err)
	}
}

// slackLockPath is where the refresh lock lives: beside the store, or beside
// the configuration file where the store has no directory.
func slackLockPath(configPath string) string {
	dir, err := store.DefaultDir()
	if err != nil || dir == "" {
		dir = filepath.Dir(configPath)
	}

	return filepath.Join(dir, slackLockName)
}

// placeSlackCredentials is Controls.PlaceSlackCredentials, refreshing through
// transport.
//
// Trade-off TRADE-17: no test sees a placement Slack accepts; that takes Slack itself.
func placeSlackCredentials(ctx context.Context, transport httpx.Doer) func(config.Config) (config.Config, error) {
	return func(cfg config.Config) (config.Config, error) {
		if runtime.GOOS != "darwin" {
			return cfg, nil
		}

		without := cfg
		without.Messaging.ClientSecret, without.Messaging.RefreshToken = "", ""
		without.Messaging.AccessToken, without.Messaging.ExpiresAt = "", ""

		starting := keptUnlessTyped(ctx, SlackStore(without), cfg.Messaging)

		renewed, err := SlackRefresher(cfg.Messaging.ClientID, transport).Refresh(ctx, starting)
		if err != nil {
			return config.Config{}, asMessagingError(err)
		}

		err = SlackStore(without).Keep(ctx, renewed)
		if err != nil {
			return config.Config{}, err
		}

		return without, nil
	}
}

// keptUnlessTyped is the secrets typed into settings, each one left blank
// taken from those keychain already keeps, so changing one leaves the other.
func keptUnlessTyped(ctx context.Context, keychain slackauth.Store, settings config.Messaging) slackauth.Credentials {
	starting := slackauth.Credentials{ClientSecret: settings.ClientSecret, RefreshToken: settings.RefreshToken}

	kept, err := keychain.Load(ctx)
	if err != nil {
		return starting
	}

	if starting.ClientSecret == "" {
		starting.ClientSecret = kept.ClientSecret
	}

	if starting.RefreshToken == "" {
		starting.RefreshToken = kept.RefreshToken
	}

	return starting
}
