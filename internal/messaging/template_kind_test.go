// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestTheTemplateShapesAReadyAnnouncementOnlyForSlack(t *testing.T) {
	t.Parallel()

	// The template is Slack mrkdwn, so a service that reads Markdown keeps the
	// built-in text rather than showing Slack's link syntax as written.
	cases := map[string]struct {
		kind config.MessagingKind
		want string
	}{
		"a slack webhook takes the template": {
			kind: config.KindSlack,
			want: "🚀 jacob needs a review of <" + readyPullURL + "|fix: tidy>",
		},
		"teams keeps the built-in text": {
			kind: config.KindTeams,
			want: "jacob opened a pull request: [fix: tidy](" + readyPullURL + ")",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			announcement := messaging.Announcement{
				Kind:             tt.kind,
				Template:         "🚀 {author} needs a review of <{url}|{title}>",
				Author:           testAuthor,
				PullRequestURL:   readyPullURL,
				PullRequestTitle: "fix: tidy",
			}

			// Act
			got := announcement.Text()

			// Assert
			if got != tt.want {
				t.Errorf("Text() = %q, want %q", got, tt.want)
			}
		})
	}
}
