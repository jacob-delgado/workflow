// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// errNoSuchChannel is Slack refusing a post to a channel it does not have.
var errNoSuchChannel = errors.New("channel_not_found")

// messagingDeps is the demo's Slack. Its directory is left unread, so an
// announcement tags no one.
func (w *World) messagingDeps() seams.Messaging {
	return seams.Messaging{Post: post}
}

// post sends text to a channel the demo's Slack has, the configured one when
// none is named; the message goes nowhere a later read could find it, as no
// seam reads a channel back.
func post(_ context.Context, to, _ string) error {
	channels := []string{"", channel, releases}
	if !slices.Contains(channels, to) {
		return fmt.Errorf("%s: %w: %w", to, messaging.ErrPostRefused, errNoSuchChannel)
	}

	return nil
}
