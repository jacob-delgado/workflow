// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slackauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
)

// refreshAhead is how long before it expires a token is refreshed: soon enough
// that a post made just before is not refused, and late enough that a token is
// used for most of its twelve hours.
const refreshAhead = 10 * time.Minute

// ErrNoClientID reports a user token's secrets kept with no app named to
// refresh them for: a login writes the two together, so only a login made
// again names it.
var ErrNoClientID = errors.New(
	"the Slack app's client ID, messaging.client_id, is not set beside the user token's secrets")

// Source hands out the user token to post with: the one kept in Store while it
// has time left, otherwise a new one from Refresher, kept in its place. A
// refresh is made under Lock, so while each process that refreshes takes it,
// two never spend the same refresh token, which works once.
type Source struct {
	Store     Store
	Refresher Refresher
	Lock      func(ctx context.Context) (func(), error)
	Now       func() time.Time
}

// Token is a user token with more than refreshAhead left on it, and other than
// expired — the token Slack just answered token_expired for, or empty when
// none was. With no app named to refresh it for, it is ErrNoClientID, which
// is also ErrNotLoggedIn. A process that waited on the lock re-reads what the
// one before it kept, and refreshes only when that is still not good enough.
func (s Source) Token(ctx context.Context, expired config.Secret) (config.Secret, error) {
	if s.Refresher.ClientID == "" {
		return "", fmt.Errorf("%w: %w", ErrNoClientID, ErrNotLoggedIn)
	}

	held, err := s.Store.Load(ctx)
	if err != nil {
		return "", err
	}

	if s.usable(held, expired) {
		return held.AccessToken, nil
	}

	unlock, err := s.Lock(ctx)
	if err != nil {
		return "", err
	}
	defer unlock()

	held, err = s.Store.Load(ctx)
	if err != nil {
		return "", err
	}

	if s.usable(held, expired) {
		return held.AccessToken, nil
	}

	// Slack spends held's refresh token as it answers, so a store that would
	// refuse the pair refuses first, with ErrKeepRefused and nothing spent.
	// The prove spends nothing either, so it ends when the caller leaves.
	err = s.Store.Prove(ctx)
	if err != nil {
		return "", err
	}

	return s.renew(context.WithoutCancel(ctx), held)
}

// renew refreshes held and keeps the pair Slack gives in its place. Slack
// spends held's refresh token as it answers, so the two run to their end
// when the caller leaves or a signal ends the command, which between them
// would lose the only pair that still works; ctx never ends, and the two
// are bounded by the transport's timing.request_timeout and the store's own
// bound, as Settings' placement of a pair is. Its end is kept only where
// something waits for it: a command's own call does, while the terminal
// exits at once on a signal and --web waits five seconds for its requests.
func (s Source) renew(ctx context.Context, held Credentials) (config.Secret, error) {
	renewed, err := s.Refresher.Refresh(ctx, held)
	if err != nil {
		return "", err
	}

	err = s.Store.Keep(ctx, renewed)
	if err != nil {
		return "", err
	}

	return renewed.AccessToken, nil
}

// usable reports a held token with time left that is not the one Slack refused.
func (s Source) usable(held Credentials, expired config.Secret) bool {
	return held.AccessToken != "" && held.AccessToken != expired && held.ExpiresAt.Sub(s.Now()) > refreshAhead
}
