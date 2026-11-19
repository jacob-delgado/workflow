// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"

	"github.com/jacob-delgado/workflow/internal/api"
)

// UpdateConfig writes the configuration file, then reopens the server with
// it, as a switch wires a directory, so every setting saved applies at once.
// The save holds the write gate shared, as every other write does; the reopen
// holds it alone, so one made while another write is in flight is not made,
// nor one the directory cannot be wired again for, and Workflow-Reopened says
// so.
func (s *server) UpdateConfig(
	ctx context.Context, request api.UpdateConfigRequestObject,
) (api.UpdateConfigResponseObject, error) {
	saver, answer := s.saveShared(ctx, request)

	written, saved := answer.(api.UpdateConfig200JSONResponse)
	if !saved {
		return answer, nil
	}

	written.Headers.WorkflowReopened = saver.reopenWithSaved(ctx)

	return written, nil
}

// saveShared saves by the server in effect once the gate is held shared — a
// switch may have put another in place since the request arrived — and
// answers that server with the save's answer.
func (s *server) saveShared(
	ctx context.Context, request api.UpdateConfigRequestObject,
) (*server, api.UpdateConfigResponseObject) {
	s.worlds.gate.RLock()
	defer s.worlds.gate.RUnlock()

	current, _ := s.worlds.served()

	return current, current.updateConfig(ctx, request)
}

// reopenWithSaved serves the file just saved from now on, wiring the
// directory again as a switch does under ctx, and reports whether it could:
// not while another write is in flight, nor where the server cannot wire a
// directory again. A switch that landed after the save reports true: it
// wired its directory from the files on disk after the write, so there is
// nothing left to reopen.
func (s *server) reopenWithSaved(ctx context.Context) bool {
	_, err := s.worlds.takeUp(ctx, s)
	s.noteTakeUp(err)

	return err == nil
}
