// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"

	"github.com/jacob-delgado/workflow/internal/convention"
)

// ErrInvalidCommit reports a commit default that a Conventional Commit would not
// accept.
var ErrInvalidCommit = errors.New("invalid commit default")

// Commit sets the defaults the commit composer opens on, so a team that scopes
// its commits the same way does not retype it. Every field is optional; empty
// keeps the composer's own default.
type Commit struct {
	// DefaultScope pre-fills the scope field when no kept draft has one. It must
	// be a scope a Conventional Commit subject would accept, or the composer would
	// open already flagged.
	DefaultScope string `json:"default_scope"`
}

// validateCommit refuses a default scope the composer would flag, so a nonsense
// value fails at load rather than at the moment the composer opens.
func (c Config) validateCommit() error {
	err := convention.ValidateScope(c.Commit.DefaultScope)
	if err != nil {
		return fmt.Errorf("%w: default scope: %w", ErrInvalidCommit, err)
	}

	return nil
}
