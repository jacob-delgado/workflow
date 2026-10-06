// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"cmp"
	"maps"
	"net/url"
	"slices"
	"strings"
)

// visibleSuffix is how many trailing characters of a token stay readable, so a
// person can tell two tokens apart without the value being usable.
const visibleSuffix = 4

// maskedUserinfo stands in for a URL's userinfo wherever one is shown.
const maskedUserinfo = "xxxxx"

// Redacted returns a copy with every token masked, safe to print or log.
func (c Config) Redacted() Config {
	redacted := c
	redacted.Jira.Token = redactSecret(c.Jira.Token)
	redacted.Jira.Headers = redactHeaders(c.Jira.Headers)
	redacted.Messaging.ClientSecret = redactSecret(c.Messaging.ClientSecret)
	redacted.Messaging.RefreshToken = redactSecret(c.Messaging.RefreshToken)
	redacted.Messaging.AccessToken = redactSecret(c.Messaging.AccessToken)
	redacted.Messaging.WebhookURL = redactSecret(c.Messaging.WebhookURL)
	redacted.Jira.BaseURL = RedactURL(c.Jira.BaseURL)
	redacted.Forge.Token = redactSecret(c.Forge.Token)

	return redacted
}

// redactSecret masks a Secret for display, keeping the recognizable tail Redact
// leaves so config show can still tell two credentials apart.
func redactSecret(secret Secret) Secret {
	return Secret(Redact(secret.Reveal()))
}

// redactHeaders masks the value of every extra Jira header. A header a proxy
// checks — a Cloudflare Access secret, say — is a credential, and there is no
// way to tell a secret one from a harmless one, so every value is masked.
func redactHeaders(headers map[string]Secret) map[string]Secret {
	if headers == nil {
		return nil
	}

	masked := make(map[string]Secret, len(headers))
	for name, value := range headers {
		masked[name] = redactSecret(value)
	}

	return masked
}

// RedactURL masks the userinfo of a URL, leaving the rest readable.
//
// A base URL is not a secret, so it is shown in full — but nothing stops someone
// writing https://user:password@jira.example.com into jira.base_url, and doctor
// prints that line into output the bug report template asks people to paste
// into a public issue. The userinfo is masked whole rather than by its
// password alone, because which half holds the part worth hiding is the
// writer's choice, not something to be guessed from here.
func RedactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil {
		return raw
	}

	parsed.User = url.User(maskedUserinfo)

	return parsed.String()
}

// DisplayURL is a URL as it may be shown: a password in it masked, and an unset
// one said so. Both doctor and the terminal interface show jira.base_url, and
// when each did this by hand, one of them forgot the mask.
func DisplayURL(raw string) string {
	if raw == "" {
		return notSet
	}

	return RedactURL(raw)
}

// Redact masks a secret, keeping only enough of the tail to recognize it.
func Redact(secret string) string {
	if secret == "" {
		return ""
	}

	if len(secret) <= visibleSuffix {
		return "****"
	}

	return "****" + secret[len(secret)-visibleSuffix:]
}

// RedactText masks every credential c holds wherever text carries it, each as
// Redacted shows it — a token, a webhook URL or a Jira header value down to
// its recognizable tail, the userinfo of jira.base_url whole — so text another
// program wrote, which could quote one, can be printed.
func (c Config) RedactText(text string) string {
	masks := c.credentialMasks()

	// Longest first, so a credential that holds another is masked whole rather
	// than around the one inside it, which would leave the rest showing.
	slices.SortStableFunc(masks, func(a, b credentialMask) int {
		return cmp.Compare(len(b.credential), len(a.credential))
	})

	redacted := text
	for _, mask := range masks {
		redacted = strings.ReplaceAll(redacted, mask.credential, mask.shown)
	}

	return redacted
}

// credentialMask is a credential and what is shown in its place.
type credentialMask struct {
	credential string
	shown      string
}

// credentialMasks pairs each credential c holds with its mask. An unset one
// is left out: replacing "" would splice the mask between every character.
func (c Config) credentialMasks() []credentialMask {
	secrets := append([]Secret{
		c.Jira.Token, c.Messaging.ClientSecret, c.Messaging.RefreshToken, c.Messaging.AccessToken,
		c.Messaging.WebhookURL, c.Forge.Token,
	},
		slices.Collect(maps.Values(c.Jira.Headers))...)

	masks := make([]credentialMask, 0, len(secrets)+1)

	for _, secret := range secrets {
		if secret != "" {
			masks = append(masks, credentialMask{credential: secret.Reveal(), shown: Redact(secret.Reveal())})
		}
	}

	parsed, err := url.Parse(c.Jira.BaseURL)
	if err == nil && parsed.User != nil {
		masks = append(masks, credentialMask{credential: parsed.User.String(), shown: maskedUserinfo})
	}

	return masks
}

// KeepStored keeps each stored secret when its incoming field is empty or
// still the masked value Redacted gave the editor — a configuration editor,
// the web's Settings or the terminal's, sends the masked form back unchanged,
// and must not overwrite the real secret with the mask.
func KeepStored(incoming, stored Config) Config {
	incoming.Jira.BaseURL = keepMaskedURL(incoming.Jira.BaseURL, stored.Jira.BaseURL)
	incoming.Jira.Token = keepSecret(incoming.Jira.Token, stored.Jira.Token)
	incoming.Messaging.ClientSecret = keepSecret(incoming.Messaging.ClientSecret, stored.Messaging.ClientSecret)
	incoming.Messaging.RefreshToken = keepSecret(incoming.Messaging.RefreshToken, stored.Messaging.RefreshToken)
	incoming.Messaging.AccessToken = keepSecret(incoming.Messaging.AccessToken, stored.Messaging.AccessToken)
	incoming.Messaging.WebhookURL = keepSecret(incoming.Messaging.WebhookURL, stored.Messaging.WebhookURL)
	incoming.Forge.Token = keepSecret(incoming.Forge.Token, stored.Forge.Token)
	incoming.Jira.Headers = keepHeaders(incoming.Jira.Headers, stored.Jira.Headers)

	return incoming
}

// keepMaskedURL keeps the stored base URL when the incoming one is only its
// masked form. jira.base_url may carry userinfo (it becomes Basic auth), which
// the read masks like any other credential; the config editor sends that masked
// URL back unchanged, and it must not overwrite the real password with the mask.
// A genuinely edited URL differs from the mask and is taken as sent.
func keepMaskedURL(incoming, stored string) string {
	if incoming == RedactURL(stored) {
		return stored
	}

	return incoming
}

// keepSecret returns the stored secret when the incoming one is empty or the
// mask of the stored value, and the incoming one otherwise.
func keepSecret(incoming, stored Secret) Secret {
	value := incoming.Reveal()
	if value == "" || value == Redact(stored.Reveal()) {
		return stored
	}

	return incoming
}

// keepHeaders applies keepSecret to each Jira header value, which is masked on
// read the same way a token is.
func keepHeaders(incoming, stored map[string]Secret) map[string]Secret {
	if incoming == nil {
		return nil
	}

	kept := make(map[string]Secret, len(incoming))
	for key, value := range incoming {
		kept[key] = keepSecret(value, stored[key])
	}

	return kept
}
