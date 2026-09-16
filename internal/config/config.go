// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package config loads the workflow configuration file that holds the
// credentials for the services workflow talks to.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FileName is the configuration file's name in both search locations.
const FileName = ".workflow.json"

// FileMode is the permission a configuration file is written with. It holds API
// tokens, so nobody but the owner may read it.
const FileMode os.FileMode = 0o600

// Errors returned by this package. Callers distinguish them with errors.Is.
var (
	// ErrNotFound reports that no configuration file exists in either location.
	ErrNotFound = errors.New("no " + FileName + " found")
	// ErrInvalid reports a file that exists but could not be understood.
	ErrInvalid = errors.New("invalid " + FileName)
)

// Jira describes how to reach an on-premises Jira instance.
type Jira struct {
	// BaseURL is the root of the Jira instance, e.g. https://jira.example.com.
	BaseURL string `json:"base_url"`
	// Token is a personal access token.
	Token string `json:"token"`
	// User is optional. Leave it empty for token (Bearer) authentication; set it
	// to authenticate with HTTP Basic instead.
	User string `json:"user"`
}

// Slack describes how workflow posts to Slack. Either transport works: a bot
// token can post anywhere it is invited and reports back what it posted, while
// an incoming webhook needs no app scopes but is fixed to one channel.
type Slack struct {
	// Token is a bot token, which starts with "xoxb-".
	Token string `json:"token"`
	// WebhookURL is an incoming webhook. It is a credential in its own right —
	// anyone holding it can post to that channel — so it is masked wherever a
	// token would be.
	WebhookURL string `json:"webhook_url"`
	// Channel is the channel updates are posted to, e.g. "#dev-workflow". It
	// applies to the bot token only; a webhook carries its own channel.
	Channel string `json:"channel"`
}

// Config is the whole configuration file.
type Config struct {
	Jira  Jira  `json:"jira"`
	Slack Slack `json:"slack"`
	// Path is the file this configuration was read from. It is not part of the
	// file format.
	Path string `json:"-"`
}

// AuthMode is how a request authenticates to Jira.
type AuthMode int

const (
	// AuthNone means no credentials are configured.
	AuthNone AuthMode = iota
	// AuthBearer sends the token as a bearer token, which is what a Jira Data
	// Center personal access token expects.
	AuthBearer
	// AuthBasic sends user and token as HTTP Basic credentials.
	AuthBasic
)

// String names the authentication mode for humans.
func (a AuthMode) String() string {
	switch a {
	case AuthNone:
		return "none"
	case AuthBearer:
		return "bearer token"
	case AuthBasic:
		return "basic auth"
	default:
		return "unknown"
	}
}

// SlackMode is the transport a message to Slack travels over.
type SlackMode int

const (
	// SlackNone means no Slack credential is configured.
	SlackNone SlackMode = iota
	// SlackBot posts with a bot token, which needs a channel and an invitation.
	SlackBot
	// SlackWebhook posts to an incoming webhook, which carries its own channel.
	SlackWebhook
)

// String names the transport for humans.
func (s SlackMode) String() string {
	switch s {
	case SlackNone:
		return "none"
	case SlackBot:
		return "bot token"
	case SlackWebhook:
		return "incoming webhook"
	default:
		return "unknown"
	}
}

// Discover returns the path of the configuration file that applies, searching
// workDir first and then homeDir. A file in the working directory replaces the
// one in the home directory rather than merging with it: a repository-local
// configuration is a complete answer, so the two can never combine into a state
// neither file describes.
//
// It returns ErrNotFound when neither location has one.
func Discover(workDir, homeDir string) (string, error) {
	for _, dir := range []string{workDir, homeDir} {
		if dir == "" {
			continue
		}

		candidate := filepath.Join(dir, FileName)

		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("%w in %s or %s", ErrNotFound, workDir, homeDir)
}

// Load reads the configuration that applies, searching workDir then homeDir.
func Load(workDir, homeDir string) (Config, error) {
	path, err := Discover(workDir, homeDir)
	if err != nil {
		return Config{}, err
	}

	return LoadFile(path)
}

// LoadFile reads the configuration from an exact path. Unknown keys are an
// error: a misspelled key that is silently ignored looks exactly like a
// credential that was never set.
func LoadFile(path string) (Config, error) {
	file, err := os.Open(path) //nolint:gosec // the path is the user's own config file, by design
	if err != nil {
		return Config{}, fmt.Errorf("opening %s: %w", path, err)
	}
	defer file.Close() //nolint:errcheck // read-only file; a failed close is not actionable

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	var cfg Config

	err = decoder.Decode(&cfg)
	if err != nil {
		return Config{}, fmt.Errorf("%w: %s: %w", ErrInvalid, path, err)
	}

	cfg.Path = path

	return cfg, nil
}

// Save writes the configuration to path at FileMode.
func Save(path string, cfg Config) error {
	encoded, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding configuration: %w", err)
	}

	encoded = append(encoded, '\n')

	err = os.WriteFile(path, encoded, FileMode)
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// Template is the starting configuration `workflow config init` writes.
func Template() Config {
	return Config{
		Jira: Jira{
			BaseURL: "https://jira.example.com",
			Token:   "",
			User:    "",
		},
		Slack: Slack{
			Token:      "",
			WebhookURL: "",
			Channel:    "#dev-workflow",
		},
		Path: "",
	}
}

// AuthMode reports how requests to this Jira instance authenticate.
func (j Jira) AuthMode() AuthMode {
	switch {
	case j.Token == "":
		return AuthNone
	case j.User != "":
		return AuthBasic
	default:
		return AuthBearer
	}
}

// Mode reports how this configuration posts to Slack. A bot token wins when
// both are set: it is the more capable transport, and treating the overlap as
// ambiguous would fail a configuration that works perfectly well.
func (s Slack) Mode() SlackMode {
	switch {
	case s.Token != "":
		return SlackBot
	case s.WebhookURL != "":
		return SlackWebhook
	default:
		return SlackNone
	}
}

// Target describes where messages go, for display.
//
// It never returns the webhook URL. That URL is itself the credential, so even
// a masked form has no business in output people are invited to paste into bug
// reports.
func (s Slack) Target() string {
	switch s.Mode() {
	case SlackBot:
		if s.Channel == "" {
			return "(no channel set)"
		}

		return s.Channel
	case SlackWebhook:
		return "the channel its webhook is bound to"
	case SlackNone:
		return "(not set)"
	default:
		return "(not set)"
	}
}

// missing names the Slack fields still needed. The two transports are reported
// as ONE entry, because either satisfies the requirement and naming both would
// read as an instruction to set both.
func (s Slack) missing() []string {
	switch s.Mode() {
	case SlackNone:
		return []string{"slack.token or slack.webhook_url"}
	case SlackBot:
		if s.Channel == "" {
			return []string{"slack.channel"}
		}
	case SlackWebhook:
		// A webhook is bound to its channel when it is created, so asking for
		// slack.channel as well would be asking for something with no effect.
	}

	return nil
}

// Missing names the configuration fields that are still empty, in the order a
// person would fill them in. An empty result means the configuration is
// complete.
func (c Config) Missing() []string {
	var missing []string

	if c.Jira.BaseURL == "" {
		missing = append(missing, "jira.base_url")
	}

	if c.Jira.Token == "" {
		missing = append(missing, "jira.token")
	}

	return append(missing, c.Slack.missing()...)
}

// Redacted returns a copy with every token masked, safe to print or log.
func (c Config) Redacted() Config {
	redacted := c
	redacted.Jira.Token = Redact(c.Jira.Token)
	redacted.Slack.Token = Redact(c.Slack.Token)
	redacted.Slack.WebhookURL = Redact(c.Slack.WebhookURL)

	return redacted
}

// visibleSuffix is how many trailing characters of a token stay readable, so a
// person can tell two tokens apart without the value being usable.
const visibleSuffix = 4

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
