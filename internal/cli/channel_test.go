// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// summary --post posts in the channel --channel names, in place of
// messaging.channel, where a Slack user token can post anywhere; a webhook,
// bound to its channel, refuses it. announce's own are internal/cli/pr's.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestSummaryRefusesAChannelAWebhookCannotHonor(t *testing.T) {
	t.Parallel()

	// Arrange
	hook := clitest.NewCapturingWebhook(t)
	repo := workedRepository(t, "Add the widget")
	clitest.WriteFile(t, repo, `{"messaging":{"webhook_url":"`+hook.URL+`"}}`)

	// Act
	_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "summary", "--from", summaryDay, "--to", summaryDay,
		"--post", "--yes", "--channel", "#x")

	// Assert
	if !strings.Contains(fmt.Sprint(err), `"--channel"`) || len(hook.Posted()) != 0 {
		t.Errorf("summary --post --channel through a webhook = %v, posted %q; want it refused, naming --channel",
			err, hook.Posted())
	}

	wantExit(t, err, 2)
}

func TestSummaryPostsInTheChannelItIsGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	slack := clitest.FakeSlack(t, nil)
	repo := workedRepository(t, "Add the widget")
	clitest.WriteFile(t, repo, clitest.SlackLoggedInConfig())

	// Act
	printed, err := clitest.RunStreams(
		t, repo, clitest.UnusedPrompt(t), "summary", "--from", summaryDay, "--to", summaryDay,
		"--post", "--yes", "--channel", "#standup",
	)
	// Assert
	if err != nil {
		t.Fatalf("summary --post --channel: %v (%+v)", err, printed)
	}

	if posted := slack.Channel(t); posted != "#standup" {
		t.Errorf("summary --post --channel '#standup' posted in %q, want #standup in place of #dev", posted)
	}
}
