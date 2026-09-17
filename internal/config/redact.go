// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import "net/url"

// visibleSuffix is how many trailing characters of a token stay readable, so a
// person can tell two tokens apart without the value being usable.
const visibleSuffix = 4

// maskedUserinfo stands in for a URL's userinfo wherever one is shown.
const maskedUserinfo = "xxxxx"

// Redacted returns a copy with every token masked, safe to print or log.
func (c Config) Redacted() Config {
	redacted := c
	redacted.Jira.Token = Redact(c.Jira.Token)
	redacted.Slack.Token = Redact(c.Slack.Token)
	redacted.Slack.WebhookURL = Redact(c.Slack.WebhookURL)
	redacted.Jira.BaseURL = RedactURL(c.Jira.BaseURL)
	redacted.Forge.Token = Redact(c.Forge.Token)

	return redacted
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
