// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"regexp"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
)

// everySettingSet is a configuration with every setting Settings edits away
// from its default, each to a value of its own, and every toggle on.
func everySettingSet() config.Config {
	cfg := settingsFile()
	cfg.Jira = config.Jira{
		BaseURL: "https://jira.every.example", Token: "jira-every-1111", User: "every-user", Project: "EVERY",
		ReviewStatus: "Every Review", MarkdownComments: true,
		Views:   []config.JiraView{{Name: "every-view", JQL: "project = EVERY"}},
		Headers: map[string]config.Secret{"X-Every": "header-every-2222"},
	}
	cfg.Issues.Forge = true
	cfg.Messaging = config.Messaging{
		Kind: config.KindTeams, ClientID: "every-client", ClientSecret: "secret-every-3333",
		RefreshToken: "xoxe-every-4444", WebhookURL: "https://hooks.every.example/5555", Channel: "#every",
		Channels: []string{"#more-every"}, Announcement: "every {title}",
	}
	cfg.Forge = config.Forge{Kind: "gitlab", Host: "git.every.example", Token: "forge-every-6666", CLI: true}
	cfg.Commit = config.Commit{
		DefaultScope: "every-scope", Types: []string{"every"}, SubjectLimit: 99, RefsTrailer: "Every",
	}
	cfg.Branch = config.Branch{
		Template: "{prefix}/{key}-every-{slug}", DefaultPrefix: "everyfix", SlugLimit: 33,
		Prefixes: map[string]string{"bug": "everybug"},
	}
	cfg.PullRequest.TitleSource = string(convention.TitleFromIssue)
	cfg.Store.Disabled = true
	cfg.Taskwarrior = config.Taskwarrior{Program: "/opt/every/task", Disabled: true}
	cfg.Timing = config.Timing{RequestTimeout: "33s", CIInterval: "44s"}
	cfg.UI = config.UI{
		Mouse: true, ASCII: true, Color: "never", Notify: true, CommentsShown: 7, WebShortcuts: true,
		Keys: map[string]string{refreshAction: "ctrl+r"},
	}

	return cfg
}

func TestEverySettingReadsTheValueItsPathHolds(t *testing.T) {
	t.Parallel()

	// Arrange
	// Each row finds its value in the configuration's JSON form by a dotted
	// path the compiler cannot check, so a path that names no setting leaves
	// its row empty, or off, whatever the file holds.
	repo := newWorld()
	repo.settings = everySettingSet()

	// Act
	view := typing(t, repo.live(t, 160, 140), reposKey, settingsKey).View().Content

	// Assert
	const toggledOn = "[x]"

	plain := ansi.Strip(view)

	for _, row := range [][2]string{
		{"Base URL", "https://jira.every.example"},
		{"Token", "****1111"},
		{"User", "every-user"},
		{"Project", "EVERY"},
		{"Review status", "Every Review"},
		{"View every-view", "project = EVERY"},
		{"Header X-Every", "****2222"},
		{toggledOn, "Write comments in Markdown"},
		{toggledOn, "List this repository's GitHub or GitLab issues"},
		{"Service", "Microsoft Teams"},
		{"Client ID", "every-client"},
		{"Client secret", "****3333"},
		{"Refresh token", "****4444"},
		{"Webhook URL", "****5555"},
		{"Channel", "#every"},
		{"Channels", "#more-every"},
		{"Announcement", "every {title}"},
		{"Kind", "GitLab"},
		{"Host", "git.every.example"},
		{"Token", "****6666"},
		{toggledOn, "Use the forge CLI"},
		{"Default scope", "every-scope"},
		{"Types", "every"},
		{"Subject limit", "99"},
		{"Issue trailer", "Every"},
		{"Name template", "{prefix}/{key}-every-{slug}"},
		{"Default prefix", "everyfix"},
		{"Prefix bug", "everybug"},
		{"Slug limit", "33"},
		{"Title source", "The issue it names"},
		{toggledOn, "Keep nothing on disk"},
		{"Task program", "/opt/every/task"},
		{toggledOn, "Turn off the Taskwarrior integration"},
		{"Request timeout", "33s"},
		{"CI interval", "44s"},
		{toggledOn, "Draw in plain ASCII"},
		{toggledOn, "Capture the mouse"},
		{"Color", "Never"},
		{toggledOn, "Ring the terminal when CI finishes"},
		{"Comments shown", "7"},
		{toggledOn, "Single-key shortcuts on the web page"},
		{refreshAction, "ctrl+r"},
	} {
		shown := regexp.MustCompile(regexp.QuoteMeta(row[0]) + `\s+` + regexp.QuoteMeta(row[1]))
		if !shown.MatchString(plain) {
			t.Errorf("no row shows %q beside %q, as the configuration holds it", row[1], row[0])
		}
	}
}
