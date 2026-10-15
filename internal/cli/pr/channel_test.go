// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

// announce posts in the channel --channel names, in place of
// messaging.channel, where a Slack user token can post anywhere; a webhook,
// bound to its channel, refuses it.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestAnnouncePostsInTheChannelItIsGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	slack := clitest.FakeSlack(t, nil)
	repo := ownedRepo(t)
	clitest.WriteFile(t, repo, clitest.SlackLoggedInConfig())

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--yes", "--channel", "#release")
	// Assert
	if err != nil {
		t.Fatalf("announce --channel: %v (%+v)", err, printed)
	}

	if posted := slack.Channel(t); posted != "#release" || !strings.Contains(printed.Stderr, "Announced to #release.") {
		t.Errorf("announce --channel '#release' posted in %q, said %q; want #release in place of #dev",
			posted, printed.Stderr)
	}
}

func TestAnnounceWhenGreenPostsInTheChannelItIsGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI runs at the first read and has passed by the next, read every 10ms.
	clitest.FakeGh(t, clitest.GhResponses{
		Pulls: clitest.OpenPull("Add login"), StatusSequence: []string{clitest.RunningStatus(), clitest.PassingStatus()},
	})
	slack := clitest.FakeSlack(t, nil)
	repo := ownedRepo(t)
	clitest.WriteFile(t, repo, strings.TrimSuffix(clitest.SlackLoggedInConfig(), "}")+`,"timing":{"ci_interval":"10ms"}}`)

	// Act
	printed, err := clitest.RunStreams(
		t, repo, clitest.UnusedPrompt(t), "announce", "--when-green", "--yes", "--channel", "#release",
	)
	// Assert
	if err != nil {
		t.Fatalf("announce --when-green --channel: %v (%+v)", err, printed)
	}

	posted := slack.Channel(t)
	if posted != "#release" || !strings.Contains(printed.Stderr, "then announcing to #release") {
		t.Errorf("announce --when-green --channel '#release' posted in %q, said %q; want it to wait on, "+
			"then announce in, #release in place of #dev", posted, printed.Stderr)
	}
}

func TestAnnounceRefusesAChannelAWebhookCannotHonor(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	hook := clitest.NewCapturingWebhook(t)
	repo := ownedRepo(t)
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"},`+
		`"messaging":{"webhook_url":"`+hook.URL+`"}}`)

	// Act
	_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--yes", "--channel", "#x")

	// Assert
	if !strings.Contains(fmt.Sprint(err), `"--channel"`) || len(hook.Posted()) != 0 {
		t.Errorf("announce --channel through a webhook = %v, posted %q; want it refused, naming --channel",
			err, hook.Posted())
	}

	wantExit(t, err, 2)
}
