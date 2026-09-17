// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package config loads the workflow configuration file that holds the
// credentials for the services workflow talks to.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

// FileName is the configuration file's name in both search locations.
const FileName = ".workflow.json"

// notSet is how an empty setting is shown: as something a reader can act on,
// rather than as a blank.
const notSet = "(not set)"

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

// Forge describes the Git forge credential — which workflow usually does not
// need to hold at all.
type Forge struct {
	// Kind is "github" or "gitlab", and is needed only for an on-premises host
	// whose name says neither — a GitHub Enterprise Server and a self-managed
	// GitLab look identical from a git remote, and their APIs differ.
	Kind string `json:"kind"`
	// Token is a GitHub or GitLab personal access token, consulted only when
	// neither the environment nor the forge's own CLI supplies one.
	//
	// It is deliberately absent from Missing(). Reporting it as missing would
	// fail `workflow doctor` for everyone correctly relying on `gh auth login`,
	// which is the common case and the one worth encouraging.
	Token string `json:"token"`
}

// UI is how the terminal interface behaves.
type UI struct {
	// Mouse captures the mouse, so a click focuses a pane or selects a row.
	// Capturing it takes away the terminal's own click-and-drag selection,
	// which is why it can be turned off here, and toggled with m in a session.
	Mouse bool `json:"mouse"`
	// ASCII draws borders and glyphs in plain ASCII, for a terminal or font
	// without box-drawing characters. There is no reliable way to detect that,
	// so it is a setting rather than a guess.
	ASCII bool `json:"ascii"`
}

// Config is the whole configuration file.
type Config struct {
	Jira  Jira  `json:"jira"`
	Slack Slack `json:"slack"`
	Forge Forge `json:"forge"`
	UI    UI    `json:"ui"`
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

// Default is the configuration before any file is read: every setting that has
// a default holds it. A file is decoded over it, so a setting the file leaves out
// keeps its default — which a bool cannot do by itself, since its zero value is
// false.
//
// Every error path returns it as well. The interface still opens without a
// configuration, to say what is wrong, and it should open working normally.
func Default() Config {
	return Config{
		Jira:  Jira{BaseURL: "", Token: "", User: ""},
		Slack: Slack{Token: "", WebhookURL: "", Channel: ""},
		Forge: Forge{Kind: "", Token: ""},
		UI:    UI{Mouse: true, ASCII: false},
		Path:  "",
	}
}

// Load reads the configuration that applies, searching workDir then homeDir.
func Load(workDir, homeDir string) (Config, error) {
	path, err := Discover(workDir, homeDir)
	if err != nil {
		return Default(), err
	}

	return LoadFile(path)
}

// LoadFile reads the configuration from an exact path. Unknown keys are an
// error: a misspelled key that is silently ignored looks exactly like a
// credential that was never set.
func LoadFile(path string) (Config, error) {
	file, err := os.Open(path) //nolint:gosec // the path is the user's own config file, by design
	if err != nil {
		return Default(), fmt.Errorf("opening %s: %w", path, err)
	}
	defer file.Close() //nolint:errcheck // read-only file; a failed close is not actionable

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	cfg := Default()

	err = decoder.Decode(&cfg)
	if err != nil {
		return Default(), fmt.Errorf("%w: %s: %w", ErrInvalid, path, err)
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
		Forge: Forge{Kind: "", Token: ""},
		UI:    UI{Mouse: true, ASCII: false},
		Path:  "",
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
		return notSet
	default:
		return notSet
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
	redacted.Jira.BaseURL = RedactURL(c.Jira.BaseURL)
	redacted.Forge.Token = Redact(c.Forge.Token)

	return redacted
}

// visibleSuffix is how many trailing characters of a token stay readable, so a
// person can tell two tokens apart without the value being usable.
const visibleSuffix = 4

// maskedUserinfo stands in for a URL's userinfo wherever one is shown.
const maskedUserinfo = "xxxxx"

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
