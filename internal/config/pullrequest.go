// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"

	"github.com/jacob-delgado/workflow/internal/convention"
)

// ErrInvalidPullRequest reports a pull-request convention a caller could not honor.
var ErrInvalidPullRequest = errors.New("invalid pull request convention")

// PullRequest shapes the pull requests workflow proposes, so a team can match its
// own convention. Every field is optional; empty keeps the built-in default.
type PullRequest struct {
	// TitleSource decides where a pull request's title comes from: "commit" (the
	// default) takes the branch's oldest commit, "issue" takes the issue it names.
	TitleSource string `json:"title_source"`
}

// validatePullRequest refuses a title source that is neither commit nor issue.
func (c Config) validatePullRequest() error {
	switch convention.TitleSource(c.PullRequest.TitleSource) {
	case "", convention.TitleFromCommit, convention.TitleFromIssue:
		return nil
	default:
		return fmt.Errorf("%w: title_source is commit or issue: %q",
			ErrInvalidPullRequest, c.PullRequest.TitleSource)
	}
}
