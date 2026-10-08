// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"context"
	"errors"
	"os"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// Guide is setup as a form asks for it, all answers at once: where it runs,
// the transport Jira is checked over, and the OS keychain, keeping a secret
// under the item named, nil where none is wired.
type Guide struct {
	Where       Where
	Doer        jira.Doer
	StoreSecret func(service, secret string) error
}

// Destination is a place a file may go, the file it would be, and whether
// the keychain can keep the token for that file: wherever one is wired, since
// any file may read the item kept for its Jira address.
type Destination struct {
	Place    Place
	Path     string
	Keychain bool
}

// Offer is what a form offers: where the file may go, the repository first.
type Offer struct {
	Places []Destination
}

// Request is a setup a form asks for: where, the answers, and whether the
// token goes to the keychain.
type Request struct {
	Place    Place
	Answers  Answers
	Keychain bool
}

// Written is what a setup wrote: the file, whether the keychain keeps the
// token, and whether git would let the file be committed.
type Written struct {
	Path       string
	Keychain   bool
	NotIgnored bool
}

// Offer is where the file may go here, and whether the keychain can keep the
// token for each.
func (g Guide) Offer() Offer {
	places := g.Where.Places()

	destinations := make([]Destination, 0, len(places))
	for _, place := range places {
		destinations = append(destinations, Destination{
			Place: place, Path: g.Where.Path(place), Keychain: g.StoreSecret != nil,
		})
	}

	return Offer{Places: destinations}
}

// Check asks Jira who the token in settings authenticates as.
func (g Guide) Check(ctx context.Context, settings config.Jira) (string, error) {
	return Check(ctx, g.Doer, settings)
}

// Write writes the file request asks for, unless one is already there: the
// answers over the home file it lies over, the token in the keychain item for
// its address when asked. It checks nothing; a form checks the token first and
// says how that went. Everything that can refuse the write does before the
// keychain is touched, and the file is made before the keychain keeps the
// token, so a write that fails leaves the keychain as it was.
func (g Guide) Write(ctx context.Context, request Request) (Written, error) {
	if request.Place == Home && g.Where.HomeDir == "" {
		return Written{}, ErrNoHome
	}

	path := g.Where.Path(request.Place)

	err := RefuseExisting(path)
	if err != nil {
		return Written{}, err
	}

	keychain, err := g.keychainFor(request)
	if err != nil {
		return Written{}, err
	}

	layers := g.Where.Layers(request.Place)

	beneath, over, err := Beneath(layers)
	if err != nil {
		return Written{}, err
	}

	cfg := request.Answers.Over(beneath)
	if keychain {
		cfg.Jira = inKeychain(cfg.Jira)
	}

	err = Create(path, layers, cfg, over)
	if err != nil {
		return Written{}, err
	}

	if keychain {
		err = g.keepInKeychain(path, request.Answers.Jira)
		if err != nil {
			return Written{}, err
		}
	}

	return Written{Path: path, Keychain: keychain, NotIgnored: NotIgnored(ctx, path)}, nil
}

// keychainFor reports whether the keychain keeps the token request asks it
// to: never with Jira left out or no token to keep, and refused where there
// is no keychain.
func (g Guide) keychainFor(request Request) (bool, error) {
	if !request.Keychain || request.Answers.Jira.BaseURL == "" || request.Answers.Jira.Token == "" {
		return false, nil
	}

	if g.StoreSecret == nil {
		return false, ErrNoKeychain
	}

	return true, nil
}

// keepInKeychain keeps the token answered in the keychain item for its
// address, which the file just made at path reads it from. The file goes
// again when the keychain does not keep the token, so a failed setup leaves
// neither behind.
func (g Guide) keepInKeychain(path string, answered config.Jira) error {
	_, err := Keep(g.StoreSecret, answered)
	if err != nil {
		return errors.Join(err, os.Remove(path))
	}

	return nil
}
