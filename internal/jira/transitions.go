// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Transition is a move Jira's workflow offers an issue from its current status.
// Its name is the workflow's verb ("Start Progress") and is rarely the status it
// leads to, so both are kept.
type Transition struct {
	ID               string
	Name             string
	ToStatus         string
	ToStatusCategory StatusCategory
	// Fields are those Jira will refuse this transition without.
	Fields []Field
}

// transitionsAnswer is the wire shape, decoded and then flattened.
type transitionsAnswer struct {
	Transitions []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		To   struct {
			Name     string `json:"name"`
			Category struct {
				Key string `json:"key"`
			} `json:"statusCategory"` //nolint:tagliatelle // Jira's field name on the wire, not ours to pick
		} `json:"to"`
		Fields map[string]wireField `json:"fields"`
	} `json:"transitions"`
}

// transitionRequest is the body that applies a transition.
type transitionRequest struct {
	Transition reference      `json:"transition"`
	Fields     map[string]any `json:"fields,omitempty"`
}

// Transitions lists the moves the credential may make on an issue right now.
func (c Client) Transitions(ctx context.Context, issueKey Key) ([]Transition, error) {
	// The fields are expanded so a transition that needs a resolution, say, can
	// ask for one — rather than being sent without it and refused.
	query := url.Values{"expand": {"transitions.fields"}}.Encode()

	request, err := c.newRequest(ctx, http.MethodGet, transitionsPath(issueKey)+"?"+query, nil)

	answer, err := decode[transitionsAnswer](c, request, err)

	return answer.result(), err
}

// ApplyTransition moves an issue through a transition Transitions offered, with
// a value for each field it needs.
func (c Client) ApplyTransition(ctx context.Context, issueKey Key, to Transition, values []FieldValue) error {
	payload, err := json.Marshal(transitionRequest{Transition: reference{ID: to.ID}, Fields: fieldsPayload(values)})
	if err != nil {
		return fmt.Errorf("encoding the transition: %w", err)
	}

	request, err := c.newRequest(ctx, http.MethodPost, transitionsPath(issueKey), bytes.NewReader(payload))
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	// Data Center answers 204 with no body: accepted is all there is to know.
	_, err = c.exchange(request)

	return err
}

// transitionsPath is where an issue's transitions live.
func transitionsPath(issueKey Key) string {
	return issuePath(issueKey) + "/transitions"
}

// result flattens the wire shape.
func (a transitionsAnswer) result() []Transition {
	found := make([]Transition, 0, len(a.Transitions))

	for _, wire := range a.Transitions {
		found = append(found, Transition{
			ID:               wire.ID,
			Name:             wire.Name,
			ToStatus:         wire.To.Name,
			ToStatusCategory: StatusCategory(wire.To.Category.Key),
			Fields:           requiredFields(wire.Fields),
		})
	}

	return found
}
