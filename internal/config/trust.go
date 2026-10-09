// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/fileowner"
)

// ErrUntrustedFile is a configuration file someone other than the user could
// have written: one another user owns, or one others may write.
var ErrUntrustedFile = errors.New("someone other than you could have written this configuration file")

// ErrLinkedFile is a repository's configuration file that is a link: a
// working tree can carry one to any file of the user's, which a save would
// replace.
var ErrLinkedFile = errors.New("a configuration file in a repository may not be a link")

// ErrCredentialInRepository is an edit that would write into a repository's
// file, a file in a working tree git could commit, a credential the files
// read did not hold.
var ErrCredentialInRepository = errors.New("a credential the configuration files do not already hold is " +
	"never written into a repository's file, which git could commit; set it in the home directory's file")

// ErrHomeOnly is a repository's file setting what only the home directory's
// file may: a program to run, or an environment variable to read.
var ErrHomeOnly = errors.New("only the home directory's file may set it")

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
// service; a webhook URL is its own address. jira.keychain is no credential:
// it reads the keychain item for the address in effect, whichever that is.
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

// movedSections are the credential sections whose address merged gives a
// value other than home's: the sections whose credentials home must not lend.
func movedSections(home, merged Config) []credentialSection {
	var moved []credentialSection

	for _, section := range credentialSections() {
		if section.address(home) != section.address(merged) {
			moved = append(moved, section)
		}
	}

	return moved
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

// homeOnlySetting is a setting only the home directory's file may make: one
// naming a program workflow runs, or an environment variable it reads and
// sends on. A repository's file is part of a working tree that may have been
// cloned from anyone, and a file found outside one may be anyone's.
type homeOnlySetting struct {
	key   string
	value func(Config) string
}

// homeOnlySettings are the settings only the home directory's file may make.
func homeOnlySettings() []homeOnlySetting {
	return []homeOnlySetting{
		{key: "jira.token_command", value: func(cfg Config) string { return cfg.Jira.TokenCommand }},
		{key: "jira.token_env", value: func(cfg Config) string { return cfg.Jira.TokenEnv }},
		{key: "taskwarrior.program", value: func(cfg Config) string { return cfg.Taskwarrior.Program }},
	}
}

// refuseHomeOnly refuses layer, the repository's file at path, when it makes
// any setting only the home directory's file may, naming each. A setting is
// made when the file sets it to other than what lies beneath it: the defaults
// it is read over, or the home file a save writes it over, from which it holds
// only what differs. Left empty, a setting is not made, so a file written
// whole, which holds every key, passes.
func refuseHomeOnly(path string, layer, beneath Config) error {
	var refused []error

	for _, setting := range homeOnlySettings() {
		made := setting.value(layer)
		if made != "" && made != setting.value(beneath) {
			refused = append(refused, fmt.Errorf("%s: %w", setting.key, ErrHomeOnly))
		}
	}

	if len(refused) == 0 {
		return nil
	}

	return fmt.Errorf("%s: %w: %w", path, ErrInvalid, errors.Join(refused...))
}

// The permission bits that let a file's group, and everyone else, write it.
const (
	groupWritable  os.FileMode = 0o020
	othersWritable os.FileMode = 0o002
)

// refuseUntrusted refuses a file someone other than the user could have
// written: one another user owns, one others may write, or one a group may
// write that is not its owner's own. A system that gives each user a group of
// their own gives it the user's id and no other member, and its umask of 002
// leaves every file the user makes writable by that group alone. Where the
// system keeps no owner, as Windows does not, nothing is refused.
func refuseUntrusted(info fs.FileInfo) error {
	owner, known := fileowner.Of(info)
	mode := info.Mode().Perm()

	switch {
	case !known:
		return nil
	case owner.User != os.Geteuid():
		return fmt.Errorf("%w: another user owns it", ErrUntrustedFile)
	case mode&othersWritable != 0, mode&groupWritable != 0 && owner.Group != owner.User:
		return fmt.Errorf("%w: its mode, %#o, lets others write it; run chmod go-w on it", ErrUntrustedFile, mode)
	}

	return nil
}

// refuseLink refuses path when it is a link, to a file or to none yet: a
// read would follow it, and a save replace whatever it names. The home file
// may be a link, as a dotfiles checkout makes it; a repository's may not.
func refuseLink(path string) error {
	if path == "" {
		return nil
	}

	info, err := os.Lstat(path)
	if err == nil && info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%s: %w", path, ErrLinkedFile)
	}

	return nil
}

// secretField is one credential of a configuration, by its path in the file.
type secretField struct {
	path  string
	value func(Config) Secret
}

// slackSecretFields are the Slack user token's credentials, which an editor
// may hand to be kept outside the file.
func slackSecretFields() []secretField {
	return []secretField{
		{path: "messaging.client_secret", value: func(cfg Config) Secret { return cfg.Messaging.ClientSecret }},
		{path: "messaging.refresh_token", value: func(cfg Config) Secret { return cfg.Messaging.RefreshToken }},
		{path: "messaging.access_token", value: func(cfg Config) Secret { return cfg.Messaging.AccessToken }},
	}
}

// jiraTokenField is the Jira token, which an editor may hand to the keychain.
func jiraTokenField() secretField {
	return secretField{path: "jira.token", value: func(cfg Config) Secret { return cfg.Jira.Token }}
}

// fileSecretFields are every credential cfg holds but the Slack secrets and
// the Jira token, each of Jira's headers among them.
func fileSecretFields(cfg Config) []secretField {
	headers := make([]secretField, 0, len(cfg.Jira.Headers))
	for name := range cfg.Jira.Headers {
		headers = append(headers, secretField{
			path: "jira.headers", value: func(cfg Config) Secret { return cfg.Jira.Headers[name] },
		})
	}

	return append([]secretField{
		{path: "forge.token", value: func(cfg Config) Secret { return cfg.Forge.Token }},
		{path: "messaging.webhook_url", value: func(cfg Config) Secret { return cfg.Messaging.WebhookURL }},
	}, headers...)
}

// refuseUnheld refuses incoming, edit's read with the editor's changes, when
// edit saves to a repository's file and incoming holds a credential among
// fields that the files read did not hold the same. A read of no file held
// none. One kept, inherited or removed is no refusal.
func (edit Edit) refuseUnheld(incoming Config, fields []secretField) error {
	if edit.Files.Repo == "" {
		return nil
	}

	held := edit.Read
	if !edit.Over.Exists() {
		held = Config{}
	}

	var unheld []string

	for _, field := range fields {
		value := field.value(incoming)
		if value != "" && value != field.value(held) {
			unheld = append(unheld, field.path)
		}
	}

	if len(unheld) == 0 {
		return nil
	}

	slices.Sort(unheld)

	return fmt.Errorf("%s: %w: %s", edit.Files.Repo, ErrCredentialInRepository,
		strings.Join(slices.Compact(unheld), ", "))
}
