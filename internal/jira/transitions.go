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
	ToStatusCategory string
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
	} `json:"transitions"`
}

// transitionRequest is the body that applies a transition.
type transitionRequest struct {
	Transition transitionID `json:"transition"`
}

// transitionID names a transition on the wire.
type transitionID struct {
	ID string `json:"id"`
}

// Transitions lists the moves the credential may make on an issue right now.
func (c Client) Transitions(ctx context.Context, issueKey string) ([]Transition, error) {
	request, err := c.newRequest(ctx, http.MethodGet, transitionsPath(issueKey), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.exchange(request)
	if err != nil {
		return nil, err
	}

	var answer transitionsAnswer

	err = json.Unmarshal(body, &answer)
	if err != nil {
		return nil, fmt.Errorf("reading the answer from %s: %w", c.settings.BaseURL, err)
	}

	return answer.result(), nil
}

// ApplyTransition moves an issue through a transition Transitions offered.
func (c Client) ApplyTransition(ctx context.Context, issueKey string, to Transition) error {
	payload, err := json.Marshal(transitionRequest{Transition: transitionID{ID: to.ID}})
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

// transitionsPath is where an issue's transitions live. The key is escaped
// because it is text a server supplied: it must stay one path segment.
func transitionsPath(issueKey string) string {
	return "/rest/api/2/issue/" + url.PathEscape(issueKey) + "/transitions"
}

// result flattens the wire shape.
func (a transitionsAnswer) result() []Transition {
	found := make([]Transition, 0, len(a.Transitions))

	for _, wire := range a.Transitions {
		found = append(found, Transition{
			ID:               wire.ID,
			Name:             wire.Name,
			ToStatus:         wire.To.Name,
			ToStatusCategory: wire.To.Category.Key,
		})
	}

	return found
}
