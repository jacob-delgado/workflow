// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// Slack user-token secrets typed into Settings on macOS are spent on a
// refresh at once and the new pair kept in the keychain, so the file saved
// holds none of them. These tests run as macOS would, over a fake security
// and a fake Slack, so no test touches the keychain of the machine it runs on.

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/slackauth"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// errKeychainLocked is a keychain that would not keep what it was handed.
var errKeychainLocked = errors.New("the keychain is locked")

// The Slack app, and the secrets typed, kept before, and given by Slack's
// refresh.
const (
	placedClientID = "9999.1111"
	typedSecret    = "client-secret-typed-7777"
	typedRefresh   = "xoxe-1-typed-refresh"
	keptRefresh    = "xoxe-1-kept-refresh"
	renewedRefresh = "xoxe-1-renewed-refresh"
	renewedAccess  = "xoxe.xoxp-1-renewed-access"
)

// fakeSecurity is macOS's security as the keychain drives it: it keeps the
// line each store hands it and reads the secret back.
type fakeSecurity struct {
	lock   sync.Mutex
	kept   string
	stored string
}

// run answers security's two commands: -i, which reads a store from its
// input, and find-generic-password, which prints the secret kept.
func (s *fakeSecurity) run(_ context.Context, program proc.Command, input []byte) ([]byte, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if program.Args[0] == "-i" {
		line := string(input)
		s.stored = line
		start := strings.Index(line, ` -w "`) + len(` -w "`)
		s.kept = strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(strings.TrimSuffix(line[start:], "\"\n"))

		return nil, nil
	}

	return []byte(s.kept + "\n"), nil
}

// line is the last line a store handed the fake.
func (s *fakeSecurity) line() string {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.stored
}

// held is the secret the fake keeps.
func (s *fakeSecurity) held() string {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.kept
}

// slackRefreshes is a Slack that answers every refresh with answer, keeping the
// refresh token each was made with.
type slackRefreshes struct {
	answer string
	spent  []string
}

// do answers request as Slack's oauth.v2.access would.
func (s *slackRefreshes) do(request *http.Request) (*http.Response, error) {
	raw, _ := io.ReadAll(request.Body)
	form, _ := url.ParseQuery(string(raw))
	s.spent = append(s.spent, form.Get("refresh_token"))

	return &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(s.answer)), Request: request,
	}, nil
}

// renewedPair is Slack's answer to a refresh of a user token.
const renewedPair = `{"ok":true,"access_token":"` + renewedAccess + `","token_type":"user","expires_in":43200,` +
	`"refresh_token":"` + renewedRefresh + `"}`

// typedInto is a Slack configuration with secret and refresh typed into it.
func typedInto(secret, refresh config.Secret) config.Config {
	cfg := config.Default()
	cfg.Messaging = config.Messaging{
		Kind: config.KindSlack, ClientID: placedClientID, Channel: "#dev", ClientSecret: secret, RefreshToken: refresh,
	}

	return cfg
}

// macOS is the system whose keychain workflow drives, and noKeychainOS one
// whose keychain it does not.
const (
	macOS        = "darwin"
	noKeychainOS = "freebsd"
)

// keptPair is the user token's secrets the keychain already keeps.
const keptPair = `{"client_secret":"client-secret-kept","refresh_token":"` + keptRefresh + `"}`

// onMacOS is the keychain macOS has, run through security.
func onMacOS(security *fakeSecurity) wiring.Keychain {
	return wiring.Keychain{GOOS: macOS, Run: security.run}
}

func TestPlacingTypedSlackSecretsKeepsTheRenewedPairInTheKeychain(t *testing.T) {
	t.Parallel()

	// Arrange
	slack := &slackRefreshes{answer: renewedPair}
	security := &fakeSecurity{}
	place := wiring.PlaceSlackCredentials(t.Context(), slack.do, onMacOS(security))

	// Act
	placed, err := place(typedInto(typedSecret, typedRefresh))

	// Assert
	if err != nil || placed.Messaging.HoldsUserTokenSecrets() || placed.Messaging.ClientID != placedClientID {
		t.Fatalf("placed %+v, %v; want the secrets out of the configuration and the rest kept",
			placed.Redacted().Messaging, err)
	}

	if !strings.Contains(security.held(), renewedRefresh) || !strings.Contains(security.held(), typedSecret) ||
		len(slack.spent) != 1 || slack.spent[0] != typedRefresh {
		t.Errorf("the keychain holds %q after refreshing with %q; want the typed refresh token spent and "+
			"the renewed pair kept with the client secret", security.held(), slack.spent)
	}
}

func TestPlacingABlankTypedRefreshTokenTakesTheOneTheKeychainKeeps(t *testing.T) {
	t.Parallel()

	// Arrange
	slack := &slackRefreshes{answer: renewedPair}
	security := &fakeSecurity{kept: keptPair}
	place := wiring.PlaceSlackCredentials(t.Context(), slack.do, onMacOS(security))

	// Act
	_, err := place(typedInto(typedSecret, ""))

	// Assert
	if err != nil || len(slack.spent) != 1 || slack.spent[0] != keptRefresh ||
		!strings.Contains(security.held(), typedSecret) {
		t.Errorf("Place = %v after refreshing with %q, keychain %q; want the kept refresh token spent beside "+
			"the typed client secret", err, slack.spent, security.held())
	}
}

func TestPlacingSecretsSlackRefusesIsARejectionWithNoSecretInIt(t *testing.T) {
	t.Parallel()

	// Arrange
	slack := &slackRefreshes{answer: `{"ok":false,"error":"invalid_refresh_token"}`}
	security := &fakeSecurity{}
	place := wiring.PlaceSlackCredentials(t.Context(), slack.do, onMacOS(security))

	// Act
	_, err := place(typedInto(typedSecret, typedRefresh))

	// Assert
	if !errors.Is(err, messaging.ErrRejected) || strings.Contains(err.Error(), typedSecret) ||
		strings.Contains(err.Error(), typedRefresh) || security.held() != "" {
		t.Errorf("Place = %v, keychain %q; want a rejection naming no secret, and nothing kept", err, security.held())
	}
}

func TestPlacingSlackSecretsOffMacOSLeavesThemInTheConfiguration(t *testing.T) {
	t.Parallel()

	// Arrange
	slack := &slackRefreshes{answer: renewedPair}
	security := &fakeSecurity{}
	place := wiring.PlaceSlackCredentials(t.Context(), slack.do, wiring.Keychain{GOOS: noKeychainOS, Run: security.run})
	typed := typedInto(typedSecret, typedRefresh)

	// Act
	placed, err := place(typed)

	// Assert
	if err != nil || !reflect.DeepEqual(placed.Messaging, typed.Messaging) || len(slack.spent) != 0 {
		t.Errorf("placed %+v, %v after %d refreshes; want the configuration as it was, nothing spent",
			placed.Redacted().Messaging, err, len(slack.spent))
	}
}

func TestPlacingABlankTypedClientSecretTakesTheOneTheKeychainKeeps(t *testing.T) {
	t.Parallel()

	// Arrange
	slack := &slackRefreshes{answer: renewedPair}
	security := &fakeSecurity{kept: keptPair}
	place := wiring.PlaceSlackCredentials(t.Context(), slack.do, onMacOS(security))

	// Act
	_, err := place(typedInto("", typedRefresh))

	// Assert
	if err != nil || !strings.Contains(security.held(), "client-secret-kept") ||
		!strings.Contains(security.held(), renewedRefresh) {
		t.Errorf("Place = %v, keychain %q; want the kept client secret kept with the renewed pair",
			err, security.held())
	}
}

func TestPlacingAPairTheKeychainWillNotKeepIsReported(t *testing.T) {
	t.Parallel()

	// Arrange
	slack := &slackRefreshes{answer: renewedPair}
	refusing := func(context.Context, proc.Command, []byte) ([]byte, error) { return nil, errKeychainLocked }
	place := wiring.PlaceSlackCredentials(t.Context(), slack.do, wiring.Keychain{GOOS: macOS, Run: refusing})

	// Act
	_, err := place(typedInto(typedSecret, typedRefresh))

	// Assert
	if !errors.Is(err, slackauth.ErrNotKept) || strings.Contains(err.Error(), typedSecret) {
		t.Errorf("Place = %v; want ErrNotKept with no secret in it", err)
	}
}
