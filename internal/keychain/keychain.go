// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package keychain builds the commands that store a secret in the operating
// system's keychain and read it back, so a token need never sit in the
// configuration file. It links nothing: it drives the platform's own tool,
// which is what keeps the build pure Go.
package keychain

import "github.com/jacob-delgado/workflow/internal/proc"

// Supported reports whether config init can store a secret in the keychain on
// goos without help. Reading one back through a token_command works more widely
// — anywhere its own tool is installed — but storing is wired for macOS here,
// through the built-in `security`.
func Supported(goos string) bool {
	return goos == "darwin"
}

// StoreCommand saves secret in the login keychain under service, updating an
// existing entry (-U) rather than adding a second one for the same service.
func StoreCommand(service, secret string) proc.Command {
	return proc.Command{
		Name: "security",
		Args: []string{"add-generic-password", "-U", "-s", service, "-w", secret},
	}
}

// LookupCommand is the token_command that reads the secret back out of the
// keychain, printing it and nothing else.
func LookupCommand(service string) string {
	return "security find-generic-password -s " + service + " -w"
}
