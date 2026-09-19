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
	"slices"
)

// FileName is the configuration file's name in both search locations.
const FileName = ".workflow.json"

// Guidance shown wherever a configuration is missing, so the first steps read
// the same on every surface.
const (
	// NoConfigHeadline says no configuration was found.
	NoConfigHeadline = "No " + FileName + " found."
	// InitStep names the command that writes a starting configuration.
	InitStep = "Create one with `workflow config init`."
	// DoctorStep names the command that checks it.
	DoctorStep = "Then run `workflow doctor` to see what is still missing."
)

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
	// ErrUnknownVersion reports a file naming a format version this build does
	// not know how to read.
	ErrUnknownVersion = errors.New("unknown configuration version")
)

// Jira describes how to reach an on-premises Jira instance.
type Jira struct {
	// BaseURL is the root of the Jira instance, e.g. https://jira.example.com.
	BaseURL string `json:"base_url"`
	// Token is a personal access token. Leave it empty to take the token from
	// TokenCommand or TokenEnv instead, so the file holds no secret.
	Token Secret `json:"token"`
	// TokenCommand is a program that prints the token, such as
	// "pass show jira/token"; TokenEnv is an environment variable that holds it.
	TokenCommand string `json:"token_command"`
	TokenEnv     string `json:"token_env"`
	// User is optional. Leave it empty for token (Bearer) authentication; set it
	// to authenticate with HTTP Basic instead.
	User string `json:"user"`
	// Headers are extra HTTP headers sent with every Jira request, added after
	// the token, for a Jira reached through a proxy that wants one of its own —
	// an SSO gateway checking a header, say. A value may be a secret, so it is
	// masked wherever the configuration is shown.
	Headers map[string]string `json:"headers"`
	// Views are named issue lists the pane can move between, each a label and its
	// JQL. Empty keeps the one built-in list: open issues assigned to you.
	Views []JiraView `json:"views"`
	// Project is the Jira project key, such as "PROJ". When set, only a branch
	// naming a key in that project is read as an issue, so a name like
	// fix/UTF-8-decoding is not mistaken for one. Empty falls back to a looser
	// guard that rejects common technical tokens (UTF, SHA, CVE) by shape alone.
	Project string `json:"project"`
}

// Slack describes how workflow posts to Slack. Either transport works: a bot
// token can post anywhere it is invited and reports back what it posted, while
// an incoming webhook needs no app scopes but is fixed to one channel.
type Slack struct {
	// Token is a bot token, which starts with "xoxb-". Leave it empty to take
	// the token from TokenCommand or TokenEnv instead.
	Token Secret `json:"token"`
	// TokenCommand is a program that prints the bot token; TokenEnv is an
	// environment variable that holds it.
	TokenCommand string `json:"token_command"`
	TokenEnv     string `json:"token_env"`
	// WebhookURL is an incoming webhook. It is a credential in its own right —
	// anyone holding it can post to that channel — so it is masked wherever a
	// token would be.
	WebhookURL Secret `json:"webhook_url"`
	// Channel is the channel updates are posted to, e.g. "#dev-workflow". It
	// applies to the bot token only; a webhook carries its own channel.
	Channel string `json:"channel"`
	// Channels are further channels a bot-token post may be routed to at post
	// time, for a change that concerns another team. The default Channel is
	// always available too.
	Channels []string `json:"channels"`
	// Announcement shapes the review message from named placeholders — {author},
	// {noun}, {title}, {url}, {key}, {summary}, {issue_url}. Empty uses the
	// built-in message. Substituted values are always escaped.
	Announcement string `json:"announcement"`
}

// Forge describes the Git forge credential — which workflow usually does not
// need to hold at all.
type Forge struct {
	// Kind is "github" or "gitlab", and is needed only for an on-premises host
	// whose name says neither — a GitHub Enterprise Server and a self-managed
	// GitLab look identical from a git remote, and their APIs differ.
	Kind string `json:"kind"`
	// Host is the host Kind and Token were written for, such as
	// git.example.com.
	Host string `json:"host"`
	// Token is a GitHub or GitLab personal access token, consulted only when
	// neither the environment nor the forge's own CLI supplies one.
	//
	// It is deliberately absent from Missing(). Reporting it as missing would
	// fail `workflow doctor` for everyone correctly relying on `gh auth login`,
	// which is the common case and the one worth encouraging.
	Token Secret `json:"token"`
	// CLI routes forge API calls through the forge's own command-line tool — gh
	// for GitHub, glab for GitLab — instead of over HTTP directly, so the login
	// those tools already hold carries the request. This is what reaches a forge
	// behind an SSO gateway a bare token cannot. It falls back to HTTP when the
	// tool is not installed.
	CLI bool `json:"cli"`
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
	// Color is whether to draw the system hues: empty (auto) draws them when the
	// terminal allows, "never" turns them off while keeping bold, faint and the
	// reverse-video cursor, which carry meaning without color.
	Color string `json:"color"`
	// Notify rings the terminal, and sends a desktop notification where the
	// terminal relays one, when CI finishes — so a developer who stepped away is
	// told rather than having to check back. Off unless set.
	Notify bool `json:"notify"`
	// CommentsShown is how many of an issue's most recent comments the detail
	// pane draws. Zero keeps the built-in default.
	CommentsShown int `json:"comments_shown"`
}

// DrawColor reports whether the system hues should be drawn. NO_COLOR (set to
// any value) and ui.color "never" both turn them off; bold, faint and reverse
// stay.
func (u UI) DrawColor(noColorEnv string) bool {
	return noColorEnv == "" && u.Color != "never"
}

// CurrentVersion is the configuration format version this build writes and
// reads. A file may leave it out — an unversioned file is read as the current
// version — but a file that names a version this build does not know is refused
// rather than half-read against a format it was not written for.
const CurrentVersion = "1"

// Config is the whole configuration file.
type Config struct {
	// Version is the file format's version; empty means the current one. It is
	// first so `config init` writes it at the top of the file.
	Version string `json:"version"`
	Jira    Jira   `json:"jira"`
	Slack   Slack  `json:"slack"`
	Forge   Forge  `json:"forge"`
	UI      UI     `json:"ui"`
	Timing  Timing `json:"timing"`
	Branch  Branch `json:"branch"`
	Commit  Commit `json:"commit"`
	// Path is the file this configuration was read from. It is not part of the
	// file format.
	Path string `json:"-"`
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
		Version: CurrentVersion,
		Jira:    Jira{BaseURL: "", Token: "", User: ""},
		Slack:   Slack{Token: "", WebhookURL: "", Channel: ""},
		Forge:   Forge{Kind: "", Host: "", Token: ""},
		UI:      UI{Mouse: true, ASCII: false},
		Path:    "",
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

	err = errors.Join(cfg.validateVersion(), cfg.validateTiming(),
		cfg.validateBranch(), cfg.validateViews(), cfg.validateCommit())
	if err != nil {
		return Default(), fmt.Errorf("%w: %s: %w", ErrInvalid, path, err)
	}

	cfg.Path = path

	return cfg, nil
}

// Template is the starting configuration `workflow config init` writes.
func Template() Config {
	return Config{
		Version: CurrentVersion,
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
		Forge: Forge{Kind: "", Host: "", Token: ""},
		UI:    UI{Mouse: true, ASCII: false},
		Path:  "",
	}
}

// Configured reports that a Jira instance is set as the issue tracker, by its
// base URL. When it is not, the forge's own issues back the Issues pane instead.
func (j Jira) Configured() bool {
	return j.BaseURL != ""
}

// AuthMode reports how requests to this Jira instance authenticate.
func (j Jira) AuthMode() AuthMode {
	switch {
	case !j.hasToken():
		return AuthNone
	case j.User != "":
		return AuthBasic
	default:
		return AuthBearer
	}
}

// hasToken reports that a token is configured — in the file, or from a command
// or an environment variable resolved at runtime.
func (j Jira) hasToken() bool {
	return j.Token != "" || j.TokenCommand != "" || j.TokenEnv != ""
}

// Mode reports how this configuration posts to Slack. A bot token wins when
// both are set: it is the more capable transport, and treating the overlap as
// ambiguous would fail a configuration that works perfectly well.
func (s Slack) Mode() SlackMode {
	switch {
	case s.hasToken():
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

// ChannelChoices are the channels a bot-token post can be sent to: the default
// channel first, then the alternates, without duplicates or blanks. A webhook
// carries its own channel and offers none.
func (s Slack) ChannelChoices() []string {
	if s.Mode() != SlackBot {
		return nil
	}

	var choices []string

	for _, channel := range append([]string{s.Channel}, s.Channels...) {
		if channel != "" && !slices.Contains(choices, channel) {
			choices = append(choices, channel)
		}
	}

	return choices
}

// hasToken reports that a bot token is configured, in the file or from a
// command or an environment variable resolved at runtime.
func (s Slack) hasToken() bool {
	return s.Token != "" || s.TokenCommand != "" || s.TokenEnv != ""
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

// Problems reports settings that are present but malformed — filled in wrong —
// as opposed to Missing, which reports what is still empty. A configuration can
// be complete and still not work, so doctor should say which values are bad
// rather than call a set-but-invalid file all clear.
func (c Config) Problems() []string {
	var problems []string

	if base := c.Jira.BaseURL; base != "" && !absoluteWebURL(base) {
		problems = append(problems, "jira.base_url is not an absolute http or https URL")
	}

	if hook := c.Slack.WebhookURL; hook != "" && !secureURL(hook.Reveal()) {
		problems = append(problems, "slack.webhook_url is not an https URL")
	}

	return problems
}

// absoluteWebURL reports whether raw is an absolute http or https URL with a
// host.
func absoluteWebURL(raw string) bool {
	parsed, err := url.Parse(raw)

	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

// secureURL reports whether raw is an https URL with a host.
func secureURL(raw string) bool {
	parsed, err := url.Parse(raw)

	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

// Missing names the configuration fields that are still empty, in the order a
// person would fill them in. An empty result means the configuration is
// complete.
func (c Config) Missing() []string {
	var missing []string

	if c.Jira.BaseURL == "" {
		missing = append(missing, "jira.base_url")
	}

	if !c.Jira.hasToken() {
		missing = append(missing, "jira.token")
	}

	return append(append(missing, c.Slack.missing()...), c.Forge.missing()...)
}

// missing names the forge field still needed. forge.kind describes forge.host,
// so by itself it describes nothing; everything else about the forge is
// optional, and a configuration that sets none of it is complete.
func (f Forge) missing() []string {
	if f.Kind != "" && f.Host == "" {
		return []string{"forge.host"}
	}

	return nil
}
