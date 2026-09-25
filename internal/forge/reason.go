// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// reasonLimit bounds how much of a refusal is read for its reason.
const reasonLimit = 64 << 10

// wireReason is how either forge explains a refusal. GitHub writes a message
// and a list of errors, each with its own message; GitLab writes a message that
// is a string or a list of strings, or an error.
type wireReason struct {
	Message json.RawMessage `json:"message"`
	Error   string          `json:"error"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// explained adds the forge's own reason to a status error, when the status is
// one the forge explains: a 403 refusing this token keeps its refusal with the
// reason after it, and any other request the forge understood and turned down,
// such as a pull request that already exists, becomes a rejection. 401 and 404
// keep their own errors.
func explained(statusErr error, body io.Reader) error {
	if !errors.Is(statusErr, ErrRefused) && !errors.Is(statusErr, ErrUnexpectedStatus) {
		return statusErr
	}

	reason, given := reasonIn(body)
	if !given {
		return statusErr
	}

	if errors.Is(statusErr, ErrRefused) {
		return fmt.Errorf("%w: %s", statusErr, reason)
	}

	return fmt.Errorf("%w: %s", ErrRejected, reason)
}

// reasonIn reads the forge's own reason out of a refusal's body, reporting
// false when it gave none that can be read.
func reasonIn(body io.Reader) (string, bool) {
	raw, err := io.ReadAll(io.LimitReader(body, reasonLimit))
	if err != nil {
		return "", false
	}

	var answer wireReason

	err = json.Unmarshal(sanitize.JSON(raw), &answer)
	if err != nil {
		return "", false
	}

	reason := answer.text()

	return reason, reason != ""
}

// text joins every part of the reason that was given.
func (w wireReason) text() string {
	parts := messages(w.Message)

	if w.Error != "" {
		parts = append(parts, w.Error)
	}

	for _, each := range w.Errors {
		if each.Message != "" {
			parts = append(parts, each.Message)
		}
	}

	return strings.Join(parts, ": ")
}

// messages reads a message that may be one string or a list of them.
func messages(raw json.RawMessage) []string {
	var one string
	if json.Unmarshal(raw, &one) == nil && one != "" {
		return []string{one}
	}

	var many []string
	if json.Unmarshal(raw, &many) == nil {
		return many
	}

	return nil
}
