// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"strconv"
)

// secretMask is what a Secret renders as. It is not the value, and not a prefix
// of it — Redacted keeps a recognizable tail for display; this is the fallback
// for anything that never went through it.
const secretMask = "****"

// Secret is a credential the configuration carries: a token, or a webhook URL
// that is a credential in its own right.
//
// It is a named type with a String and a GoString that mask rather than a bare
// string, and that is a guard, not a decoration: every verb a string takes —
// %v, %s, %q, %x, %X and %#v — goes through one of them and yields the mask,
// as does a Secret in an exported field or a map printed with %v, %+v or %#v.
// fmt calls neither method for a verb a string rejects (%d, %t, ...), which go
// vet's printf check, on in the linters, flags, nor for a Secret in an
// unexported field. Redact already masks wherever a configuration is shown on
// purpose; this stands behind it, so a future raw %v of a Config cannot print
// what Redact was never asked to hide. Reading the real value takes an
// explicit Reveal, which is easy to find in review and impossible to do by
// accident.
type Secret string

var (
	_ fmt.Stringer   = Secret("")
	_ fmt.GoStringer = Secret("")
)

// String masks the secret.
func (s Secret) String() string {
	if s == "" {
		return ""
	}

	return secretMask
}

// GoString masks the secret under %#v, quoted so the output still reads as Go
// syntax: "****", or "" for an empty Secret.
func (s Secret) GoString() string {
	return strconv.Quote(s.String())
}

// Reveal returns the real value. Call it only where the credential is sent, or
// inspected to validate it — never where it might be printed.
func (s Secret) Reveal() string {
	return string(s)
}
