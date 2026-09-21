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

// Worklog is a unit of work logged against an issue, as Jira stored it.
type Worklog struct {
	ID        string
	TimeSpent string
}

// wireWorklog is a worklog as the API returns it.
//
//nolint:tagliatelle // Jira's field names on the wire, not ours to pick
type wireWorklog struct {
	ID        string `json:"id"`
	TimeSpent string `json:"timeSpent"`
}

// worklog is the domain worklog for a wire one.
func (w wireWorklog) worklog() Worklog {
	return Worklog(w)
}

// AddWorklog logs work against an issue: how long was spent, in Jira's own
// duration form ("2h", "30m", "1d 4h"), and an optional note. It returns the
// worklog as Jira recorded it.
func (c Client) AddWorklog(ctx context.Context, issueKey Key, timeSpent, comment string) (Worklog, error) {
	payload, err := json.Marshal(struct {
		TimeSpent string `json:"timeSpent"` //nolint:tagliatelle // Jira's field name on the wire, not ours to pick
		Comment   string `json:"comment,omitempty"`
	}{TimeSpent: timeSpent, Comment: comment})
	if err != nil {
		return Worklog{}, fmt.Errorf("encoding the worklog: %w", err)
	}

	request, err := c.newRequest(ctx, http.MethodPost, issuePath(issueKey)+"/worklog", bytes.NewReader(payload))
	if err == nil {
		request.Header.Set("Content-Type", "application/json")
	}

	wire, err := decode[wireWorklog](c, request, err)
	if err != nil {
		return Worklog{}, err
	}

	return wire.worklog(), nil
}
