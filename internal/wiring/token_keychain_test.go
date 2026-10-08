// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// The keychain keeps a Jira token for each address, so a repository whose
// file points Jira somewhere else reads the token kept for that address and
// never the one kept for the home file's. These tests run as macOS would, over
// a fake security, so no test reads the keychain of the machine it runs on.

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/keychain"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// The two Jira addresses, the keychain items they name, and the tokens kept
// for each.
const (
	homeJira       = "https://jira.example.com"
	secondJira     = "https://jira.other.example"
	homeItem       = "workflow-jira " + homeJira
	secondItem     = "workflow-jira " + secondJira
	homeToken      = "home-keychain-token-1111"
	secondToken    = "second-keychain-token-2222"
	homeFileToken  = "home-file-token-3333"
	itemNotFound   = 44
	unreadableItem = 51
)

// keychainHolding is macOS's security as a read drives it: it answers
// find-generic-password with the token kept under the service asked for, and
// remembers each service asked.
type keychainHolding struct {
	lock   sync.Mutex
	tokens map[string]string
	asked  []string
}

// run answers find-generic-password -a account -s service -w.
func (k *keychainHolding) run(_ context.Context, program proc.Command, _ []byte) ([]byte, error) {
	k.lock.Lock()
	defer k.lock.Unlock()

	service := program.Args[slices.Index(program.Args, "-s")+1]
	k.asked = append(k.asked, service)

	token, kept := k.tokens[service]
	if !kept {
		return nil, &proc.ExitError{Program: "security", Code: itemNotFound, Stderr: "could not be found"}
	}

	return []byte(token + "\n"), nil
}

// services are the items asked for, in order.
func (k *keychainHolding) services() []string {
	k.lock.Lock()
	defer k.lock.Unlock()

	return slices.Clone(k.asked)
}

// macOSHolding is the keychain macOS has, holding tokens.
func macOSHolding(tokens map[string]string) (*keychainHolding, wiring.Keychain) {
	held := &keychainHolding{tokens: tokens}

	return held, wiring.Keychain{GOOS: macOS, Run: held.run}
}

// writeConfig writes contents to a configuration file in a directory of its
// own.
func writeConfig(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), config.FileName)
	write(t, path, contents, config.FileMode)

	return path
}

// movedJira is the Jira settings in effect where a repository's file points
// Jira at the second address, over a home file that reads its token from the
// keychain and holds one of its own besides.
func movedJira(t *testing.T) config.Jira {
	t.Helper()

	home := writeConfig(t, `{"jira": {"base_url": "`+homeJira+`", "keychain": true, "token": "`+homeFileToken+`"}}`)
	repo := writeConfig(t, `{"jira": {"base_url": "`+secondJira+`"}}`)

	cfg, _, err := config.LoadLayersAt(config.Files{Home: home, Repo: repo})
	if err != nil {
		t.Fatalf("loading the layers: %v", err)
	}

	return cfg.Jira
}

func TestTheHomeTokenIsNeverSentToASecondAddress(t *testing.T) {
	t.Parallel()

	// Arrange
	held, system := macOSHolding(map[string]string{homeItem: homeToken})

	// Act
	token, _, err := wiring.ResolveToken(t.Context(), movedJira(t), system)

	// Assert
	if !errors.Is(err, keychain.ErrNotStored) || token != "" {
		t.Errorf("ResolveToken = %q, %v; want no token, the keychain holding none for the second address",
			token, err)
	}

	if asked := held.services(); !slices.Equal(asked, []string{secondItem}) {
		t.Errorf("the keychain was asked for %q, want only the second address's item", asked)
	}
}

func TestASecondAddressReadsTheTokenKeptForIt(t *testing.T) {
	t.Parallel()

	// Arrange
	_, system := macOSHolding(map[string]string{homeItem: homeToken, secondItem: secondToken})

	// Act
	token, source, err := wiring.ResolveToken(t.Context(), movedJira(t), system)

	// Assert
	if err != nil || token != secondToken {
		t.Errorf("ResolveToken = %q, %v; want the token kept for the second address", token, err)
	}

	if !strings.Contains(source, "keychain") || !strings.Contains(source, secondJira) {
		t.Errorf("the token's source is %q, want the keychain item for the second address named", source)
	}
}

func TestResolveTokenTakesTheKeychainAfterTheFileAndBeforeTheOtherSources(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		settings config.Jira
		want     string
	}{
		"the file's token wins": {
			settings: config.Jira{BaseURL: homeJira, Token: homeFileToken, Keychain: true},
			want:     homeFileToken,
		},
		"the keychain over a command": {
			settings: config.Jira{BaseURL: homeJira, Keychain: true, TokenCommand: failingCommand},
			want:     homeToken,
		},
		"the keychain over a variable": {
			settings: config.Jira{BaseURL: homeJira, Keychain: true, TokenEnv: unsetVariable},
			want:     homeToken,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			_, system := macOSHolding(map[string]string{homeItem: homeToken})

			// Act
			token, _, err := wiring.ResolveToken(t.Context(), tt.settings, system)

			// Assert
			if err != nil || token.Reveal() != tt.want {
				t.Errorf("ResolveToken = %q, %v; want %q", token, err, tt.want)
			}
		})
	}
}

func TestAKeychainWhereNoneIsWiredIsNoToken(t *testing.T) {
	t.Parallel()

	// Arrange
	held := &keychainHolding{tokens: map[string]string{homeItem: homeToken}}
	elsewhere := wiring.Keychain{GOOS: noKeychainOS, Run: held.run}

	// Act
	token, _, err := wiring.ResolveToken(t.Context(), config.Jira{BaseURL: homeJira, Keychain: true}, elsewhere)

	// Assert
	if !errors.Is(err, keychain.ErrNotWired) || token != "" || len(held.services()) != 0 {
		t.Errorf("ResolveToken = %q, %v after asking %q; want ErrNotWired, asking nothing",
			token, err, held.services())
	}
}

func TestAnUnreadableKeychainItemIsToldWithoutWhatSecurityPrinted(t *testing.T) {
	t.Parallel()

	// Arrange
	printed := func(context.Context, proc.Command, []byte) ([]byte, error) {
		return nil, &proc.ExitError{Program: "security", Code: unreadableItem, Stderr: "SECRET-VALUE nearby"}
	}
	system := wiring.Keychain{GOOS: macOS, Run: printed}

	// Act
	_, _, err := wiring.ResolveToken(t.Context(), config.Jira{BaseURL: homeJira, Keychain: true}, system)

	// Assert
	if err == nil || strings.Contains(err.Error(), "SECRET-VALUE") {
		t.Errorf("ResolveToken = %v, want the failure told without what security printed", err)
	}
}

func TestATokenKeptOnMacOSGoesToTheItemNamed(t *testing.T) {
	t.Parallel()

	// Arrange
	security := &fakeSecurity{}
	keep := onMacOS(security).JiraTokenKeeper(t.Context())

	// Act
	err := keep(secondItem, secondToken)

	// Assert
	if err != nil || security.held() != secondToken || !strings.Contains(security.line(), `-s "`+secondItem+`"`) {
		t.Errorf("keep = %v, the keychain holds the token %t in %q; want it under the item named",
			err, security.held() == secondToken, security.line())
	}
}

func TestNoTokenKeeperIsWiredWhereThereIsNoKeychain(t *testing.T) {
	t.Parallel()

	// Act
	keep := wiring.Keychain{GOOS: noKeychainOS, Run: (&fakeSecurity{}).run}.JiraTokenKeeper(t.Context())

	// Assert
	if keep != nil {
		t.Error("a keeper is wired where workflow drives no keychain, want none")
	}
}
