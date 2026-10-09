// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/keychain"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/messaging/directory"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/slackauth"
)

// slackLockName is the file a Slack token refresh is made under, beside the
// store, so every workflow running on the machine takes turns.
const slackLockName = "slack-refresh.lock"

// SlackAPIVariable names the environment variable that points every Slack Web
// API request at another address than messaging.APIBase: a fake under test, or
// a proxy. It is never a configuration key, since a repository's
// .workflow.json must not be able to send a Slack token anywhere.
const SlackAPIVariable = "WORKFLOW_SLACK_API"

// ErrSlackAPIRefused reports a SlackAPIVariable that would send the Slack
// token in clear text off this machine, or that is no address at all.
var ErrSlackAPIRefused = errors.New(SlackAPIVariable +
	" must be an https:// address, or http:// to this machine (127.0.0.1, ::1 or localhost)")

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

// replace puts settings in effect for every post and directory read after it.
func (l *liveMessaging) replace(settings config.Messaging) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.settings = settings
}

// messagingSetup is what a post is made from: the settings in effect, the
// configuration files a user token's credentials may be kept in, and the
// transport and request log.
type messagingSetup struct {
	settings      func() config.Messaging
	files         config.Files
	httpTransport httpx.Doer
	log           *RequestLog
	env           Environment
}

// messagingDeps is what a surface asks of the messaging service. Each post is
// made by a client built from the settings in effect, whose user token is
// asked for anew, since a rotating token can run out between two posts. The
// Slack directory is read through the session's directory, which answers
// messaging.ErrNoCredential while the settings in effect have no Slack user
// token.
func messagingDeps(ctx context.Context, setup messagingSetup, slack *directory.Slack) seams.Messaging {
	bound := seams.Messaging{Post: func(channel, text string) error {
		return messagingClient(setup).Post(ctx, channel, text)
	}}

	return withDirectory(ctx, bound, slack)
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
	base, toSlack := setup.env.SlackAPI(do)

	return messaging.New(toSlack, base, settings).WithToken(
		setup.env.SlackToken(config.Config{Messaging: settings, Path: setup.files.Target(), Files: setup.files}, do),
	)
}

// SlackAPI is where the Slack Web API is asked and the transport to ask it
// through: messaging.APIBase over transport, or the address SlackAPIVariable
// names in the environment. An address it refuses comes back with a transport
// that refuses every request, so nothing is sent — and no token leaves —
// anywhere.
func (e Environment) SlackAPI(transport httpx.Doer) (string, httpx.Doer) {
	base, err := slackAPIBase(e.Getenv(SlackAPIVariable))
	if err != nil {
		return messaging.APIBase, func(*http.Request) (*http.Response, error) { return nil, err }
	}

	return base, transport
}

// slackAPIBase is value, the address SlackAPIVariable names, or
// messaging.APIBase when it is unset or empty.
func slackAPIBase(value string) (string, error) {
	if value == "" {
		return messaging.APIBase, nil
	}

	address, err := url.Parse(value)
	if err != nil || address.Host == "" {
		return "", ErrSlackAPIRefused
	}

	if address.Scheme == "https" || address.Scheme == "http" && httpx.OnThisMachine(address.Hostname()) {
		return value, nil
	}

	return "", ErrSlackAPIRefused
}

// Keychain is the operating system's keychain as the wiring reaches it: the
// system workflow runs on, how security is run there, and the environment
// variables it reads the user's name from where the system cannot say.
type Keychain struct {
	GOOS   string
	Run    keychain.Runner
	Getenv func(name string) string
}

// Keychain is the keychain of the system workflow runs on, its security
// found on the environment's PATH.
func (e Environment) Keychain() Keychain {
	return Keychain{GOOS: runtime.GOOS, Run: e.capture, Getenv: e.Getenv}
}

// SlackStore is where cfg keeps its Slack user token's credentials on the
// system workflow runs on, as Keychain.SlackStore says.
func (e Environment) SlackStore(cfg config.Config) slackauth.Store {
	return e.Keychain().SlackStore(cfg)
}

// SlackStore is where cfg keeps its Slack user token's credentials with k:
// the macOS keychain, or the configuration file. doctor, the login and every
// post go through it, so they cannot come to look in different places.
func (k Keychain) SlackStore(cfg config.Config) slackauth.Store {
	item, _ := keychain.Open(k.GOOS, slackauth.KeychainService, k.Run, user.Current, k.Getenv)

	return slackauth.Choose(k.GOOS, cfg, item)
}

// SlackRefresher refreshes the Slack user token of the app clientID names,
// through do, at the address SlackAPI says.
func (e Environment) SlackRefresher(clientID string, do httpx.Doer) slackauth.Refresher {
	base, toSlack := e.SlackAPI(do)

	return slackauth.Refresher{Do: toSlack, Base: base, ClientID: clientID, Now: time.Now}
}

// SlackToken hands out cfg's Slack user token, refreshing it through do when it
// is about to run out, under a lock every workflow on the machine shares. Its
// failures read as the messaging client's: none set up is no credential, a
// Slack not reached is unreachable, a wait Slack asked for is a rate limit,
// and a refresh Slack refused is a credential it would not accept.
func (e Environment) SlackToken(cfg config.Config, do httpx.Doer) messaging.TokenSource {
	source := slackauth.Source{
		Store: e.SlackStore(cfg), Refresher: e.SlackRefresher(cfg.Messaging.ClientID, do),
		Lock: slackauth.FileLock(e.slackLockPath(cfg.Path)), Now: time.Now,
	}

	return func(ctx context.Context, expired config.Secret) (config.Secret, error) {
		token, err := source.Token(ctx, expired)

		return token, asMessagingError(err)
	}
}

// asMessagingError is a token source's failure as the messaging client's error.
// A lock held elsewhere, and a wait Slack asked for, are no refusal of the
// token, so they keep their own words; a status Slack gave no verdict with is
// the messaging client's unexpected status.
func asMessagingError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, slackauth.ErrNotLoggedIn), errors.Is(err, slackauth.ErrNotKept):
		return fmt.Errorf("%w: %w", messaging.ErrNoCredential, err)
	case errors.Is(err, slackauth.ErrLocked), errors.Is(err, httpx.ErrRateLimited):
		return err
	case errors.Is(err, slackauth.ErrUnreachable):
		return fmt.Errorf("%w: %w", messaging.ErrUnreachable, err)
	case errors.Is(err, slackauth.ErrUnexpectedStatus):
		return fmt.Errorf("%w: %w", messaging.ErrUnexpectedStatus, err)
	default:
		return fmt.Errorf("%w: %w", messaging.ErrRejected, err)
	}
}

// slackLockPath is where the refresh lock lives: beside the store, or beside
// the configuration file where the store has no directory.
func (e Environment) slackLockPath(configPath string) string {
	dir, err := e.StateDir()
	if err != nil || dir == "" {
		dir = filepath.Dir(configPath)
	}

	return filepath.Join(dir, slackLockName)
}

// PlaceSlackCredentials is Controls.PlaceSlackCredentials with system's
// keychain, refreshing through transport. Where the keychain keeps a Slack
// user token's secrets, those typed into Settings are spent on a refresh at
// once and the new pair kept there, and the configuration it answers holds
// none of them; elsewhere it answers the configuration as it was, for the
// file to keep them.
func (e Environment) PlaceSlackCredentials(
	ctx context.Context, transport httpx.Doer, system Keychain,
) func(config.Config) (config.Config, error) {
	return func(cfg config.Config) (config.Config, error) {
		if system.GOOS != "darwin" {
			return cfg, nil
		}

		without := cfg
		without.Messaging.ClientSecret, without.Messaging.RefreshToken = "", ""
		without.Messaging.AccessToken, without.Messaging.ExpiresAt = "", ""

		starting := keptUnlessTyped(ctx, system.SlackStore(without), cfg.Messaging)

		// Slack spends the refresh token as it answers, so the refresh and the
		// keep run whatever becomes of the caller: one who left between them
		// would lose the only pair that still works.
		renewing := context.WithoutCancel(ctx)

		renewed, err := e.SlackRefresher(cfg.Messaging.ClientID, transport).Refresh(renewing, starting)
		if err != nil {
			return config.Config{}, asMessagingError(err)
		}

		err = system.SlackStore(without).Keep(renewing, renewed)
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

// slackUserClient is the client a directory reads with: built from the
// settings in effect, and messaging.ErrNoCredential while they post with no
// Slack user token — a webhook, Teams or Discord cannot read the directory.
// Only Slack has a user-token mode, so the mode alone says the service is
// Slack.
func slackUserClient(setup messagingSetup) func() (messaging.Client, error) {
	return func() (messaging.Client, error) {
		if setup.settings().Mode() != config.MessagingUser {
			return messaging.Client{}, messaging.ErrNoCredential
		}

		return messagingClient(setup), nil
	}
}

// withDirectory is bound with the directory's reads.
func withDirectory(ctx context.Context, bound seams.Messaging, slack *directory.Slack) seams.Messaging {
	bound.ChannelMembers = func(channel string) ([]loop.SlackTarget, error) { return slack.ChannelMembers(ctx, channel) }
	bound.UserGroups = func() ([]loop.SlackTarget, error) { return slack.UserGroups(ctx) }
	bound.RefreshDirectory = slack.Refresh
	bound.Workspace = func() (string, error) { return slack.Workspace(ctx) }
	bound.Grant = func() (messaging.Grant, error) { return slack.Grant(ctx) }

	return bound
}
