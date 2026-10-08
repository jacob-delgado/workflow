// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import "maps"

// tokenKey is the key a section's token is written under.
const tokenKey = "token"

// credentialSection is a section of the configuration that sends a credential
// to an address it also names.
type credentialSection struct {
	name        string
	address     func(Config) string
	credentials []string
}

// credentialSections are the sections holding a credential, each with the
// address the credential is sent to and the keys that make up the credential.
// A Slack user token goes only to Slack, so for messaging the address is the
// service; a webhook URL is its own address.
func credentialSections() []credentialSection {
	return []credentialSection{
		{
			name:        "jira",
			address:     func(cfg Config) string { return cfg.Jira.BaseURL },
			credentials: []string{tokenKey, "token_command", "token_env", "headers"},
		},
		{
			name:        "forge",
			address:     func(cfg Config) string { return cfg.Forge.Host },
			credentials: []string{tokenKey},
		},
		{
			name:    "messaging",
			address: func(cfg Config) string { return cfg.Messaging.Kind.Service() },
			credentials: []string{
				"client_id", "client_secret", "refresh_token", "access_token", "expires_at", "webhook_url",
			},
		},
	}
}

// movedSections are the credential sections whose address over gives a value
// other than beneath's: the sections whose credentials beneath must not lend.
func movedSections(beneath, over any) ([]credentialSection, error) {
	home, err := configOf(beneath)
	if err != nil {
		return nil, err
	}

	merged, err := configOf(mergeValues(beneath, over))
	if err != nil {
		return nil, err
	}

	var moved []credentialSection

	for _, section := range credentialSections() {
		if section.address(home) != section.address(merged) {
			moved = append(moved, section)
		}
	}

	return moved, nil
}

// configOf is value read as a configuration, unvalidated.
func configOf(value any) (Config, error) {
	encoded, err := encodeValue(value)
	if err != nil {
		return Default(), err
	}

	return decode(encoded)
}

// withoutCredentials is beneath with the credentials of each section in moved
// taken out.
func withoutCredentials(beneath any, moved []credentialSection) any {
	root, isObject := beneath.(map[string]any)
	if !isObject {
		return beneath
	}

	kept := maps.Clone(root)

	for _, section := range moved {
		held, isObject := root[section.name].(map[string]any)
		if !isObject {
			continue
		}

		lent := maps.Clone(held)
		for _, key := range section.credentials {
			delete(lent, key)
		}

		kept[section.name] = lent
	}

	return kept
}
