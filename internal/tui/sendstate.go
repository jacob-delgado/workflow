// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

// sendState is an outbound request: whether it is in flight, and the error it
// came back with. Every overlay that posts, applies or writes carries one, and
// so does the Messaging pane's own post, so "in flight, then failed with this"
// is written and named one way rather than as a sending bool beside an err, an
// applyErr or a problem.
type sendState struct {
	sending bool
	err     error
}

// starting marks a request as in flight, clearing any earlier error.
func starting() sendState {
	return sendState{sending: true}
}

// failed records that the request came back with err, and is no longer in
// flight.
func (s sendState) failed(err error) sendState {
	return sendState{sending: false, err: err}
}
