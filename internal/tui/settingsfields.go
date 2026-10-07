// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// settingKind is how a setting is shown and edited.
type settingKind int

// The kinds of setting: typed text; a URL whose userinfo is a credential, so
// it is shown masked; a credential, never shown; a count; a comma-separated
// list; one of a fixed set; and one that is on or off.
const (
	settingText settingKind = iota
	settingURL
	settingSecret
	settingCount
	settingList
	settingChoice
	settingToggle
)

// option is a value a setting offers, beside the words it is offered in.
type option struct {
	value, words string
}

// setting is one row of Settings: the section it is under, its label, its
// path in the configuration's JSON form, what it is, and what it does.
type setting struct {
	section, label, path, hint string
	kind                       settingKind
	choices                    []option
}

// words is how a choice's value reads, an unset one as the first choice: the
// default the configuration takes. A file holding any other is refused as
// it is read.
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

// settingsFields are the settings the web's Settings edits, in its order,
// labels and hints, for the configuration read, in its JSON form; noun is the
// forge's word for a pull request.
func settingsFields(noun string, read map[string]any) []setting {
	return slices.Concat(jiraSettings(noun, read), messagingSettings(), forgeSettings(), commitSettings(),
		branchSettings(), pullRequestSettings(noun), storeAndTaskwarriorSettings())
}

// tokenHint says where the Jira token comes from: the file's own token wins,
// then token_env, then token_command, so a token typed over a source sends the
// secret into the file the source kept it out of. Neither source is a secret.
func tokenHint(read map[string]any) string {
	token, _ := valueAt(read, "jira.token").(string)
	command, _ := valueAt(read, "jira.token_command").(string)
	variable, _ := valueAt(read, "jira.token_env").(string)

	var source string

	switch {
	case variable != "":
		source = "token_env: " + sanitize.Line(variable)
	case command != "":
		source = "token_command: " + sanitize.Line(command)
	case token != "":
		return "Leave empty to keep the stored token."
	default:
		return "A personal access token. token_command or token_env, set in the file, keep it out of the file."
	}

	if token != "" {
		return "The token stored here is used over " + source + "."
	}

	return "Taken from " + source + ". A token typed here is kept in the file and used instead."
}

// jiraSettings are where the tracker is and who reads it.
func jiraSettings(noun string, read map[string]any) []setting {
	const section = "Jira"

	return []setting{
		{section: section, label: "Base URL", path: "jira.base_url", kind: settingURL},
		{section: section, label: "Token", path: "jira.token", kind: settingSecret, hint: tokenHint(read)},
		{section: section, label: "User", path: "jira.user", hint: "Empty authenticates with the token as a bearer."},
		{section: section, label: "Project", path: "jira.project"},
		{
			section: section, label: "Review status", path: "jira.review_status",
			hint: `The status an issue moves to once its ` + noun + ` is open, e.g. "In Review". Empty makes no offer.`,
		},
		{
			section: section, label: "Write comments in Markdown, posted as Jira wiki markup",
			path: "jira.markdown_comments", kind: settingToggle,
		},
		{
			section: section, label: "List this repository's GitHub or GitLab issues beside Jira's",
			path: "issues.forge", kind: settingToggle,
		},
	}
}

// messagingSettings are where announcements go.
func messagingSettings() []setting {
	const section = "Messaging"

	return []setting{
		{
			section: section, label: "Service", path: "messaging.kind", kind: settingChoice,
			hint: "Slack posts with a user token or a webhook, not both; the others post over a webhook.",
			choices: []option{
				{"slack", "Slack"}, {"teams", "Microsoft Teams"}, {"discord", "Discord"}, {"webhook", "Plain webhook"},
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
			hint: "The longest a subject may be, in characters; 0 keeps 72.",
		},
		{
			section: section, label: "Issue trailer", path: "commit.refs_trailer",
			hint: `The trailer label added to a commit body; empty keeps "Refs".`,
		},
	}
}

// branchSettings are how a branch is named for an issue.
func branchSettings() []setting {
	const section = "Branch"

	return []setting{
		{
			section: section, label: "Name template", path: "branch.template",
			hint: "Uses {prefix}, {key} and {slug}; must contain {key}.",
		},
		{
			section: section, label: "Default prefix", path: "branch.default_prefix",
			hint: `The prefix for an unmapped type; empty keeps "feat".`,
		},
		{
			section: section, label: "Slug limit", path: "branch.slug_limit", kind: settingCount,
			hint: "Caps the summary slug's length; 0 keeps 48.",
		},
	}
}

// pullRequestSettings are where a pull request's title comes from.
func pullRequestSettings(noun string) []setting {
	return []setting{
		{
			section: "Pull request", label: "Title source", path: "pull_request.title_source", kind: settingChoice,
			hint: "Where a " + noun + "'s title comes from.",
			choices: []option{
				{"commit", "The branch's oldest commit"}, {"issue", "The issue it names"},
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

// typedValue is what text typed into a setting stands for: the text, a count,
// or a list's entries; and false for a credential left empty, which keeps the
// stored one.
func typedValue(field setting, text string) (any, bool, error) {
	switch field.kind {
	case settingSecret:
		return text, text != "", nil
	case settingCount:
		count, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || count < 0 {
			return nil, false, errNotACount
		}

		// A count reads back from JSON as a float64, so it is kept as one.
		return float64(count), true, nil
	case settingList:
		return splitList(text), true, nil
	case settingText, settingURL, settingChoice, settingToggle:
		return text, true, nil
	}

	return text, true, nil
}

// editedText is a setting's value as its field starts: empty for a
// credential, which is never shown, and the value as shown otherwise.
func (f settingsForm) editedText(field setting) string {
	if field.kind == settingSecret {
		return ""
	}

	return f.shown(field)
}

// edited is the configuration as read with the edits laid over it, checked
// as a file on disk is.
func (f settingsForm) edited() ([]byte, error) {
	values := maps.Clone(f.values)

	for path, value := range f.edits {
		section, name, _ := strings.Cut(path, ".")

		// Every section is in the configuration's JSON form, which encodes
		// each one whole; the section is copied, so the read stays as read.
		inner, _ := values[section].(map[string]any)
		inner = maps.Clone(inner)
		inner[name] = value
		values[section] = inner
	}

	// Trade-off TRADE-13: the configuration's JSON form always encodes.
	edited, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("writing the configuration: %w", err)
	}

	return edited, nil
}

// seed is a configuration's JSON form, read back as values, or why it could
// not be had.
func seed(cfg config.Config, err error) (map[string]any, error) {
	if err != nil {
		return nil, err
	}

	// Trade-off TRADE-13: a Config always encodes, and its JSON always reads
	// back.
	var values map[string]any

	read, err := json.Marshal(cfg)
	if err == nil {
		err = json.Unmarshal(read, &values)
	}

	if err != nil {
		return nil, fmt.Errorf("reading the configuration: %w", err)
	}

	return values, nil
}

// valueAt is the value at a dotted path in a configuration's JSON form, or
// nil where there is none.
func valueAt(values map[string]any, path string) any {
	section, name, _ := strings.Cut(path, ".")
	inner, _ := values[section].(map[string]any)

	return inner[name]
}

// shown is a setting's value as the screen may show it: a credential masked,
// whatever was typed for it.
func (f settingsForm) shown(field setting) string {
	value := f.value(field.path)
	text, _ := value.(string)

	switch field.kind {
	case settingText, settingToggle:
		return sanitize.Line(text)
	case settingSecret:
		if _, typed := f.edits[field.path]; typed {
			return "new value, hidden"
		}

		return config.Redact(text)
	case settingURL:
		return sanitize.Line(config.RedactURL(text))
	case settingCount:
		count, _ := value.(float64)

		return strconv.Itoa(int(count))
	case settingList:
		return sanitize.Line(strings.Join(listOf(value), ", "))
	case settingChoice:
		return field.words(text)
	}

	return sanitize.Line(text)
}

// listOf is a list setting's entries, as read or as typed.
func listOf(value any) []string {
	if typed, ok := value.([]string); ok {
		return typed
	}

	read, _ := value.([]any)
	entries := make([]string, 0, len(read))

	for _, entry := range read {
		text, _ := entry.(string)
		entries = append(entries, text)
	}

	return entries
}

// verb is what enter does to a setting.
func (s setting) verb() string {
	verb := "edit"

	switch s.kind {
	case settingToggle:
		verb = "turn on or off"
	case settingChoice:
		verb = "change"
	case settingText, settingURL, settingSecret, settingCount, settingList:
	}

	return verb
}
