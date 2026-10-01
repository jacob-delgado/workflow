// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slackauth

import (
	"context"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
)

// refreshAhead is how long before it expires a token is refreshed: soon enough
// that a post made just before is not refused, and late enough that a token is
// used for most of its twelve hours.
const refreshAhead = 10 * time.Minute

// Source hands out the user token to post with: the one kept in Store while it
// has time left, otherwise a new one from Refresher, kept in its place. A
// refresh is made under Lock, so two processes never spend the same refresh
// token, which works once.
type Source struct {
	Store     Store
	Refresher Refresher
	Lock      func(ctx context.Context) (func(), error)
	Now       func() time.Time
}

// Token is a user token with more than refreshAhead left on it, and other than
// expired — the token Slack just answered token_expired for, or empty when
// none was. A process that waited on the lock re-reads what the one before it
// kept, and refreshes only when that is still not good enough.
func (s Source) Token(ctx context.Context, expired config.Secret) (config.Secret, error) {
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

	renewed, err := s.Refresher.Refresh(ctx, held)
	if err != nil {
		return "", err
	}

	err = s.Store.Save(ctx, renewed)
	if err != nil {
		return "", err
	}

	return renewed.AccessToken, nil
}

// usable reports a held token with time left that is not the one Slack refused.
func (s Source) usable(held Credentials, expired config.Secret) bool {
	return held.AccessToken != "" && held.AccessToken != expired && held.ExpiresAt.Sub(s.Now()) > refreshAhead
}
