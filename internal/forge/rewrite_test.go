// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

// A description rewritten — the issue's line added, as linking a branch adds
// it — starts from the description as the forge holds it, not as it is shown:
// what is shown has its controls and bidirectional marks replaced and every
// carriage return dropped, and sending that back would rewrite the author's
// whole description.

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// asItsAuthorWroteIt is a description as its author wrote it: right-to-left
// text with a bidirectional override, and Windows line endings.
const asItsAuthorWroteIt = "Fixes the parser.\r\n\r\n\u202Eright to left\u202C\r\n"

// withIssueLine adds the issue's line, as linking a branch does.
func withIssueLine(body string) (string, bool) {
	return body + "\nPROJ-7\n", true
}

func TestRewriteDescriptionChangesOnlyWhatTheRewriteChanges(t *testing.T) {
	t.Parallel()

	quoted, err := json.Marshal(asItsAuthorWroteIt)
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]struct {
		repo   forge.Repo
		path   string
		answer string
		method string
		field  string
	}{
		"a GitHub pull request": {
			repo: githubRepo(), path: githubPullsPath + "/43", method: http.MethodPatch, field: "body",
			answer: `{"number":43,"title":"fix: parser","body":` + string(quoted) + `}`,
		},
		"a GitLab merge request": {
			repo: gitlabRepo(), path: gitlabMergesPath + "/43", method: http.MethodPut, field: "description",
			answer: `{"iid":43,"title":"fix: parser","description":` + string(quoted) + `}`,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, seen := recordingForge(t, routing(map[string]string{tt.path: tt.answer}))

			// Act
			changed, err := client.RewriteDescription(t.Context(), tt.repo, forge.PullRequest{Number: 43}, withIssueLine)

			// Assert
			if err != nil || !changed {
				t.Fatalf("RewriteDescription = %v, %v; want it changed", changed, err)
			}

			write := lastRequest(t, seen)
			if write.method != tt.method || write.body[tt.field] != asItsAuthorWroteIt+"\nPROJ-7\n" {
				t.Errorf("wrote %s %q, want the description as asItsAuthorWroteIt with the issue's line added",
					write.method, write.body[tt.field])
			}

			if _, sent := write.body["title"]; sent {
				t.Errorf("wrote the title %q too, want it left as it is", write.body["title"])
			}
		})
	}
}

func TestRewriteDescriptionWritesNothingWhenNothingChanges(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, routing(map[string]string{
		githubPullsPath + "/43": `{"number":43,"body":"Jira: PROJ-7"}`,
	}))

	// Act
	changed, err := client.RewriteDescription(t.Context(), githubRepo(), forge.PullRequest{Number: 43},
		func(body string) (string, bool) { return body, false })

	// Assert
	if err != nil || changed || len(*seen) != 1 || (*seen)[0].method != http.MethodGet {
		t.Errorf("RewriteDescription = %v, %v after %+v; want one read and no write", changed, err, *seen)
	}
}

func TestRewriteDescriptionThatCannotReadWritesNothing(t *testing.T) {
	t.Parallel()

	pullPath := githubPullsPath + "/43"

	cases := map[string]struct {
		repo   forge.Repo
		answer func(recorded) (int, string)
		want   error
	}{
		"on a forge it cannot name": {
			repo: unknownForge(), answer: conversation(nil, nil), want: forge.ErrUnknownForge,
		},
		// Both forges answer 404, not 403, for a repository the token cannot see.
		"in a repository the token cannot see": {
			repo: githubRepo(), answer: answering(http.StatusNotFound, `{"message":"Not Found"}`),
			want: forge.ErrNoRepository,
		},
		"when the read is refused": {
			repo: githubRepo(), answer: conversation(nil, map[string]bool{pullPath: true}), want: forge.ErrRefused,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, seen := recordingForge(t, tt.answer)

			// Act
			changed, err := client.RewriteDescription(t.Context(), tt.repo, forge.PullRequest{Number: 43}, withIssueLine)

			// Assert
			if !errors.Is(err, tt.want) || changed {
				t.Errorf("RewriteDescription = %v, %v; want %v and nothing changed", changed, err, tt.want)
			}

			for _, asked := range *seen {
				if asked.method != http.MethodGet {
					t.Errorf("asked %s %s, want nothing written", asked.method, asked.path)
				}
			}
		})
	}
}
