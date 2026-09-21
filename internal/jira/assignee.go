// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Assign sets an issue's assignee to a user by their Data Center username. Jira
// answers 204 with no body, so a nil error is all there is to know.
func (c Client) Assign(ctx context.Context, issueKey Key, assignee string) error {
	payload, err := json.Marshal(struct {
		Name string `json:"name"`
	}{Name: assignee})
	if err != nil {
		return fmt.Errorf("encoding the assignee: %w", err)
	}

	request, err := c.newRequest(ctx, http.MethodPut, issuePath(issueKey)+"/assignee", bytes.NewReader(payload))
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	_, err = c.exchange(request)

	return err
}
