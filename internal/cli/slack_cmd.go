// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/slackauth"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

var (
	// errLoginNeedsConfig reports a login with no configuration file to name
	// the Slack app in.
	errLoginNeedsConfig = errors.New("no " + config.FileName + " to log in with; run `workflow config init` first")
	// errLoginOverWebhook reports a login while Slack posts through a webhook,
	// which a user token cannot stand beside.
	errLoginOverWebhook = errors.New("slack posts through messaging.webhook_url here; " +
		"remove it first, since Slack posts with a user token or a webhook, not both")
	// errLoginNotSlack reports a login while messaging posts to another service.
	errLoginNotSlack = errors.New("messaging.kind is not slack; a user token is Slack's alone")
	// errLoginBlank reports an answer left blank that the login cannot do
	// without.
	errLoginBlank = errors.New("no answer given")
)

// newSlackCmd builds `workflow slack`, under which the Slack user token is set
// up.
func newSlackCmd(prompt Prompt) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slack",
		Short: "Set up posting to Slack with a rotating user token",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}

	cmd.AddCommand(newSlackLoginCmd(prompt))

	return cmd
}

// newSlackLoginCmd builds `workflow slack login`.
func newSlackLoginCmd(prompt Prompt) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Set up the Slack user token workflow posts with",
		Long: `Ask for your Slack app's client ID, its client secret and a refresh token
(xoxe-1-…), refresh the token once to prove them, and keep what Slack gives
back: in the macOS keychain, or in ` + config.FileName + ` on Linux and Windows or
when the file already holds them. The client ID is written to ` + config.FileName + `.

The app needs token rotation turned on and the chat:write user token scope.
Tagging people and user groups in an announcement is optional and also needs
users:read, channels:read, groups:read and usergroups:read; without them the
announcement posts untagged and says which scope to add. workflow refreshes the
token before its twelve hours run out, and keeps each new one where this put
the first.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, err := connect(cmd)
			if err != nil {
				return err
			}
			defer conn.closeLog()

			return slackLogin(cmd.Context(), outputOf(cmd).notes, prompt, conn.cfg, dryRunRequested(cmd))
		},
	}
}

// slackLogin sets up cfg's Slack user token, or says what it would do under a
// dry run. It makes no artifact, so all it says goes to notes.
func slackLogin(ctx context.Context, notes io.Writer, prompt Prompt, cfg config.Config, dryRun bool) error {
	err := loginAllowed(cfg)
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Fprintf(notes, "dry run: would ask for the Slack app's client ID and secret and a refresh token, "+
			"refresh it once, and keep it in %s; nothing was asked or written\n", wiring.SlackStore(cfg).Where())

		return nil
	}

	clientID, starting, err := askForLogin(prompt)
	if err != nil {
		return err
	}

	cfg, err = nameSlackApp(cfg, clientID)
	if err != nil {
		return err
	}

	return keepFirstToken(ctx, notes, cfg, starting)
}

// loginAllowed refuses a login the configuration cannot take.
func loginAllowed(cfg config.Config) error {
	switch {
	case cfg.Path == "":
		return errLoginNeedsConfig
	case cfg.Messaging.Kind != "" && cfg.Messaging.Kind != config.KindSlack:
		return errLoginNotSlack
	case cfg.Messaging.WebhookURL != "":
		return errLoginOverWebhook
	default:
		return nil
	}
}

// askForLogin asks for the app's client ID on screen, then its secret and the
// refresh token off it.
func askForLogin(prompt Prompt) (string, slackauth.Credentials, error) {
	clientID, err := askFor(prompt.Line, "Slack app client ID (Basic Information): ", "client ID")
	if err != nil {
		return "", slackauth.Credentials{}, err
	}

	secret, err := askFor(prompt.Secret, "Slack app client secret (not shown): ", "client secret")
	if err != nil {
		return "", slackauth.Credentials{}, err
	}

	refresh, err := askFor(prompt.Secret, "Refresh token, xoxe-1-… (not shown): ", "refresh token")
	if err != nil {
		return "", slackauth.Credentials{}, err
	}

	return clientID, slackauth.Credentials{ClientSecret: config.Secret(secret), RefreshToken: config.Secret(refresh)}, nil
}

// askFor asks question through ask and refuses a blank answer, naming what.
func askFor(ask func(string) (string, error), question, what string) (string, error) {
	answer, err := ask(question)
	if errors.Is(err, io.EOF) {
		return "", fmt.Errorf("%w; run it at a terminal, since it asks for secrets", errNoTerminal)
	}

	if err != nil {
		return "", err
	}

	answer = strings.TrimSpace(answer)
	if answer == "" {
		return "", fmt.Errorf("%w for the %s", errLoginBlank, what)
	}

	return answer, nil
}

// nameSlackApp writes the app's client ID, and the Slack kind, into the
// configuration file a save writes, over the revision it reads.
func nameSlackApp(cfg config.Config, clientID string) (config.Config, error) {
	held, revision, err := config.LoadLayersAt(cfg.Layers())
	if err != nil {
		return config.Config{}, err
	}

	held.Messaging.Kind, held.Messaging.ClientID = config.KindSlack, clientID

	_, err = config.SaveLayers(cfg.Layers(), held, revision)
	if err != nil {
		return config.Config{}, err
	}

	return held, nil
}

// keepFirstToken refreshes the starting credentials once, keeps what Slack
// gives back where cfg keeps a user token, and says whose token it is.
func keepFirstToken(ctx context.Context, notes io.Writer, cfg config.Config, starting slackauth.Credentials) error {
	transport := onlineDoer(cfg)
	store := wiring.SlackStore(cfg)

	renewed, err := wiring.SlackRefresher(cfg.Messaging.ClientID, transport).Refresh(ctx, starting)
	if err != nil {
		return err
	}

	err = store.Keep(ctx, renewed)
	if err != nil {
		return err
	}

	base, toSlack := wiring.SlackAPI(transport)

	identity, err := messaging.New(toSlack, base, cfg.Messaging).
		WithToken(wiring.SlackToken(cfg, transport)).AuthTest(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(notes, "Logged in to Slack as %s in %s. The token is kept in %s and refreshed before it expires.\n",
		identity.User, identity.Team, store.Where())

	return nil
}
