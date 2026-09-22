// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jacob-delgado/workflow/internal/convention"
)

// ErrInvalidCommit reports a commit default that a Conventional Commit would not
// accept.
var ErrInvalidCommit = errors.New("invalid commit default")

// Commit shapes the commit messages the composer helps write, so a team with its
// own conventions does not fight the defaults. Every field is optional; empty or
// zero keeps the built-in default.
type Commit struct {
	// DefaultScope pre-fills the scope field when no kept draft has one. It must
	// be a scope a Conventional Commit subject would accept, or the composer would
	// open already flagged.
	DefaultScope string `json:"default_scope"`
	// Types are the commit types the composer offers and validates against, in the
	// order to offer them; empty keeps the built-in Conventional Commit types.
	// Each is a lowercase word, matched as a branch prefix too.
	Types []string `json:"types"`
	// SubjectLimit is the longest a subject may be, in characters; zero keeps the
	// built-in 72.
	SubjectLimit int `json:"subject_limit"`
	// RefsTrailer labels the issue trailer added to a commit body, such as
	// "Closes"; empty keeps "Refs". It is a single trailer word, no colon.
	RefsTrailer string `json:"refs_trailer"`
}

// validateCommit refuses commit conventions the composer could not honor, so a
// nonsense value fails at load rather than at the moment the composer opens.
func (c Config) validateCommit() error {
	err := convention.ValidateScope(c.Commit.DefaultScope)
	if err != nil {
		return fmt.Errorf("%w: default scope: %w", ErrInvalidCommit, err)
	}

	for _, commitType := range c.Commit.Types {
		err = convention.ValidateType(commitType)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidCommit, err)
		}
	}

	if c.Commit.SubjectLimit < 0 {
		return fmt.Errorf("%w: subject_limit cannot be negative: %d", ErrInvalidCommit, c.Commit.SubjectLimit)
	}

	if trailer := strings.TrimSpace(c.Commit.RefsTrailer); strings.ContainsAny(trailer, ": \t\n") {
		return fmt.Errorf("%w: refs_trailer is a single word without a colon: %q", ErrInvalidCommit, c.Commit.RefsTrailer)
	}

	return nil
}
