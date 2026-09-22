// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

// Store controls the on-disk store that keeps a little workflow state between
// sessions — the commit scope last used in a repository, what was announced, and
// the last issue list seen. It is on by default and never holds a secret.
type Store struct {
	// Disabled turns the store off, so workflow keeps nothing between sessions —
	// the behavior from before the store existed. For a machine where no workflow
	// state should touch the disk.
	Disabled bool `json:"disabled"`
}
