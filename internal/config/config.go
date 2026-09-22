// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package config loads the workflow configuration file that holds the
// credentials for the services workflow talks to.
package config

import (
	"errors"
	"net/url"
	"os"
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
	// ErrSlackRenamed reports a file still using the old top-level "slack" block,
	// which is now "messaging" with a "kind". It turns the decoder's cryptic
	// unknown-field error into a message that names the migration.
	ErrSlackRenamed = errors.New(
		`the "slack" block was renamed to "messaging"; rename the key and add "kind": "slack"`,
	)
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
	// MarkdownComments rewrites a comment written in Markdown as the wiki markup
	// Jira renders before posting it, so headings, emphasis, code and links come
	// out formatted rather than literal. Off by default: a comment that is
	// already wiki markup, or means its asterisks literally, is posted unchanged.
	MarkdownComments bool `json:"markdown_comments"`
	// ReviewStatus is the status an issue moves to once its pull request is open,
	// e.g. "In Review". Opening a pull request offers the change to this status by
	// name — "in review" and "in progress" share a category, so the loop cannot
	// pick it out on its own. Empty makes no offer.
	ReviewStatus string `json:"review_status"`
}

// Messaging describes how workflow posts team updates. Kind picks the service;
// Slack alone offers two transports — a bot token that can post anywhere it is
// invited and reports back what it posted, or an incoming webhook fixed to one
// channel — while Teams, Discord and a plain webhook post over an incoming
// webhook only.
type Messaging struct {
	// Kind is the service posts go to: "slack" (the default when empty),
	// "teams", "discord", or a plain "webhook". It decides the body and the link
	// markup the notifier sends.
	Kind MessagingKind `json:"kind"`
	// Token is a Slack bot token, which starts with "xoxb-". Leave it empty to
	// take the token from TokenCommand or TokenEnv instead. It is ignored by the
	// webhook-only kinds.
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
	// applies to a Slack bot token only; a webhook carries its own channel.
	Channel string `json:"channel"`
	// Channels are further channels a bot-token post may be routed to at post
	// time, for a change that concerns another team. The default Channel is
	// always available too.
	Channels []string `json:"channels"`
	// Announcement shapes the Slack review message from named placeholders —
	// {author}, {noun}, {title}, {url}, {key}, {summary}, {issue_url}. Empty, or
	// any non-Slack kind, uses the built-in message. Substituted values are
	// always escaped.
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
	// Keys rebinds the interface's keys. Each entry maps an action to the single
	// key that should trigger it, e.g. {"commit": "C", "comment": "ctrl+e"}; the
	// help then shows the new key. An action left out keeps its default. The
	// interface refuses to start on a map that names an action it does not know,
	// or that binds two actions in the same context to one key. The known
	// actions, by where they work, are:
	//
	//   Moving:  next-pane, previous-pane, jump-to-pane, up, down, scroll-up,
	//            scroll-down
	//   Issues:  change-status, comment, assign, log-work, branch-for-issue,
	//            filter, switch-view, load-more, open-link, copy-link, refresh
	//   Branch:  new-branch, switch-task, worktree, rebase, push, stage,
	//            stage-all, commit, amend, fixup, run-pre-commit, set-up-lefthook
	//   Review:  open-pull-request, checks, rerun-checks, merge, finish-branch,
	//            post, post-when-green
	//   Composer: edit, edit-body, next-template, toggle-draft, toggle-breaking,
	//            verbatim, next-field, previous-field, cycle-type-left,
	//            cycle-type-right, toggle-option
	//   Running: stop, run-again, full-output
	//   Everywhere: apply, close, toggle-mouse, toggle-help, quit, interrupt
	Keys map[string]string `json:"keys"`
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
	Version     string      `json:"version"`
	Jira        Jira        `json:"jira"`
	Messaging   Messaging   `json:"messaging"`
	Forge       Forge       `json:"forge"`
	UI          UI          `json:"ui"`
	Timing      Timing      `json:"timing"`
	Branch      Branch      `json:"branch"`
	Commit      Commit      `json:"commit"`
	PullRequest PullRequest `json:"pull_request"`
	// Path is the file this configuration was read from. It is not part of the
	// file format.
	Path string `json:"-"`
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
		Version:   CurrentVersion,
		Jira:      Jira{BaseURL: "", Token: "", User: ""},
		Messaging: Messaging{Token: "", WebhookURL: "", Channel: ""},
		Forge:     Forge{Kind: "", Host: "", Token: ""},
		UI:        UI{Mouse: true, ASCII: false},
		Path:      "",
	}
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
		Messaging: Messaging{
			Kind:       KindSlack,
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

// Service names the messaging service posts go to, for display.
func (m Messaging) Service() string {
	return m.Kind.Service()
}

// Mode reports how this configuration posts. A Slack bot token wins when both a
// token and a webhook are set: it is the more capable transport, and treating
// the overlap as ambiguous would fail a configuration that works perfectly
// well. The webhook-only kinds ignore a bot token and post over the webhook.
func (m Messaging) Mode() MessagingMode {
	if m.Kind.webhookOnly() {
		if m.WebhookURL != "" {
			return MessagingWebhook
		}

		return MessagingNone
	}

	switch {
	case m.hasToken():
		return MessagingBot
	case m.WebhookURL != "":
		return MessagingWebhook
	default:
		return MessagingNone
	}
}

// Target describes where messages go, for display.
//
// It never returns the webhook URL. That URL is itself the credential, so even
// a masked form has no business in output people are invited to paste into bug
// reports.
func (m Messaging) Target() string {
	switch m.Mode() {
	case MessagingBot:
		if m.Channel == "" {
			return "(no channel set)"
		}

		return m.Channel
	case MessagingWebhook:
		return "the channel its webhook is bound to"
	case MessagingNone:
		return notSet
	default:
		return notSet
	}
}

// ChannelChoices are the channels a bot-token post can be sent to: the default
// channel first, then the alternates, without duplicates or blanks. A webhook
// carries its own channel and offers none.
func (m Messaging) ChannelChoices() []string {
	if m.Mode() != MessagingBot {
		return nil
	}

	var choices []string

	for _, channel := range append([]string{m.Channel}, m.Channels...) {
		if channel != "" && !slices.Contains(choices, channel) {
			choices = append(choices, channel)
		}
	}

	return choices
}

// hasToken reports that a bot token is configured, in the file or from a
// command or an environment variable resolved at runtime.
func (m Messaging) hasToken() bool {
	return m.Token != "" || m.TokenCommand != "" || m.TokenEnv != ""
}

// missing names the messaging fields still needed. Slack's two transports are
// reported as ONE entry, because either satisfies the requirement and naming
// both would read as an instruction to set both; a webhook-only kind names just
// the webhook URL.
func (m Messaging) missing() []string {
	switch m.Mode() {
	case MessagingNone:
		if m.Kind.webhookOnly() {
			return []string{"messaging.webhook_url"}
		}

		return []string{"messaging.token or messaging.webhook_url"}
	case MessagingBot:
		if m.Channel == "" {
			return []string{"messaging.channel"}
		}
	case MessagingWebhook:
		// A webhook is bound to its channel when it is created, so asking for
		// messaging.channel as well would be asking for something with no effect.
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

	if hook := c.Messaging.WebhookURL; hook != "" && !secureURL(hook.Reveal()) {
		problems = append(problems, "messaging.webhook_url is not an https URL")
	}

	if !c.Messaging.Kind.Known() {
		problems = append(problems,
			"messaging.kind is not one of slack, teams, discord or webhook")
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

	return append(append(missing, c.Messaging.missing()...), c.Forge.missing()...)
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
