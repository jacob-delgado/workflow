// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import "fmt"

// secretMask is what a Secret renders as. It is not the value, and not a prefix
// of it — Redacted keeps a recognizable tail for display; this is the fallback
// for anything that never went through it.
const secretMask = "****"

// Secret is a credential the configuration carries: a token, or a webhook URL
// that is a credential in its own right.
//
// It is a named type with a String that masks rather than a bare string, and
// that is a guard, not a decoration: every formatting verb — %v, %s, %q, and a
// Secret nested in any struct printed with %+v — goes through String and yields
// the mask. Redact already masks wherever a configuration is shown on purpose;
// this stands behind it, so a future raw %v of a Config cannot leak what Redact
// was never asked to hide. Reading the real value takes an explicit Reveal,
// which is easy to find in review and impossible to do by accident.
type Secret string

var _ fmt.Stringer = Secret("")

// String masks the secret.
func (s Secret) String() string {
	if s == "" {
		return ""
	}

	return secretMask
}

// Reveal returns the real value. Call it only where the credential is sent, or
// inspected to validate it — never where it might be printed.
func (s Secret) Reveal() string {
	return string(s)
}
