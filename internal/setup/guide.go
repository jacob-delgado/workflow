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
// the transport Jira is checked over, and the OS keychain, nil where none is
// wired.
type Guide struct {
	Where       Where
	Doer        jira.Doer
	StoreSecret func(secret string) (string, error)
}

// Destination is a place a file may go, and the file it would be.
type Destination struct {
	Place Place
	Path  string
}

// Offer is what a form offers: where the file may go, the repository first,
// and whether the keychain can keep the token.
type Offer struct {
	Places   []Destination
	Keychain bool
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

// Offer is where the file may go here, and whether the keychain is wired.
func (g Guide) Offer() Offer {
	places := g.Where.Places()

	destinations := make([]Destination, 0, len(places))
	for _, place := range places {
		destinations = append(destinations, Destination{Place: place, Path: g.Where.Path(place)})
	}

	return Offer{Places: destinations, Keychain: g.StoreSecret != nil}
}

// Check asks Jira who the token in settings authenticates as.
func (g Guide) Check(ctx context.Context, settings config.Jira) (string, error) {
	return Check(ctx, g.Doer, settings)
}

// Write writes the file request asks for, unless one is already there: the
// answers over the home file it lies over, the token in the keychain when
// asked. It checks nothing; a form checks the token first and says how that
// went. Everything that can refuse the write does before the keychain is
// touched, and the file is made before the keychain keeps the token, so a
// write that fails leaves the keychain as it was.
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
		cfg.Jira.Token = ""
	}

	err = Create(path, layers, cfg, over)
	if err != nil {
		return Written{}, err
	}

	if keychain {
		err = g.keepInKeychain(path, cfg, request.Answers.Jira.Token)
		if err != nil {
			return Written{}, err
		}
	}

	return Written{Path: path, Keychain: keychain, NotIgnored: NotIgnored(ctx, path)}, nil
}

// keychainFor reports whether the keychain keeps the token request asks it
// to: never with Jira left out or no token to keep, refused where there is no
// keychain, and refused for a file other than the home directory's, the one
// file that may read it back.
func (g Guide) keychainFor(request Request) (bool, error) {
	if !request.Keychain || request.Answers.Jira.BaseURL == "" || request.Answers.Jira.Token == "" {
		return false, nil
	}

	if g.StoreSecret == nil {
		return false, ErrNoKeychain
	}

	if !g.Where.IsHomeFile(request.Place) {
		return false, ErrKeychainAtHome
	}

	return true, nil
}

// keepInKeychain keeps token in the keychain and points the home file just
// made at path, holding cfg, at it. The file goes again when the keychain
// does not keep the token, so a failed setup leaves neither behind.
func (g Guide) keepInKeychain(path string, cfg config.Config, token config.Secret) error {
	cfg.Jira.Token = token

	settings, err := Keep(g.StoreSecret, cfg.Jira)
	if err != nil {
		return errors.Join(err, os.Remove(path))
	}

	cfg.Jira = settings

	return config.Save(path, cfg)
}
