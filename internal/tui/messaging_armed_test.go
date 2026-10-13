// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// openingWorld is a branch with no pull request yet whose CI, once one opens,
// runs, read only when asked, since its interval is past the harness's
// horizon; announcing such a pull request automatically is on, as it is by
// default.
func openingWorld() *world {
	repo := withoutPull()
	repo.cfg.Messaging.AutoAnnounce = true
	repo.ciInterval = 2 * time.Second
	repo.ci = []forge.CI{{State: forge.CIRunning, Total: 1}}

	return repo
}

// taggingOpeningWorld is openingWorld with a Slack user token and a code
// owner linked to someone on Slack, as taggingWorld has.
func taggingOpeningWorld() *world {
	repo := taggingWorld()
	repo.pullFound = false
	repo.cfg.Messaging.AutoAnnounce = true
	repo.ciInterval = 2 * time.Second
	repo.ci = []forge.CI{{State: forge.CIRunning, Total: 1}}

	return repo
}

// opens is the pull request opened from the Review pane, skipping the offers that
// follow, and looks at the messaging pane; the forge finds it from then on.
func opens(t *testing.T, repo *world) tui.Model {
	t.Helper()

	opened := typing(t, repo.live(t, 160, 40), "4", "n", keyEnter, keyEsc, keyEsc, "5")

	repo.mu.Lock()
	defer repo.mu.Unlock()

	repo.pullFound = true

	return opened
}

func TestOpeningAPullRequestArmsItsAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := openingWorld()

	// Act: open the pull request
	armed := opens(t, repo)

	// Assert: its announcement waits for CI
	requireScreen(t, armed.View().Content, "◐ announces when CI passes")

	if calls := repo.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q before CI passed", calls)
	}

	// Act: CI passes, and is read again
	passesNow(repo)
	typing(t, armed, "r")

	// Assert: it is announced, once, as opened
	if calls := repo.asked("post "); len(calls) != 1 || !strings.Contains(calls[0], "opened a pull request") {
		t.Errorf("post calls = %q, want the opened pull request announced once", calls)
	}
}

func TestTheOpenedNoticeSaysTheAnnouncementIsArmed(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := openingWorld()

	// Act
	opened := typing(t, repo.live(t, 200, 40), "4", "n", keyEnter).View().Content

	// Assert
	requireScreen(t, opened, "announces to "+slackChannel+" once CI passes")
}

func TestOpeningArmsNothingWithAutoAnnounceOff(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := openingWorld()
	repo.cfg.Messaging.AutoAnnounce = false

	// Act
	opened := opens(t, repo)

	// Assert
	refuseScreen(t, opened.View().Content, "announces when CI passes")
}

func TestOpeningArmsNothingWithoutMessaging(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := openingWorld()
	repo.cfg.Messaging = config.Messaging{AutoAnnounce: true}

	// Act
	opened := typing(t, repo.live(t, 160, 40), "4", "n", keyEnter).View().Content

	// Assert
	refuseScreen(t, opened, "once CI passes")
}

func TestOpeningADraftArmsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := openingWorld()
	repo.pull.Draft = true

	// Act
	opened := typing(t, repo.live(t, 200, 40), "4", "n", keyEnter).View().Content

	// Assert
	requireScreen(t, opened, "a draft is not announced; announce it once it is ready")
	refuseScreen(t, opened, "once CI passes")
}

func TestStopWaitingDropsAnArmedAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := openingWorld()
	armed := opens(t, repo)

	// Act
	stopped := typing(t, armed, "x")

	passesNow(repo)
	typing(t, stopped, "r")

	// Assert
	if calls := repo.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q after waiting was stopped", calls)
	}
}

func TestQuittingWithAnArmedAnnouncementAsksFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	armed := opens(t, openingWorld())

	// Act
	asked := typing(t, armed, "q").View().Content

	// Assert
	requireScreen(t, asked, "An announcement is waiting for CI and will be lost.")
}

func TestAnArmedAnnouncementTagsAsAnnounceYesDoes(t *testing.T) {
	t.Parallel()

	// Arrange
	// A Slack user token, and a code owner linked to someone on Slack.
	repo := taggingOpeningWorld()
	armed := opens(t, repo)

	// Act
	passesNow(repo)
	typing(t, armed, "r")

	// Assert
	if calls := repo.asked("post "); len(calls) != 1 || !strings.Contains(calls[0], "<@"+carlaID+">") {
		t.Errorf("post calls = %q, want one tagging the linked owner", calls)
	}
}

func TestAnArmedAnnouncementWhoseWorkspaceCannotBeReadPostsUntagged(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := taggingOpeningWorld()
	repo.slack.workspaceErr = errForgeDown
	armed := opens(t, repo)

	// Act
	passesNow(repo)
	typing(t, armed, "r")

	// Assert
	if calls := repo.asked("post "); len(calls) != 1 || strings.Contains(calls[0], "<@") {
		t.Errorf("post calls = %q, want one, tagging no one", calls)
	}
}

func TestAnArmedAnnouncementTagsTheOwnersAgainstTheBaseItOpenedInto(t *testing.T) {
	t.Parallel()

	// Arrange
	// It opens into release, which the forge says it merges into, rather than
	// the branch's own base.
	repo := taggingOpeningWorld()
	repo.pull.Base = "release"
	armed := opens(t, repo)
	before := len(repo.asked("code-owners "))

	// Act
	passesNow(repo)
	typing(t, armed, "r")

	// Assert
	read := repo.asked("code-owners ")[before:]
	if len(read) == 0 || slices.ContainsFunc(read, func(call string) bool { return call != "code-owners release" }) {
		t.Errorf("code owners read as %q as it went, want them read against release", read)
	}
}

func TestAnArmedAnnouncementOutlastsAFindBegunBeforeItsPullRequestOpened(t *testing.T) {
	t.Parallel()

	// Arrange
	// n pushes first, and the branch read after the push looks for the pull
	// request alongside the create: that find reads the forge before the pull
	// request is on it, and answers once the pull request has opened.
	repo := openingWorld()
	repo.branch.Ahead = 1
	armed := opens(t, repo)

	// Act
	passesNow(repo)
	typing(t, armed, "r")

	// Assert
	if calls := repo.asked("post "); len(calls) != 1 {
		t.Errorf("post calls = %q, want the opened pull request announced once", calls)
	}
}

// postedOnceCIPasses is the one post an armed announcement made once CI
// passed and was read again.
func postedOnceCIPasses(t *testing.T, repo *world, armed tui.Model) string {
	t.Helper()

	passesNow(repo)
	typing(t, armed, "r")

	return postedText(t, repo)
}

func TestAnArmedAnnouncementPostsUntaggedWithoutTheScopeTaggingNeeds(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := taggingOpeningWorld()
	repo.slack.membersErr = &messaging.MissingScopeError{Needed: "users:read"}
	armed := opens(t, repo)

	// Act
	posted := postedOnceCIPasses(t, repo, armed)

	// Assert
	if strings.Contains(posted, "<@") || strings.Contains(posted, "<!subteam") {
		t.Errorf("posted %q, want it untagged, as a preview without users:read posts", posted)
	}
}

func TestAnArmedAnnouncementTagsNoGroupWithoutTheScopeGroupsNeed(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := taggingOpeningWorld()
	repo.slack.groupsErr = &messaging.MissingScopeError{Needed: groupsScope}
	armed := opens(t, repo)

	// Act
	posted := postedOnceCIPasses(t, repo, armed)

	// Assert
	if !strings.Contains(posted, "<@"+carlaID+">") || strings.Contains(posted, "<!subteam") {
		t.Errorf("posted %q, want carla tagged and no group, as a preview without usergroups:read posts", posted)
	}
}

func TestAnArmedAnnouncementWithNoUserGroupsToReadStillTags(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := taggingOpeningWorld()
	repo.slack.noGroups = true
	armed := opens(t, repo)

	// Act
	posted := postedOnceCIPasses(t, repo, armed)

	// Assert
	if !strings.Contains(posted, "<@"+carlaID+">") {
		t.Errorf("posted %q, want carla tagged", posted)
	}
}
