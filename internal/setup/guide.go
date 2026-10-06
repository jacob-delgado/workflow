// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"context"

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
	return Offer{
		Places: []Destination{
			{Place: Repository, Path: g.Where.Path(Repository)},
			{Place: Home, Path: g.Where.Path(Home)},
		},
		Keychain: g.StoreSecret != nil,
	}
}

// Check asks Jira who the token in settings authenticates as.
func (g Guide) Check(ctx context.Context, settings config.Jira) (string, error) {
	return Check(ctx, g.Doer, settings)
}

// Write writes the file request asks for, unless one is already there: the
// answers over the home file it lies over, the token in the keychain when
// asked. It checks nothing; a form checks the token first and says how that
// went.
func (g Guide) Write(ctx context.Context, request Request) (Written, error) {
	path := g.Where.Path(request.Place)

	err := RefuseExisting(path)
	if err != nil {
		return Written{}, err
	}

	answers := request.Answers
	keychain := request.Keychain && answers.Jira.BaseURL != ""

	if keychain {
		answers.Jira, err = Keep(g.StoreSecret, answers.Jira)
		if err != nil {
			return Written{}, err
		}
	}

	layers := g.Where.Layers(request.Place)

	beneath, over, err := Beneath(layers)
	if err != nil {
		return Written{}, err
	}

	err = Save(path, layers, answers.Over(beneath), over)
	if err != nil {
		return Written{}, err
	}

	return Written{Path: path, Keychain: keychain, NotIgnored: NotIgnored(ctx, path)}, nil
}
