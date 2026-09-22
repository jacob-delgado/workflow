// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidBranch reports a branch template that could hide the issue key.
var ErrInvalidBranch = errors.New("invalid branch template")

// keyPlaceholder is the part of a branch template that stands in for the issue
// key, and the one part a template cannot leave out.
const keyPlaceholder = "{key}"

// Branch shapes the branch name workflow proposes for an issue, so a team can
// match its own convention. Every field is optional; empty keeps the default.
type Branch struct {
	// Template shapes the name from {prefix}, {key} and {slug}; empty keeps the
	// default "{prefix}/{key}-{slug}". It must contain {key}, because everything
	// the interface derives from a branch reads the issue key back out of it.
	Template string `json:"template"`
	// Prefixes maps an issue type to its branch prefix, such as {"bug": "bugfix"};
	// the type is matched without regard to case. A type not listed takes
	// DefaultPrefix. Configuring this replaces the built-in map rather than adding
	// to it, so what the file says is the whole rule.
	Prefixes map[string]string `json:"prefixes"`
	// DefaultPrefix is the prefix for a type not in Prefixes; empty keeps "feat".
	DefaultPrefix string `json:"default_prefix"`
	// SlugLimit caps the summary slug in a branch name, in characters; zero keeps
	// the built-in 48.
	SlugLimit int `json:"slug_limit"`
}

// validateBranch refuses a template that could hide the issue key or a negative
// slug limit, so a nonsense value fails at load rather than producing branches
// nothing can read back.
func (c Config) validateBranch() error {
	if c.Branch.Template != "" && !strings.Contains(c.Branch.Template, keyPlaceholder) {
		return fmt.Errorf("%w: must contain %s: %q", ErrInvalidBranch, keyPlaceholder, c.Branch.Template)
	}

	if c.Branch.SlugLimit < 0 {
		return fmt.Errorf("%w: slug_limit cannot be negative: %d", ErrInvalidBranch, c.Branch.SlugLimit)
	}

	return nil
}
