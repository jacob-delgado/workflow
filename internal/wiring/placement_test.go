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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/keychain"
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

// probeItem is the keychain item a Slack token's store proves itself by.
const probeItem = "workflow-slack-probe"

// fakeSecurity is macOS's security as the keychain drives it: it keeps the
// line each store hands it and reads the secret back, and keeps the prove's
// probe item apart, deleting it as asked. Where refusing, it will store
// nothing but the probe, and where blind, it cannot read back what it has
// stored. Like the program it stands in for, it is never run for a caller
// that has left.
type fakeSecurity struct {
	lock     sync.Mutex
	kept     string
	stored   string
	probes   map[string]string
	refusing bool
	blind    bool
}

// run answers security's commands: -i, which reads a store from its input,
// find-generic-password, which prints the secret kept, and
// delete-generic-password, which forgets it.
func (s *fakeSecurity) run(ctx context.Context, program proc.Command, input []byte) ([]byte, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if ctx.Err() != nil {
		return nil, fmt.Errorf("security: %w", ctx.Err())
	}

	if program.Args[0] == "-i" {
		return nil, s.store(string(input))
	}

	service := program.Args[slices.Index(program.Args, "-s")+1]
	if service == probeItem {
		return s.probe(program.Args[0], service)
	}

	if s.blind && s.stored != "" {
		return nil, errKeychainLocked
	}

	return []byte(s.kept + "\n"), nil
}

// store keeps the secret line hands it under line's service.
func (s *fakeSecurity) store(line string) error {
	service := line[strings.Index(line, ` -s "`)+len(` -s "`) : strings.Index(line, `" -w "`)]
	start := strings.Index(line, ` -w "`) + len(` -w "`)
	secret := strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(strings.TrimSuffix(line[start:], "\"\n"))

	if service == probeItem {
		if s.probes == nil {
			s.probes = map[string]string{}
		}

		s.probes[service] = secret

		return nil
	}

	if s.refusing {
		return errKeychainLocked
	}

	s.stored, s.kept = line, secret

	return nil
}

// probe answers command for the probe item.
func (s *fakeSecurity) probe(command, service string) ([]byte, error) {
	secret, kept := s.probes[service]
	if !kept {
		return nil, &proc.ExitError{Program: securityProgram, Code: itemNotFound, Stderr: "could not be found"}
	}

	if command == "delete-generic-password" {
		delete(s.probes, service)

		return nil, nil
	}

	return []byte(secret + "\n"), nil
}

// probesLeft is how many probe items the fake keeps.
func (s *fakeSecurity) probesLeft() int {
	s.lock.Lock()
	defer s.lock.Unlock()

	return len(s.probes)
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

// keychainAccount is the login name every test's keychain keeps a secret
// under, whatever account the machine running the tests has, or none: the
// build container runs them as a uid with no passwd entry and no $USER.
const keychainAccount = "tester"

// placing is the wiring's environment a placement runs in: its refresh lock
// in a store directory of the test's own, never the developer's.
func placing(t *testing.T) wiring.Environment {
	t.Helper()

	return stateIn(stateDirAt(t.TempDir()))
}

// testUser is the user running the tests, as the keychain looks them up.
func testUser() (*user.User, error) {
	return &user.User{Username: keychainAccount}, nil
}

// noVariables is an environment that sets no variable.
func noVariables(string) string {
	return ""
}

// onMacOSRunning is the keychain macOS has, run through run, for testUser.
func onMacOSRunning(run keychain.Runner) wiring.Keychain {
	return wiring.Keychain{GOOS: macOS, Run: run, CurrentUser: testUser, Getenv: noVariables}
}

// onMacOS is the keychain macOS has, run through security.
func onMacOS(security *fakeSecurity) wiring.Keychain {
	return onMacOSRunning(security.run)
}

func TestPlacingTypedSlackSecretsKeepsTheRenewedPairInTheKeychain(t *testing.T) {
	t.Parallel()

	// Arrange
	slack := &slackRefreshes{answer: renewedPair}
	security := &fakeSecurity{}
	place := placing(t).PlaceSlackCredentials(slack.do, onMacOS(security))

	// Act
	err := place(t.Context(), typedInto(typedSecret, typedRefresh))

	// Assert
	if err != nil || !strings.Contains(security.held(), renewedRefresh) ||
		!strings.Contains(security.held(), typedSecret) || len(slack.spent) != 1 || slack.spent[0] != typedRefresh {
		t.Errorf("Place = %v, the keychain holds %q after refreshing with %q; want the typed refresh token "+
			"spent and the renewed pair kept with the client secret", err, security.held(), slack.spent)
	}

	if left := security.probesLeft(); left != 0 {
		t.Errorf("the keychain keeps %d probe items, want none left behind", left)
	}
}

func TestPlacingABlankTypedRefreshTokenTakesTheOneTheKeychainKeeps(t *testing.T) {
	t.Parallel()

	// Arrange
	slack := &slackRefreshes{answer: renewedPair}
	security := &fakeSecurity{kept: keptPair}
	place := placing(t).PlaceSlackCredentials(slack.do, onMacOS(security))

	// Act
	err := place(t.Context(), typedInto(typedSecret, ""))

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
	place := placing(t).PlaceSlackCredentials(slack.do, onMacOS(security))

	// Act
	err := place(t.Context(), typedInto(typedSecret, typedRefresh))

	// Assert
	if !errors.Is(err, messaging.ErrRejected) || strings.Contains(err.Error(), typedSecret) ||
		strings.Contains(err.Error(), typedRefresh) || security.held() != "" {
		t.Errorf("Place = %v, keychain %q; want a rejection naming no secret, and nothing kept", err, security.held())
	}
}

func TestPlacingSlackSecretsOffMacOSIsLeftToTheFile(t *testing.T) {
	t.Parallel()

	// Arrange
	slack := &slackRefreshes{answer: renewedPair}
	security := &fakeSecurity{}
	system := wiring.Keychain{GOOS: noKeychainOS, Run: security.run, Getenv: os.Getenv}

	// Act
	place := placing(t).PlaceSlackCredentials(slack.do, system)

	// Assert
	if place != nil {
		t.Error("PlaceSlackCredentials off macOS is wired; want none, so the file keeps the secrets typed")
	}
}

func TestPlacingABlankTypedClientSecretTakesTheOneTheKeychainKeeps(t *testing.T) {
	t.Parallel()

	// Arrange
	slack := &slackRefreshes{answer: renewedPair}
	security := &fakeSecurity{kept: keptPair}
	place := placing(t).PlaceSlackCredentials(slack.do, onMacOS(security))

	// Act
	err := place(t.Context(), typedInto("", typedRefresh))

	// Assert
	if err != nil || !strings.Contains(security.held(), "client-secret-kept") ||
		!strings.Contains(security.held(), renewedRefresh) {
		t.Errorf("Place = %v, keychain %q; want the kept client secret kept with the renewed pair",
			err, security.held())
	}
}

func TestPlacingIntoAKeychainThatWillNotStoreAsksSlackNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	slack := &slackRefreshes{answer: renewedPair}
	// The keychain reads what it keeps, and will store nothing.
	refusingStores := func(_ context.Context, program proc.Command, _ []byte) ([]byte, error) {
		if program.Args[0] == "-i" {
			return nil, errKeychainLocked
		}

		return []byte(keptPair + "\n"), nil
	}
	place := placing(t).PlaceSlackCredentials(slack.do, onMacOSRunning(refusingStores))

	// Act
	err := place(t.Context(), typedInto(typedSecret, typedRefresh))

	// Assert
	if !errors.Is(err, slackauth.ErrKeepRefused) || len(slack.spent) != 0 ||
		strings.Contains(err.Error(), typedSecret) || strings.Contains(err.Error(), typedRefresh) {
		t.Errorf("Place = %v after refreshing with %q; want %v, naming no secret, and the typed refresh token "+
			"not spent", err, slack.spent, slackauth.ErrKeepRefused)
	}
}

func TestPlacingAPairTheKeychainWillNotKeepIsReported(t *testing.T) {
	t.Parallel()

	// Arrange
	slack := &slackRefreshes{answer: renewedPair}
	// The keychain takes the prove's probe, then refuses the new pair.
	security := &fakeSecurity{kept: keptPair, refusing: true}
	place := placing(t).PlaceSlackCredentials(slack.do, onMacOS(security))

	// Act
	err := place(t.Context(), typedInto(typedSecret, typedRefresh))

	// Assert
	if !errors.Is(err, slackauth.ErrNotKept) || len(slack.spent) != 1 || strings.Contains(err.Error(), typedSecret) {
		t.Errorf("Place = %v after refreshing with %q; want ErrNotKept after one refresh, with no secret in it",
			err, slack.spent)
	}
}

func TestPlacingAPairTheKeychainCannotReadBackIsNotKeptRatherThanUnreadable(t *testing.T) {
	t.Parallel()

	// Arrange
	slack := &slackRefreshes{answer: renewedPair}
	// The keychain reads what it keeps and takes the new pair, then cannot
	// read it back: Slack has spent the typed refresh token by then, so the
	// pair is not kept, which saving again cannot mend as unlocking would.
	security := &fakeSecurity{kept: keptPair, blind: true}
	place := placing(t).PlaceSlackCredentials(slack.do, onMacOS(security))

	// Act
	err := place(t.Context(), typedInto(typedSecret, typedRefresh))

	// Assert
	if !errors.Is(err, slackauth.ErrNotKept) || errors.Is(err, keychain.ErrNotRead) ||
		strings.Contains(err.Error(), typedSecret) {
		t.Errorf("Place = %v; want ErrNotKept, not ErrNotRead, with no secret in it", err)
	}
}

func TestPlacingSecretsIntoAKeychainThatCannotBeReadSpendsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// A locked keychain refuses a read as it would the store after it.
	slack := &slackRefreshes{answer: renewedPair}
	refusing := func(context.Context, proc.Command, []byte) ([]byte, error) { return nil, errKeychainLocked }
	place := placing(t).PlaceSlackCredentials(slack.do, onMacOSRunning(refusing))

	// Act
	err := place(t.Context(), typedInto(typedSecret, typedRefresh))

	// Assert
	if !errors.Is(err, errKeychainLocked) || errors.Is(err, slackauth.ErrNotKept) || len(slack.spent) != 0 ||
		strings.Contains(err.Error(), typedSecret) || strings.Contains(err.Error(), typedRefresh) {
		t.Errorf("Place = %v after refreshing with %q; want the keychain's refusal, naming no secret, "+
			"before the typed refresh token is spent", err, slack.spent)
	}
}

func TestPlacingSecretsKeepsTheRenewedPairThoughItsAskerLeavesOnceSlackAnswers(t *testing.T) {
	t.Parallel()

	// Arrange
	// Slack spends the typed refresh token the moment it answers, so the pair
	// it gave is then the only one that works.
	asking, leave := context.WithCancel(t.Context())
	t.Cleanup(leave)

	slack := &slackRefreshes{answer: renewedPair}
	answeredThenLeft := func(request *http.Request) (*http.Response, error) {
		response, err := slack.do(request)

		leave()

		return response, err
	}

	security := &fakeSecurity{}
	place := placing(t).PlaceSlackCredentials(answeredThenLeft, onMacOS(security))

	// Act
	_ = place(asking, typedInto(typedSecret, typedRefresh))

	// Assert
	if !strings.Contains(security.held(), renewedRefresh) {
		t.Errorf("the keychain holds %q, want the renewed pair kept though its asker left", security.held())
	}
}

func TestPlacingSecretsWaitsForARefreshAnotherWorkflowIsMaking(t *testing.T) {
	t.Parallel()

	// Arrange
	// Another workflow holds the refresh lock while it spends the pair the
	// keychain keeps and keeps the one Slack gives back.
	dir := t.TempDir()

	unlock, err := slackauth.FileLock(filepath.Join(dir, slackLockFile))(t.Context())
	if err != nil {
		t.Fatalf("holding the lock: %v", err)
	}
	defer unlock()

	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	slack := &slackRefreshes{answer: renewedPair}
	security := &fakeSecurity{kept: keptPair}
	place := stateIn(stateDirAt(dir)).PlaceSlackCredentials(slack.do, onMacOS(security))

	// Act
	err = place(ctx, typedInto(typedSecret, ""))

	// Assert
	if !errors.Is(err, slackauth.ErrLocked) || len(slack.spent) != 0 || security.held() != keptPair {
		t.Errorf("Place = %v after refreshing with %q, keychain %q; want it to wait on the lock, "+
			"spending and keeping nothing while another workflow refreshes", err, slack.spent, security.held())
	}
}
