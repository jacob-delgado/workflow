// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira

import (
	"context"
	"net/http"
)

// Assign sets an issue's assignee to a user by their Data Center username. Jira
// answers 204 with no body, so a nil error is all there is to know.
func (c Client) Assign(ctx context.Context, issueKey Key, assignee string) error {
	request, err := c.newJSONRequest(ctx, http.MethodPut, issuePath(issueKey)+"/assignee", struct {
		Name string `json:"name"`
	}{Name: assignee})
	if err != nil {
		return err
	}

	_, err = c.exchange(request)

	return err
}
