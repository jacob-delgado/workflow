// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slackauth_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/slackauth"
)

// memory is a Store kept in memory, safe to share between goroutines, counting
// its saves.
type memory struct {
	lock  sync.Mutex
	held  slackauth.Credentials
	saves int
}

func (m *memory) store() slackauth.Store {
	return slackauth.NewStore("memory",
		func(context.Context) (slackauth.Credentials, error) {
			m.lock.Lock()
			defer m.lock.Unlock()

			return m.held, nil
		},
		func(_ context.Context, credentials slackauth.Credentials) error {
			m.lock.Lock()
			defer m.lock.Unlock()

			m.held, m.saves = credentials, m.saves+1

			return nil
		})
}

// sourceOver is a Source over held, refreshing through server, locking with a
// lock file in a directory of the test's own.
func sourceOver(t *testing.T, held *memory, refresher slackauth.Refresher) slackauth.Source {
	t.Helper()

	return slackauth.Source{
		Store: held.store(), Refresher: refresher, Now: testNow,
		Lock: slackauth.FileLock(filepath.Join(t.TempDir(), "slack.lock")),
	}
}

// pairExpiringIn is the starting pair with its access token expiring after left.
func pairExpiringIn(left time.Duration) slackauth.Credentials {
	pair := startingPair()
	pair.ExpiresAt = testNow().Add(left)

	return pair
}

func TestATokenWithTimeLeftIsUsedAsItIs(t *testing.T) {
	t.Parallel()

	// Arrange
	server, seen := fakeSlack(t, rotated)
	held := &memory{held: pairExpiringIn(time.Hour)}

	// Act
	token, err := sourceOver(t, held, refresherFor(server)).Token(t.Context(), "")

	// Assert
	if err != nil || token != oldAccess || len(seen()) != 0 {
		t.Errorf("Token = %q, %v after %d refreshes; want the held token and none", token.Reveal(), err, len(seen()))
	}
}

func TestATokenAboutToExpireIsRefreshedAndKept(t *testing.T) {
	t.Parallel()

	// Arrange
	server, _ := fakeSlack(t, rotated)
	held := &memory{held: pairExpiringIn(5 * time.Minute)}

	// Act
	token, err := sourceOver(t, held, refresherFor(server)).Token(t.Context(), "")

	// Assert
	if err != nil || token != newAccess {
		t.Fatalf("Token = %q, %v; want the refreshed token", token.Reveal(), err)
	}

	if held.held.RefreshToken != newRefresh || held.saves != 1 {
		t.Errorf("kept %+v after %d saves, want the new pair saved once", held.held, held.saves)
	}
}

func TestATokenSlackCalledExpiredIsRefreshedThoughItHasTimeLeft(t *testing.T) {
	t.Parallel()

	// Arrange
	server, _ := fakeSlack(t, rotated)
	held := &memory{held: pairExpiringIn(time.Hour)}

	// Act
	token, err := sourceOver(t, held, refresherFor(server)).Token(t.Context(), oldAccess)

	// Assert
	if err != nil || token != newAccess {
		t.Errorf("Token = %q, %v; want a refresh past the token Slack refused", token.Reveal(), err)
	}
}

func TestTwoAskingAtOnceRefreshOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	server, seen := fakeSlack(t, rotated)
	held := &memory{held: pairExpiringIn(time.Minute)}
	source := sourceOver(t, held, refresherFor(server))

	var (
		wait   sync.WaitGroup
		tokens [2]config.Secret
	)

	// Act
	for index := range tokens {
		wait.Go(func() {
			tokens[index], _ = source.Token(t.Context(), "")
		})
	}

	wait.Wait()

	// Assert
	if len(seen()) != 1 || tokens[0] != newAccess || tokens[1] != newAccess {
		t.Errorf("%d refreshes gave %q and %q, want one, both using its token",
			len(seen()), tokens[0].Reveal(), tokens[1].Reveal())
	}
}
