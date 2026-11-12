// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestReadPullRequestReadsOnePullByItsNumber(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo     forge.Repo
		routes   map[string]string
		readPath string
		want     forge.PullRequest
	}{
		github: {
			repo: githubRepo(),
			routes: map[string]string{
				githubPullsPath + "/42": `{"number":42,"html_url":"https://github.com/example/repo/pull/42",` +
					`"title":"fix: token","state":"open","mergeable":true,"base":{"ref":"main"},"head":{"sha":"abc"}}`,
				githubPullsPath + "/42/reviews": `[{"state":"APPROVED","user":{"login":"ana"}}]`,
			},
			readPath: githubPullsPath + "/42",
			want: forge.PullRequest{
				Number: 42, URL: "https://github.com/example/repo/pull/42", Title: prTitle, Base: baseBranch,
				HeadCommit: "abc", Approvals: 1, Mergeable: forge.MergeClean,
			},
		},
		gitlab: {
			repo: gitlabRepo(),
			routes: map[string]string{
				gitlabMergesPath + "/7": `{"iid":7,"web_url":"https://gitlab.com/group/sub/repo/-/merge_requests/7",` +
					`"title":"fix: token","state":"opened","merge_status":"can_be_merged","target_branch":"main",` +
					`"sha":"abc"}`,
				gitlabMergesPath + "/7/approvals": `{"approved_by":[{}]}`,
			},
			readPath: gitlabMergesPath + "/7",
			want: forge.PullRequest{
				Number: 7, URL: "https://gitlab.com/group/sub/repo/-/merge_requests/7", Title: prTitle,
				Base: baseBranch, HeadCommit: "abc", Approvals: 1, Mergeable: forge.MergeClean,
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, seen := recordingForge(t, routing(tt.routes))

			// Act
			pull, err := client.ReadPullRequest(t.Context(), tt.repo, tt.want.Number)

			// Assert
			if err != nil || pull != tt.want {
				t.Errorf("ReadPullRequest = %+v, %v, want %+v", pull, err, tt.want)
			}

			if read := requestTo(*seen, tt.readPath); read.method != http.MethodGet {
				t.Errorf("the pull request was not read at %s; requests were %+v", tt.readPath, *seen)
			}

			// Its mergeability comes with the read, so it is not read twice.
			if reads := requestsTo(*seen, tt.readPath); reads != 1 {
				t.Errorf("the pull request was read at %s %d times, want once; requests were %+v",
					tt.readPath, reads, *seen)
			}
		})
	}
}

func TestReadPullRequestReadsAPullThatIsNoLongerOpen(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo  forge.Repo
		path  string
		body  string
		state forge.PullState
	}{
		"a merged GitHub pull": {
			repo: githubRepo(), path: githubPullsPath + "/42",
			body:  `{"number":42,"state":"closed","merged_at":"2026-01-01T00:00:00Z"}`,
			state: forge.StateMerged,
		},
		"a closed GitHub pull": {
			repo: githubRepo(), path: githubPullsPath + "/42", body: `{"number":42,"state":"closed","merged_at":null}`,
			state: forge.StateClosed,
		},
		"a merged GitLab merge request": {
			repo: gitlabRepo(), path: gitlabMergesPath + "/42", body: `{"iid":42,"state":"merged"}`,
			state: forge.StateMerged,
		},
		"a closed GitLab merge request": {
			repo: gitlabRepo(), path: gitlabMergesPath + "/42", body: `{"iid":42,"state":"closed"}`,
			state: forge.StateClosed,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, seen := recordingForge(t, routing(map[string]string{tt.path: tt.body}))

			// Act
			pull, err := client.ReadPullRequest(t.Context(), tt.repo, 42)

			// Assert
			if err != nil || pull.Number != 42 || pull.State != tt.state {
				t.Errorf("ReadPullRequest = %+v, %v, want #42 %v", pull, err, tt.state)
			}

			// Its reviews matter only while it is open, so they are not read.
			if asked := len(*seen); asked != 1 {
				t.Errorf("asked the forge %d times (%+v), want only the pull request read", asked, *seen)
			}
		})
	}
}

func TestReadPullRequestReportsAReadTheForgeRefuses(t *testing.T) {
	t.Parallel()

	for name, repo := range map[string]forge.Repo{github: githubRepo(), gitlab: gitlabRepo()} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := recordingForge(t, answering(http.StatusForbidden, `{"message":"Resource not accessible"}`))

			// Act
			_, err := client.ReadPullRequest(t.Context(), repo, 42)

			// Assert
			if !errors.Is(err, forge.ErrRefused) {
				t.Errorf("ReadPullRequest = %v, want the forge's refusal", err)
			}
		})
	}
}

func TestReadPullRequestNeedsAForgeThatIsKnown(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, answering(http.StatusOK, "{}"))

	// Act
	_, err := client.ReadPullRequest(t.Context(), unknownForge(), 42)

	// Assert
	if !errors.Is(err, forge.ErrUnknownForge) || len(*seen) != 0 {
		t.Errorf("ReadPullRequest returned %v and asked %v, want ErrUnknownForge before asking", err, *seen)
	}
}
