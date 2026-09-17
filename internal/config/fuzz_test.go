// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// visibleTail is how much of a secret Redact deliberately keeps, so a person can
// tell two credentials apart. Everything before it is what must never survive.
const visibleTail = 4

func FuzzRedactRevealsOnlyTheTail(f *testing.F) {
	// Deliberately NOT shaped like a real "xoxb-" token: gitleaks scans this
	// repository and a convincing fixture would fail `task secrets` for
	// everyone. The fuzzer mutates from here anyway, so the seed only needs to
	// be long enough to exercise the tail logic.
	f.Add("credential-0123456789-0123456789-abcdefghijklmnop", "different-prefix-")
	f.Add("https://hooks.slack.com/services/T0/B0/secret", "https://other/")
	f.Add("\x00\xff\xfe invalid utf-8 \xc3\x28", "\xff\xfe")
	// Both found by this fuzzer while the property was still stated as "no run
	// of the secret may appear in the mask". "*0000" masks to "****0000", where
	// the mask's own asterisks and the kept tail spell a window that also
	// occurs in the input; "*00000000" does the same across the mask boundary.
	// Neither is a leak — they are a substring test failing to express an
	// informational claim, which is what drove the property below.
	f.Add("*0000", "*")
	f.Add("*00000000", "*")
	// Also found here: a "secret" that already looks like a masked value is
	// returned unchanged, because it IS its own mask. That tripped a second
	// assertion asking that Redact never return its input — which was an
	// aesthetic expectation, not a security one, and is implied anyway: the
	// identity function could not satisfy the property below.
	f.Add("****0000", "0")

	// The property: a masked value must depend ONLY on the last visibleTail
	// characters. Two different secrets sharing a tail must be indistinguishable
	// afterwards, which is exactly "the mask tells you nothing about what it
	// hid" — and unlike a substring check it cannot be fooled by the mask's own
	// characters colliding with the secret's.
	f.Fuzz(func(t *testing.T, secret, otherPrefix string) {
		// Arrange
		if len(secret) <= visibleTail || otherPrefix == "" {
			return
		}

		tail := secret[len(secret)-visibleTail:]
		twin := otherPrefix + tail

		// Act
		masked, twinMasked := config.Redact(secret), config.Redact(twin)

		// Assert
		if masked != twinMasked {
			t.Fatalf("Redact distinguishes %q from %q, so the mask carries information about the hidden part",
				secret, twin)
		}
	})
}

func FuzzLoadFileNeverLeaksACredential(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"jira":{"base_url":"https://jira.example.com","token":"t","user":""}}`))
	f.Add([]byte(`{"slack":{"token":"xoxb-aaaabbbbccccdddd","webhook_url":"","channel":"#c"}}`))
	f.Add([]byte(`{"slack":{"webhook_url":"https://hooks.slack.com/services/A/B/ccccdddd"}}`))
	f.Add([]byte(`not json at all`))
	f.Add([]byte(`{"unknown_key": 1}`))

	f.Fuzz(func(t *testing.T, document []byte) {
		// Arrange
		path := filepath.Join(t.TempDir(), config.FileName)

		err := os.WriteFile(path, document, config.FileMode)
		if err != nil {
			t.Fatalf("writing the fuzz document: %v", err)
		}

		// Act
		cfg, loadErr := config.LoadFile(path)

		// Assert
		if loadErr != nil {
			// The error is shown to people and pasted into issues, so it must
			// describe the file rather than quote it.
			if strings.Contains(loadErr.Error(), string(document)) && len(document) > visibleTail {
				t.Fatalf("LoadFile error quoted the file's contents: %v", loadErr)
			}

			return
		}

		redacted := cfg.Redacted()
		masked := redacted.Jira.Token + "\n" + redacted.Slack.Token + "\n" + redacted.Slack.WebhookURL

		for _, secret := range []string{cfg.Jira.Token, cfg.Slack.Token, cfg.Slack.WebhookURL} {
			if len(secret) <= visibleTail {
				continue
			}

			if strings.Contains(masked, secret) {
				t.Fatalf("a credential survived Redacted(): %q", secret)
			}
		}
	})
}
