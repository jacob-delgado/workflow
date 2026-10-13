// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// announce and summary --post post in the channel --channel names, in place
// of messaging.channel, where a Slack user token can post anywhere; a webhook,
// bound to its channel, refuses it.

import (
	"fmt"
	"strings"
	"testing"
)

func TestAnnouncePostsInTheChannelItIsGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	slack := fakeSlack(t, nil)
	repo := ownedRepo(t)
	writeFile(t, repo, slackLoggedInConfig())

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "announce", "--yes", "--channel", "#release")
	// Assert
	if err != nil {
		t.Fatalf("announce --channel: %v (%+v)", err, printed)
	}

	if posted := slack.channel(t); posted != "#release" || !strings.Contains(printed.stderr, "Announced to #release.") {
		t.Errorf("announce --channel '#release' posted in %q, said %q; want #release in place of #dev",
			posted, printed.stderr)
	}
}

func TestAnnounceWhenGreenPostsInTheChannelItIsGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI runs at the first read and has passed by the next, read every 10ms.
	fakeGh(t, ghResponses{pulls: openPull("Add login"), statusSequence: []string{runningStatus(), passingStatus()}})
	slack := fakeSlack(t, nil)
	repo := ownedRepo(t)
	writeFile(t, repo, strings.TrimSuffix(slackLoggedInConfig(), "}")+`,"timing":{"ci_interval":"10ms"}}`)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "announce", "--when-green", "--yes", "--channel", "#release")
	// Assert
	if err != nil {
		t.Fatalf("announce --when-green --channel: %v (%+v)", err, printed)
	}

	posted := slack.channel(t)
	if posted != "#release" || !strings.Contains(printed.stderr, "then announcing to #release") {
		t.Errorf("announce --when-green --channel '#release' posted in %q, said %q; want it to wait on, "+
			"then announce in, #release in place of #dev", posted, printed.stderr)
	}
}

func TestAnnounceRefusesAChannelAWebhookCannotHonor(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	hook := newCapturingWebhook(t)
	repo := ownedRepo(t)
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"},`+
		`"messaging":{"webhook_url":"`+hook.url+`"}}`)

	// Act
	_, err := runStreams(t, repo, unusedPrompt(t), "announce", "--yes", "--channel", "#x")

	// Assert
	if !strings.Contains(fmt.Sprint(err), `"--channel"`) || len(hook.posted()) != 0 {
		t.Errorf("announce --channel through a webhook = %v, posted %q; want it refused, naming --channel",
			err, hook.posted())
	}

	wantExit(t, err, 2)
}

func TestSummaryRefusesAChannelAWebhookCannotHonor(t *testing.T) {
	t.Parallel()

	// Arrange
	hook := newCapturingWebhook(t)
	repo := workedRepository(t, "Add the widget")
	writeFile(t, repo, `{"messaging":{"webhook_url":"`+hook.url+`"}}`)

	// Act
	_, err := runStreams(t, repo, unusedPrompt(t), "summary", "--from", summaryDay, "--to", summaryDay,
		"--post", "--yes", "--channel", "#x")

	// Assert
	if !strings.Contains(fmt.Sprint(err), `"--channel"`) || len(hook.posted()) != 0 {
		t.Errorf("summary --post --channel through a webhook = %v, posted %q; want it refused, naming --channel",
			err, hook.posted())
	}

	wantExit(t, err, 2)
}

func TestSummaryPostsInTheChannelItIsGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	slack := fakeSlack(t, nil)
	repo := workedRepository(t, "Add the widget")
	writeFile(t, repo, slackLoggedInConfig())

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "summary", "--from", summaryDay, "--to", summaryDay,
		"--post", "--yes", "--channel", "#standup")
	// Assert
	if err != nil {
		t.Fatalf("summary --post --channel: %v (%+v)", err, printed)
	}

	if posted := slack.channel(t); posted != "#standup" {
		t.Errorf("summary --post --channel '#standup' posted in %q, want #standup in place of #dev", posted)
	}
}
