// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// The Jira headers storedHeaders holds, and a value typed over one's mask.
const (
	accessHeader = "CF-Access-Client-Secret"
	proxyHeader  = "X-Proxy-Token"
	accessSecret = "cf-access-secret-1111"
	proxySecret  = "proxy-token-2222"
	typedHeader  = "typed-header-3333"
)

// storedHeaders is a configuration whose file holds two Jira headers.
func storedHeaders() config.Config {
	stored := config.Default()
	stored.Jira.Headers = map[string]config.Secret{accessHeader: accessSecret, proxyHeader: proxySecret}

	return stored
}

func TestKeepStoredKeepsOrTakesEachJiraHeaderAsTheEditorSentIt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sent func(masked map[string]config.Secret) map[string]config.Secret
		want map[string]config.Secret
	}{
		{
			name: "both sent back masked keep the stored values",
			sent: func(masked map[string]config.Secret) map[string]config.Secret { return masked },
			want: map[string]config.Secret{accessHeader: accessSecret, proxyHeader: proxySecret},
		},
		{
			name: "one sent empty keeps its stored value",
			sent: func(masked map[string]config.Secret) map[string]config.Secret {
				masked[proxyHeader] = ""

				return masked
			},
			want: map[string]config.Secret{accessHeader: accessSecret, proxyHeader: proxySecret},
		},
		{
			name: "one typed over its mask takes the typed value",
			sent: func(masked map[string]config.Secret) map[string]config.Secret {
				masked[proxyHeader] = typedHeader

				return masked
			},
			want: map[string]config.Secret{accessHeader: accessSecret, proxyHeader: typedHeader},
		},
		{
			name: "one left out is removed",
			sent: func(masked map[string]config.Secret) map[string]config.Secret {
				delete(masked, proxyHeader)

				return masked
			},
			want: map[string]config.Secret{accessHeader: accessSecret},
		},
		{
			name: "one added beside them is taken as typed",
			sent: func(masked map[string]config.Secret) map[string]config.Secret {
				masked["X-Team"] = typedHeader

				return masked
			},
			want: map[string]config.Secret{accessHeader: accessSecret, proxyHeader: proxySecret, "X-Team": typedHeader},
		},
		{
			name: "every one removed leaves none",
			sent: func(map[string]config.Secret) map[string]config.Secret { return map[string]config.Secret{} },
			want: map[string]config.Secret{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			stored := storedHeaders()
			incoming := stored.Redacted()
			incoming.Jira.Headers = test.sent(maps.Clone(incoming.Jira.Headers))

			// Act
			kept := config.KeepStored(incoming, stored, nil)

			// Assert
			if !maps.Equal(kept.Jira.Headers, test.want) {
				t.Errorf("kept headers %v, want %v, each with its own value",
					slices.Sorted(maps.Keys(kept.Jira.Headers)), slices.Sorted(maps.Keys(test.want)))
			}
		})
	}
}

func TestJiraHeadersKeptFromAnEditAreMaskedWhenShown(t *testing.T) {
	t.Parallel()

	// Arrange
	stored := storedHeaders()
	incoming := stored.Redacted()
	incoming.Jira.Headers[proxyHeader] = typedHeader

	// Act
	shown := config.KeepStored(incoming, stored, nil).Redacted()

	// Assert
	for name, value := range shown.Jira.Headers {
		if slices.Contains([]string{accessSecret, proxySecret, typedHeader}, value.Reveal()) {
			t.Errorf("header %s is shown unmasked", name)
		}
	}

	if got := shown.Jira.Headers[proxyHeader].Reveal(); got != config.Redact(typedHeader) {
		t.Errorf("the typed header shows as %q, want %q", got, config.Redact(typedHeader))
	}
}
