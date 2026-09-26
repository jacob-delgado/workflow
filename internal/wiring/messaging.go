// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"fmt"
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// messagingDeps is what a surface asks of the messaging service, each post
// reaching it through messagingClient, which finds a bot token the first time a
// post needs one.
func messagingDeps(ctx context.Context, messagingClient func() (messaging.Client, error)) seams.Messaging {
	return seams.Messaging{Post: func(channel, text string) error {
		client, err := messagingClient()
		if err != nil {
			return err
		}

		return client.Post(ctx, channel, text)
	}}
}

// connectMessaging builds the client that posts to the messaging service. Only
// a Slack bot token is looked for, running its command when one is set; the
// webhook kinds carry the credential in the URL. A token source that gives none
// is no credential at all, and is reported as such before anything is sent.
func connectMessaging(
	ctx context.Context, settings config.Messaging, httpTransport httpx.Doer, log *RequestLog,
) (messaging.Client, error) {
	// A webhook's path is its credential; only a bot posts to a route.
	wrap := log.wrapWebhook

	if settings.Mode() == config.MessagingBot {
		token, err := resolveSetToken(ctx, settings.Token, settings.TokenCommand, settings.TokenEnv)
		if err != nil {
			return messaging.Client{}, fmt.Errorf("%w: %w", messaging.ErrNoCredential, err)
		}

		settings.Token, wrap = token, log.Wrap
	}

	service := strings.ToLower(settings.Service())

	//nolint:bodyclose // wrap only relays the response; the messaging client reads and closes its body.
	return messaging.New(wrap(service, httpTransport), messaging.APIBase, settings), nil
}
