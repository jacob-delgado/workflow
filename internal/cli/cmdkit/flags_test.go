// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cmdkit_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/config"
)

func TestPostingInPostsWhereTheChannelSays(t *testing.T) {
	t.Parallel()

	const dev = "#dev"

	webhook := config.Messaging{WebhookURL: "https://hooks.slack.example/services/not-real", Channel: dev}

	cases := map[string]struct {
		settings config.Messaging
		channel  string
		want     string
		refused  bool
	}{
		"no channel leaves the settings":     {settings: webhook, channel: "  ", want: dev},
		"a channel where none is set up":     {settings: config.Messaging{}, channel: " #release ", want: "#release"},
		"a channel a webhook cannot post to": {settings: webhook, channel: "#release", want: dev, refused: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			posting, err := cmdkit.PostingIn(tt.settings, tt.channel)

			// Assert
			if posting.Channel != tt.want || errors.Is(err, cmdkit.ErrUsage) != tt.refused {
				t.Errorf("%s = %q, %v; want %q, refused as usage: %v", name, posting.Channel, err, tt.want, tt.refused)
			}
		})
	}
}

func TestNoMessagingSaysHowToSetItUp(t *testing.T) {
	t.Parallel()

	// Act
	err := cmdkit.MessagingNotConfigured()

	// Assert
	if !errors.Is(err, cmdkit.ErrMessagingNotConfigured) || !strings.Contains(err.Error(), "workflow slack login") {
		t.Errorf("no messaging = %v, want it refused, naming how to set Slack up", err)
	}
}

func TestCheckPortRefusesAPortNothingListensOn(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args []string
		why  string
	}{
		"the default port, not serving":   {args: nil},
		"a port, serving":                 {args: []string{webFlag, portFlag, "8080"}},
		"a port, not serving":             {args: []string{portFlag, "8080"}, why: "it takes effect only with --web"},
		"a port below the first, serving": {args: []string{webFlag, portFlag, "0"}, why: "it must be from 1 to 65535"},
		"a port past the last, serving":   {args: []string{webFlag, portFlag, "65536"}, why: "it must be from 1 to 65535"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			env, _ := inATempDir(t)
			root := probeTree(env, func(cmd *cobra.Command, _ []string) error {
				web, _ := cmd.Flags().GetBool("web")
				port, _ := cmd.Flags().GetInt("port")

				return cmdkit.CheckPort(cmd, web, port)
			})

			// Act
			_, err := runTree(t, root, tt.args...)

			// Assert
			if tt.why == "" && err != nil {
				t.Errorf("%s = %v, want the port taken", name, err)
			}

			if tt.why != "" && (!errors.Is(err, cmdkit.ErrUsage) || !strings.Contains(err.Error(), tt.why)) {
				t.Errorf("%s = %v, want a usage error saying %q", name, err, tt.why)
			}
		})
	}
}

func TestTheRootServesOnThePortItIsHandedUnlessToldOtherwise(t *testing.T) {
	t.Parallel()

	// Arrange
	env, _ := inATempDir(t)

	var port int

	root := probeTree(env, func(cmd *cobra.Command, _ []string) error {
		port, _ = cmd.Flags().GetInt("port")

		return nil
	})

	// Act
	_, err := runTree(t, root)

	// Assert
	if err != nil || port != probePort {
		t.Errorf("bare root = %v, serving on %d; want %d, the port it was handed", err, port, probePort)
	}
}

func TestTheOldLogFlagHoldsNoValueOfItsOwn(t *testing.T) {
	t.Parallel()

	// Arrange
	env, _ := inATempDir(t)
	root := probeTree(env, func(*cobra.Command, []string) error { return nil })

	// Act
	old := root.PersistentFlags().Lookup("log")

	// Assert
	if old == nil || !old.Hidden || old.Value.String() != "" || old.Value.Type() != "string" {
		t.Errorf("--log = %+v, want it hidden, holding no value, read as the string it took", old)
	}
}
