// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slackauth_test

// The checks that keep a malformed answer from Slack from being kept as the
// user token's credentials, keep Slack's words out of an error unless they
// are a code, and stop a refresh at the first thing that fails.

import (
	"context"
	"errors"
	"os/user"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/keychain"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/slackauth"
)

// noReason is what an error says in place of a code Slack's answer may not
// hold.
const noReason = "no reason given"

// errStoreUnread is a store that could not be read.
var errStoreUnread = errors.New("the store could not be read")

// errLockFailed is a lock that could not be taken for a reason of its own.
var errLockFailed = errors.New("the lock could not be taken")

func TestARefreshAnswerLeavingOutThePairOrItsLifetimeIsRefused(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"no access token":  strings.Replace(rotated, `"access_token":"`+newAccess+`"`, `"access_token":""`, 1),
		"no refresh token": strings.Replace(rotated, `"refresh_token":"`+newRefresh+`"`, `"refresh_token":""`, 1),
		"no lifetime":      strings.Replace(rotated, `"expires_in":43200`, `"expires_in":0`, 1),
	}

	for name, answer := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			server, _ := fakeSlack(t, answer)

			// Act
			renewed, err := refresherFor(server).Refresh(t.Context(), startingPair())

			// Assert
			if !errors.Is(err, slackauth.ErrRefreshRefused) || renewed != (slackauth.Credentials{}) {
				t.Errorf("Refresh = %+v, %v; want ErrRefreshRefused and nothing to keep", renewed, err)
			}
		})
	}
}

func TestARefreshShowsOnlyACodeOfSlacksAnswer(t *testing.T) {
	t.Parallel()

	tooLong := strings.Repeat("x", 70)
	cases := map[string]struct {
		answer, held string
	}{
		"an error holding an escape": {
			answer: `{"ok":false,"error":"bad\u001b[31m_code"}`, held: "\x1b",
		},
		"an error past a code's length": {answer: `{"ok":false,"error":"` + tooLong + `"}`, held: tooLong},
		"no error at all":               {answer: `{"ok":false}`, held: "error"},
		"a token type that is no code": {
			answer: strings.Replace(rotated, `"token_type":"user"`, `"token_type":"Bot Token"`, 1), held: "Bot Token",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			server, _ := fakeSlack(t, tt.answer)

			// Act
			_, err := refresherFor(server).Refresh(t.Context(), startingPair())

			// Assert
			if err == nil || !strings.Contains(err.Error(), noReason) || strings.Contains(err.Error(), tt.held) {
				t.Errorf("Refresh = %v; want %q and none of Slack's %q", err, noReason, tt.held)
			}
		})
	}
}

func TestARefreshWhoseAnswerIsNotJSONNeverReachedSlack(t *testing.T) {
	t.Parallel()

	// Arrange
	server, _ := fakeSlack(t, `<html>a proxy's page</html>`)

	// Act
	_, err := refresherFor(server).Refresh(t.Context(), startingPair())

	// Assert
	if !errors.Is(err, slackauth.ErrUnreachable) || strings.Contains(err.Error(), "proxy") {
		t.Errorf("Refresh = %v; want ErrUnreachable without the page's words", err)
	}
}

func TestARefreshToAnAddressThatIsNoneAsksNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	refresher := slackauth.Refresher{Base: "http://slack.example\x7f", ClientID: clientID, Now: testNow}

	// Act
	_, err := refresher.Refresh(t.Context(), startingPair())

	// Assert
	if !errors.Is(err, slackauth.ErrUnreachable) || strings.Contains(err.Error(), clientSecret) {
		t.Errorf("Refresh = %v; want ErrUnreachable with no credential in it", err)
	}
}

// storeFailing is a store over held whose loads fail from the one numbered
// failingFrom on, counted from one; zero never fails.
func storeFailing(held *memory, failingFrom int) slackauth.Store {
	inner := held.store()
	loads := 0

	return slackauth.NewStore("failing",
		func(ctx context.Context) (slackauth.Credentials, error) {
			loads++
			if failingFrom > 0 && loads >= failingFrom {
				return slackauth.Credentials{}, errStoreUnread
			}

			return inner.Load(ctx)
		},
		inner.Save)
}

func TestATokenStopsAtTheFirstStepThatFails(t *testing.T) {
	t.Parallel()

	takeLock := func(context.Context) (func(), error) { return func() {}, nil }
	refuseLock := func(context.Context) (func(), error) { return nil, errLockFailed }

	cases := map[string]struct {
		failingLoad int
		lock        func(context.Context) (func(), error)
		answer      string
		want        error
	}{
		"the first read":        {failingLoad: 1, lock: takeLock, answer: rotated, want: errStoreUnread},
		"the lock":              {lock: refuseLock, answer: rotated, want: errLockFailed},
		"the read under a lock": {failingLoad: 2, lock: takeLock, answer: rotated, want: errStoreUnread},
		"the refresh": {
			lock: takeLock, answer: `{"ok":false,"error":"invalid_refresh_token"}`, want: slackauth.ErrRefreshRefused,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			server, _ := fakeSlack(t, tt.answer)
			held := &memory{held: pairExpiringIn(0)}
			source := slackauth.Source{
				Store: storeFailing(held, tt.failingLoad), Refresher: refresherFor(server), Lock: tt.lock, Now: testNow,
			}

			// Act
			token, err := source.Token(t.Context(), "")

			// Assert
			if !errors.Is(err, tt.want) || token != "" || held.saves != 0 {
				t.Errorf("Token = %q, %v, %d saves; want %v and nothing kept", token.Reveal(), err, held.saves, tt.want)
			}
		})
	}
}

func TestAPairWithNoAccessTokenIsRefreshed(t *testing.T) {
	t.Parallel()

	// Arrange
	server, seen := fakeSlack(t, rotated)
	pair := pairExpiringIn(time.Hour)
	pair.AccessToken = ""
	held := &memory{held: pair}

	// Act
	token, err := sourceOver(t, held, refresherFor(server)).Token(t.Context(), "")

	// Assert
	if err != nil || token != newAccess || len(seen()) != 1 {
		t.Errorf("Token = %q, %v after %d refreshes; want one refresh's new token", token.Reveal(), err, len(seen()))
	}
}

// keychainAnswering is a macOS keychain item whose security answers a read
// with out and exits with code, failing when code is not zero.
func keychainAnswering(t *testing.T, out string, code int) keychain.Item {
	t.Helper()

	run := func(ctx context.Context, _ proc.Command, _ []byte) ([]byte, error) {
		if code == 0 {
			return []byte(out + "\n"), nil
		}

		return proc.Capture(ctx, proc.Command{Name: "sh", Args: []string{"-c", "exit " + strconv.Itoa(code)}}, nil)
	}

	item, ok := keychain.Open("darwin", slackauth.KeychainService, run,
		func() (*user.User, error) { return &user.User{Username: "fred"}, nil }, func(string) string { return "" })
	if !ok {
		t.Fatal("no keychain item on darwin")
	}

	return item
}

func TestTheKeychainStoreSaysWhyItHoldsNoPair(t *testing.T) {
	t.Parallel()

	const itemNotFound = 44

	cases := map[string]struct {
		item func(t *testing.T) keychain.Item
		want error
	}{
		"no entry": {
			item: func(t *testing.T) keychain.Item {
				t.Helper()

				return keychainAnswering(t, "", itemNotFound)
			},
			want: slackauth.ErrNotLoggedIn,
		},
		"an entry that is not workflow's": {
			item: func(t *testing.T) keychain.Item {
				t.Helper()

				return keychainAnswering(t, "not json", 0)
			},
			want: slackauth.ErrNotLoggedIn,
		},
		"an expiry that is no time": {
			item: func(t *testing.T) keychain.Item {
				t.Helper()

				return keychainAnswering(t,
					`{"client_secret":"s","refresh_token":"xoxe-1-x","expires_at":"next tuesday"}`, 0)
			},
			want: slackauth.ErrNotLoggedIn,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := slackauth.KeychainStore(tt.item(t)).Load(t.Context())

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("Load = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestTheKeychainStoreReportsAKeychainItCannotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	item := keychainAnswering(t, "", 1)

	// Act
	_, err := slackauth.KeychainStore(item).Load(t.Context())

	// Assert
	if err == nil || errors.Is(err, slackauth.ErrNotLoggedIn) {
		t.Errorf("Load = %v; want the keychain's own failure, not a login to make", err)
	}
}

func TestAFileStoreKeepingAnExpiryThatIsNoTimeSaysToLogIn(t *testing.T) {
	t.Parallel()

	// Arrange
	path := configFile(t, `{"messaging": {"kind": "slack", "client_id": "`+clientID+`", "client_secret": "s",`+
		` "refresh_token": "xoxe-1-x", "expires_at": "next tuesday"}}`)

	// Act
	_, err := slackauth.FileStore(config.Files{Home: path}).Load(t.Context())

	// Assert
	if !errors.Is(err, slackauth.ErrNotLoggedIn) {
		t.Errorf("Load = %v, want ErrNotLoggedIn", err)
	}
}
