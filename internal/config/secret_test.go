// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

const plaintext = "xoxb-super-secret-value"

func TestASecretMasksUnderEveryVerbAndWhenNested(t *testing.T) {
	t.Parallel()

	// A Config printed raw is the leak this type guards against: a stray %v or a
	// log line that never went through Redacted must still not show the value.
	cfg := config.Config{
		Jira: config.Jira{
			Token:   config.Secret(plaintext),
			Headers: map[string]config.Secret{"X-Gateway-Secret": config.Secret(plaintext)},
		},
		Messaging: config.Messaging{Token: config.Secret(plaintext), WebhookURL: config.Secret(plaintext)},
		Forge:     config.Forge{Token: config.Secret(plaintext)},
	}

	printed := map[string]string{
		"%v of the secret":  fmt.Sprintf("%v", config.Secret(plaintext)),
		"%s in a sentence":  fmt.Sprintf("the token is %s here", config.Secret(plaintext)),
		"%q of the secret":  fmt.Sprintf("%q", config.Secret(plaintext)),
		"%#v of the secret": fmt.Sprintf("%#v", config.Secret(plaintext)),
		"%v of the config":  fmt.Sprintf("%v", cfg),
		"%+v of the config": fmt.Sprintf("%+v", cfg),
		"%#v of the config": fmt.Sprintf("%#v", cfg),
	}

	for name, out := range printed {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if strings.Contains(out, plaintext) {
				t.Errorf("%s leaked the secret: %s", name, out)
			}
		})
	}
}

func TestASecretPrintsAsAQuotedMaskUnderSharpV(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		secret config.Secret
		want   string
	}{
		"a set secret is the mask, quoted":   {secret: config.Secret(plaintext), want: `"****"`},
		"an empty secret is an empty string": {secret: config.Secret(""), want: `""`},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := fmt.Sprintf("%#v", tt.secret)

			// Assert
			if got != tt.want {
				t.Errorf("%%#v = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRevealReturnsTheRealValueAndEmptyShowsNothing(t *testing.T) {
	t.Parallel()

	// Act & Assert
	if got := config.Secret(plaintext).Reveal(); got != plaintext {
		t.Errorf("Reveal() = %q, want the real value", got)
	}

	if got := config.Secret("").String(); got != "" {
		t.Errorf("empty Secret String() = %q, want the empty string", got)
	}
}
