// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// A merge request into the base from a fork's branch of the same name is
// someone else's: GitLab marks it by a source project other than its target.
func TestGitLabFindPassesOverAForksMergeRequestOfTheSameName(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := recordingForge(t, routing(map[string]string{
		gitlabMergesPath: `[{"iid":3,"state":"opened","source_project_id":42,"target_project_id":7}]`,
	}))

	// Act
	pull, found, err := client.FindPullRequest(t.Context(), gitlabRepo(), forge.Head{Branch: featureBranch})

	// Assert
	if err != nil || found {
		t.Errorf("FindPullRequest = %+v, %v, %v; want none found", pull, found, err)
	}
}

func TestGitLabFindTakesTheBasesOwnMergeRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := recordingForge(t, routing(map[string]string{
		gitlabMergesPath: `[{"iid":3,"state":"opened","source_project_id":42,"target_project_id":7},` +
			`{"iid":5,"state":"opened","source_project_id":7,"target_project_id":7}]`,
	}))

	// Act
	pull, found, err := client.FindPullRequest(t.Context(), gitlabRepo(), forge.Head{Branch: featureBranch})

	// Assert
	if err != nil || !found || pull.Number != 5 {
		t.Errorf("FindPullRequest = %+v, %v, %v; want the base's own !5", pull, found, err)
	}
}

// gitlabFork is a fork of gitlabRepo, and its project path in GitLab's API.
const (
	gitlabFork        = "contributor/repo"
	gitlabForkProject = "/projects/contributor%2Frepo"
	gitlabBaseProject = "/projects/group%2Fsub%2Frepo"
)

// forkHead is the branch pushed to gitlabFork.
func forkHead() forge.Head {
	return forge.Head{Branch: featureBranch, Fork: gitlabFork}
}

func TestGitLabFindsAForksMergeRequestByItsProject(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := recordingForge(t, routing(map[string]string{
		gitlabForkProject: `{"id":42}`,
		gitlabMergesPath: `[{"iid":3,"state":"opened","source_project_id":7,"target_project_id":7},` +
			`{"iid":9,"state":"opened","source_project_id":42,"target_project_id":7}]`,
	}))

	// Act
	pull, found, err := client.FindPullRequest(t.Context(), gitlabRepo(), forkHead())

	// Assert
	if err != nil || !found || pull.Number != 9 {
		t.Errorf("FindPullRequest = %+v, %v, %v; want the fork's !9", pull, found, err)
	}
}

// GitLab opens a fork's merge request on the fork, naming the project it
// merges into; the iid it answers is the target's, so everything after
// addresses the base as before.
func TestGitLabOpensAForksMergeRequestIntoTheBase(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, routing(map[string]string{
		gitlabBaseProject:                     `{"id":7}`,
		gitlabForkProject + "/merge_requests": `{"iid":9,"web_url":"https://gitlab.com/group/sub/repo/-/merge_requests/9"}`,
	}))
	request := forge.NewPullRequest{Title: prTitle, Head: forkHead(), Base: baseBranch}

	// Act
	pull, err := client.CreatePullRequest(t.Context(), gitlabRepo(), request)

	// Assert
	posted := (*seen)[len(*seen)-1]
	if err != nil || pull.Number != 9 || posted.method != http.MethodPost ||
		posted.path != gitlabForkProject+"/merge_requests" || posted.body["target_project_id"] != float64(7) ||
		posted.body["source_branch"] != featureBranch || posted.body["target_branch"] != baseBranch {
		t.Errorf("CreatePullRequest = %+v, %v after %s %s %v; want it posted to the fork into project 7",
			pull, err, posted.method, posted.path, posted.body)
	}
}

func TestGitLabOpensTheBasesOwnMergeRequestWithNoTarget(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, answering(http.StatusCreated, `{"iid":9}`))
	request := forge.NewPullRequest{Title: prTitle, Head: forge.Head{Branch: featureBranch}, Base: baseBranch}

	// Act
	_, err := client.CreatePullRequest(t.Context(), gitlabRepo(), request)

	// Assert
	_, named := (*seen)[0].body["target_project_id"]
	if err != nil || len(*seen) != 1 || (*seen)[0].path != gitlabMergesPath || named {
		t.Errorf("CreatePullRequest = %v after %v; want one post to the base naming no target", err, *seen)
	}
}

func TestGitLabRefusesAForkItCannotSee(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := recordingForge(t, func(asked recorded) (int, string) {
		if asked.path == gitlabForkProject {
			return http.StatusNotFound, `{"message":"404 Project Not Found"}`
		}

		return http.StatusOK, "[]"
	})

	// Act
	_, _, err := client.FindPullRequest(t.Context(), gitlabRepo(), forkHead())

	// Assert
	if !errors.Is(err, forge.ErrNoRepository) || !strings.Contains(err.Error(), gitlabFork) {
		t.Errorf("FindPullRequest = %v, want %v naming %s", err, forge.ErrNoRepository, gitlabFork)
	}
}

// A contributor needs to see the project a fork's merge request merges into;
// one they cannot is said, naming it, and nothing is posted.
func TestGitLabOpensNothingIntoABaseItCannotSee(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, answering(http.StatusNotFound, `{"message":"404 Project Not Found"}`))
	request := forge.NewPullRequest{Title: prTitle, Head: forkHead(), Base: baseBranch}

	// Act
	_, err := client.CreatePullRequest(t.Context(), gitlabRepo(), request)

	// Assert
	if !errors.Is(err, forge.ErrNoRepository) || !strings.Contains(err.Error(), gitlabRepo().Path) || len(*seen) != 1 {
		t.Errorf("CreatePullRequest = %v after %v; want %v naming the base, and no post",
			err, *seen, forge.ErrNoRepository)
	}
}
