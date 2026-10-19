// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"github.com/jacob-delgado/workflow/internal/seams"
)

// messagingDeps is the demo's Slack. Its directory is left unread, so an
// announcement tags no one.
func (w *World) messagingDeps() seams.Messaging {
	return seams.Messaging{}
}
