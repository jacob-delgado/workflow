// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strconv"
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// settingKind is how a setting is shown and edited.
type settingKind int

// The kinds of setting: typed text; a URL whose userinfo is a credential, so
// it is shown masked; a credential, never shown; a count; a comma-separated
// list; one of a fixed set; one that is on or off; an entry of a collection;
// and the row that adds one.
const (
	settingText settingKind = iota
	settingURL
	settingSecret
	settingCount
	settingList
	settingChoice
	settingToggle
	settingEntry
	settingAdd
)

// option is a value a setting offers, beside the words it is offered in.
type option struct {
	value, words string
}

// setting is one row of Settings: the section it is under, its label, its
// path in the configuration's JSON form, what it is, and what it does. A row
// of a collection names the collection, and the entry it stands for.
type setting struct {
	section, label, path, hint string
	kind                       settingKind
	choices                    []option
	list                       collection
	entry                      string
}

// words is how a choice's value reads, an unset one as the first choice: the
// default the configuration takes, which a file may also name, as "slack" or
// "commit" — the first choice is empty, and stands for it. A file holding any
// other is refused as it is read.
func (s setting) words(value string) string {
	for _, offered := range s.choices {
		if offered.value == value {
			return offered.words
		}
	}

	return s.choices[0].words
}

// cycled is the choice after value, or before it when step is -1, wrapping
// round.
func (s setting) cycled(value string, step int) string {
	at := slices.IndexFunc(s.choices, func(offered option) bool { return offered.value == value })
	next := (max(at, 0) + step + len(s.choices)) % len(s.choices)

	return s.choices[next].value
}

// settings are the settings the web's Settings edits, in its order, labels
// and hints: the token's hint from the configuration read, and a row for each
// entry of a collection as the form holds it.
func (f settingsForm) settings() []setting {
	return slices.Concat(jiraSettings(f.noun, f.values, f.value), messagingSettings(), forgeSettings(),
		commitSettings(), branchSettings(f.value), pullRequestSettings(f.noun), storeAndTaskwarriorSettings(),
		timingSettings(), terminalSettings(), keyboardSettings(f.actions))
}

// keychainLabel is the Jira keychain's row, the words the web's checkbox
// carries too.
const keychainLabel = "Read the token from your keychain, kept there for this address (macOS)"

// tokenHint says where the Jira token comes from: the file's own token as
// read wins, then the keychain as the form holds it, then token_env, then
// token_command, so a token typed over a source is used instead of it. None of
// the sources is a secret.
func tokenHint(read map[string]any, held func(string) any) string {
	token, _ := valueAt(read, "jira.token").(string)
	source := tokenSource(held)

	switch {
	case source == "" && token != "":
		return "Leave empty to keep the stored token."
	case source == "":
		return "A personal access token, " + typedTokenKept + "."
	case token != "":
		return "The token stored here is used over " + source + "."
	default:
		return "Taken from " + source + ". One typed here is used instead, " + typedTokenKept + "."
	}
}

// typedTokenKept says where a Jira token typed into Settings goes.
const typedTokenKept = "kept in your keychain for this address on macOS or in the file elsewhere"

// tokenSource names the source the Jira token is read from when the file holds
// none: the keychain over the variable over the command, or none of them.
func tokenSource(held func(string) any) string {
	keychain, _ := held("jira.keychain").(bool)
	variable, _ := held("jira.token_env").(string)
	command, _ := held("jira.token_command").(string)

	switch {
	case keychain:
		return "your keychain, for this address"
	case variable != "":
		return "token_env: " + sanitize.Line(variable)
	case command != "":
		return "token_command: " + sanitize.Line(command)
	default:
		return ""
	}
}

// jiraSettings are where the tracker is and who reads it, and the views and
// headers as the form holds them.
func jiraSettings(noun string, read map[string]any, held func(string) any) []setting {
	const section = "Jira"

	views := collection{title: "View", noun: "view", nameWord: entryName, valueWord: "JQL", field: "jql"}
	lists := slices.Concat(
		views.rows(section, "jira.views", "The issue lists v moves between, each a name and its JQL. "+
			"None keeps the one built-in list: open issues assigned to you.", held("jira.views")),
		headerCollection().rows(section, "jira.headers", "Sent with every Jira request, for a proxy that "+
			"wants one. Each value is a credential: leave it empty to keep it.", held("jira.headers")),
	)

	return slices.Concat([]setting{
		{section: section, label: "Base URL", path: "jira.base_url", kind: settingURL},
		{section: section, label: "Token", path: "jira.token", kind: settingSecret, hint: tokenHint(read, held)},
		{section: section, label: keychainLabel, path: "jira.keychain", kind: settingToggle},
		{section: section, label: "User", path: "jira.user", hint: "Empty authenticates with the token as a bearer."},
		{section: section, label: "Project", path: "jira.project"},
		{
			section: section, label: "Review status", path: "jira.review_status",
			hint: `The status an issue moves to once its ` + noun + ` is open, e.g. "In Review". Empty makes no offer.`,
		},
	}, lists, []setting{
		{
			section: section, label: "Write comments in Markdown, posted as Jira wiki markup",
			path: "jira.markdown_comments", kind: settingToggle,
		},
		{
			section: section, label: "List this repository's GitHub or GitLab issues beside Jira's",
			path: "issues.forge", kind: settingToggle,
		},
	})
}

// messagingSettings are where announcements go.
func messagingSettings() []setting {
	const section = "Messaging"

	return []setting{
		{
			section: section, label: "Service", path: "messaging.kind", kind: settingChoice,
			hint: "Slack posts with a user token or a webhook, not both; the others post over a webhook.",
			choices: []option{
				{"", "Slack (default)"},
				{"teams", "Microsoft Teams"},
				{"discord", "Discord"},
				{"webhook", "Plain webhook"},
			},
		},
		{
			section: section, label: "Client ID", path: "messaging.client_id",
			hint: "Slack user token: your app's client ID, from its Basic Information page.",
		},
		{
			section: section, label: "Client secret", path: "messaging.client_secret", kind: settingSecret,
			hint: "Slack user token: your app's client secret; leave empty to keep the stored one.",
		},
		{
			section: section, label: "Refresh token", path: "messaging.refresh_token", kind: settingSecret,
			hint: "Slack user token: a refresh token (xoxe-1-…); workflow refreshes it and keeps each new one.",
		},
		{
			section: section, label: "Webhook URL", path: "messaging.webhook_url", kind: settingSecret,
			hint: "A credential; leave empty to keep it. For Slack, set this or the user token, not both.",
		},
		{
			section: section, label: "Channel", path: "messaging.channel",
			hint: "With a Slack user token; a webhook posts to its own channel.",
		},
		{
			section: section, label: "Channels", path: "messaging.channels", kind: settingList,
			hint: "Comma-separated: further channels an announcement can go to instead, with a Slack user token.",
		},
		{
			section: section, label: "Announcement", path: "messaging.announcement",
			hint: "Slack only: the review message, from {author}, {noun}, {title}, {url}, {key}, {summary} " +
				"and {issue_url}. Empty keeps the built-in message.",
		},
	}
}

// forgeSettings are which forge the pull requests live on and how to reach it.
func forgeSettings() []setting {
	const section = "Forge"

	return []setting{
		{
			section: section, label: "Kind", path: "forge.kind", kind: settingChoice,
			choices: []option{{"", "Auto-detect"}, {"github", "GitHub"}, {"gitlab", "GitLab"}},
		},
		{section: section, label: "Host", path: "forge.host"},
		{
			section: section, label: "Token", path: "forge.token", kind: settingSecret,
			hint: "Leave empty to keep the stored token.",
		},
		{section: section, label: "Use the forge CLI for authentication", path: "forge.cli", kind: settingToggle},
	}
}

// commitSettings are the commit convention.
func commitSettings() []setting {
	const section = "Commit"

	defaults := convention.DefaultCommitConvention()

	return []setting{
		{
			section: section, label: "Default scope", path: "commit.default_scope",
			hint: "Pre-fills the scope field until a commit here uses a scope of its own.",
		},
		{
			section: section, label: "Types", path: "commit.types", kind: settingList,
			hint: "Comma-separated, in the order to offer them; empty keeps the Conventional Commit types.",
		},
		{
			section: section, label: "Subject limit", path: "commit.subject_limit", kind: settingCount,
			hint: "The longest a subject may be, in characters; 0 keeps " +
				strconv.Itoa(defaults.SubjectLimit()) + ".",
		},
		{
			section: section, label: "Issue trailer", path: "commit.refs_trailer",
			hint: "The trailer label added to a commit body; empty keeps " +
				strconv.Quote(defaults.RefsLabel()) + ".",
		},
	}
}

// branchSettings are how a branch is named for an issue, and the prefixes as
// the form holds them.
func branchSettings(held func(string) any) []setting {
	const section = "Branch"

	defaults := convention.DefaultBranchNaming()

	prefixes := collection{
		title: "Prefix", noun: "prefix", nameWord: "issue type", valueWord: "prefix",
		matchedBy: convention.IssueTypeKey,
	}

	return slices.Concat([]setting{
		{
			section: section, label: "Name template", path: "branch.template",
			hint: "Uses {prefix}, {key} and {slug}; must contain {key}.",
		},
		{
			section: section, label: "Default prefix", path: "branch.default_prefix",
			hint: "The prefix for an unmapped type; empty keeps " + strconv.Quote(defaults.DefaultPrefix()) + ".",
		},
	}, prefixes.rows(section, "branch.prefixes", "The prefix for each issue type, matched without regard "+
		"to case. Any here replace the built-in ones.", held("branch.prefixes")), []setting{
		{
			section: section, label: "Slug limit", path: "branch.slug_limit", kind: settingCount,
			hint: "Caps the summary slug's length; 0 keeps " + strconv.Itoa(defaults.SlugLimit()) + ".",
		},
	})
}

// pullRequestSettings are where a pull request's title comes from.
func pullRequestSettings(noun string) []setting {
	return []setting{
		{
			section: "Pull request", label: "Title source", path: "pull_request.title_source", kind: settingChoice,
			hint: "Where a " + noun + "'s title comes from.",
			choices: []option{
				{"", "The branch's oldest commit (default)"}, {"issue", "The issue it names"},
			},
		},
	}
}

// storeAndTaskwarriorSettings are whether anything is kept on disk, and which
// task program is Taskwarrior.
func storeAndTaskwarriorSettings() []setting {
	const restarts = " A change here applies when workflow restarts."

	return []setting{
		{section: "Store", label: "Keep nothing on disk between sessions", path: "store.disabled", kind: settingToggle},
		{
			section: "Taskwarrior", label: "Task program", path: "taskwarrior.program",
			hint: "Empty tries every task in an absolute PATH directory and keeps the first that is " +
				"Taskwarrior 3.5.0 or newer." + restarts,
		},
		{
			section: "Taskwarrior", label: "Turn off the Taskwarrior integration", path: "taskwarrior.disabled",
			kind: settingToggle, hint: strings.TrimSpace(restarts),
		},
	}
}

// timingSettings are how long workflow waits.
func timingSettings() []setting {
	const section = "Timing"

	return []setting{
		{
			section: section, label: "Request timeout", path: "timing.request_timeout",
			hint: "How long each request to a service may take, as a duration such as 20s; empty keeps " +
				config.DefaultRequestTimeout.String() + ".",
		},
		{
			section: section, label: "CI interval", path: "timing.ci_interval",
			hint: "How often CI is asked about while it runs, as a duration such as 30s; empty keeps " +
				config.DefaultCIInterval.String() + ".",
		},
	}
}

// terminalSettings are how the terminal interface draws and behaves.
func terminalSettings() []setting {
	const (
		section  = "Terminal"
		nextOpen = "A change applies when the interface next opens."
	)

	return []setting{
		{section: section, label: "Draw in plain ASCII", path: "ui.ascii", kind: settingToggle, hint: nextOpen},
		{section: section, label: "Capture the mouse", path: "ui.mouse", kind: settingToggle, hint: nextOpen},
		{
			section: section, label: "Color", path: "ui.color", kind: settingChoice,
			hint:    "Never keeps bold, faint and the reverse-video cursor, which mean the same without color. " + nextOpen,
			choices: []option{{"", "When the terminal allows (default)"}, {"never", "Never"}},
		},
		{section: section, label: "Ring the terminal when CI finishes", path: "ui.notify", kind: settingToggle},
		{
			section: section, label: "Comments shown", path: "ui.comments_shown", kind: settingCount,
			hint: "How many of an issue's latest comments the detail shows; 0 keeps " +
				strconv.Itoa(defaultCommentsShown) + ".",
		},
	}
}

// keyboardSettings are the web page's single-key shortcuts, and the key each
// action is moved to.
func keyboardSettings(actions []seams.KeyAction) []setting {
	const section = "Keyboard"

	return append([]setting{
		{
			section: section, label: "Single-key shortcuts on the web page", path: "ui.web_shortcuts",
			kind: settingToggle,
		},
	}, keysOf(actions).rows(section, "ui.keys", "The key the action is moved to; empty keeps its default. "+
		"A key two actions would share where both work is refused when you save.", nil)...)
}

// verb is what enter does to a setting.
func (s setting) verb() string {
	verb := "edit"

	switch s.kind {
	case settingToggle:
		verb = "turn on or off"
	case settingChoice:
		verb = "change"
	case settingAdd:
		verb = "add"
	case settingText, settingURL, settingSecret, settingCount, settingList, settingEntry:
	}

	return verb
}
