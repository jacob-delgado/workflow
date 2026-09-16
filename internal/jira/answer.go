// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
)

// anonymousUser is what Data Center reports in X-Ausername when it served a
// request without authenticating it.
const anonymousUser = "anonymous"

// reasonLimit bounds how much of a failed answer is read for its reason. Jira's
// are a few hundred bytes; anything longer is not one.
const reasonLimit = 64 << 10

// reasons is Jira's error body: what was wrong with the request as a whole, and
// what was wrong with each field.
//
//nolint:tagliatelle // Jira's field names on the wire, not ours to pick
type reasons struct {
	ErrorMessages []string          `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
}

// answerError translates an answer into an error, root cause first.
//
// A token Data Center does not accept is not refused: the request is served
// ANONYMOUSLY, and Jira says so only in X-Ausername. A search then answers 200
// with an empty list, which would read as "nothing assigned to you", and any
// failure explains what an anonymous user may not do — so the header is checked
// before the status says anything.
//
// After that, Jira's own reason beats the status wherever it gave one: "400"
// tells nobody what to change. 401 is the exception, because its status already
// says it and callers test for ErrUnauthorized by identity.
func (c Client) answerError(response *http.Response, requested *url.URL) error {
	if response.Header.Get("X-Ausername") == anonymousUser {
		return ErrUnauthorized
	}

	err := statusError(response.StatusCode, requested)
	if err == nil || errors.Is(err, ErrUnauthorized) {
		return err
	}

	reason := c.reason(response.Body)
	if reason == "" {
		return err
	}

	return fmt.Errorf("%w: %s", ErrRejected, reason)
}

// reason reads Jira's explanation from a failed answer, or "" when the body is
// not one — such as the HTML page of a proxy standing where Jira should be.
func (c Client) reason(body io.Reader) string {
	var answer reasons

	err := json.NewDecoder(io.LimitReader(body, reasonLimit)).Decode(&answer)
	if err != nil {
		return ""
	}

	explained := slices.Clone(answer.ErrorMessages)
	for _, field := range slices.Sorted(maps.Keys(answer.Errors)) {
		explained = append(explained, answer.Errors[field])
	}

	return c.masked(strings.Join(explained, "; "))
}

// masked hides the configured token in text a server chose, which is about to
// be printed. The token cannot be empty here — newRequest sends nothing without
// one — which matters, because replacing "" would splice the mask between every
// character.
func (c Client) masked(text string) string {
	return strings.ReplaceAll(text, c.settings.Token, config.Redact(c.settings.Token))
}
