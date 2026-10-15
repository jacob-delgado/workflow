// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// streamFrames runs the events handler on a millisecond interval until it has
// pushed webtest.StreamedFrames snapshots, and returns the body. The handler
// runs on the test's goroutine, so the seams it calls need no lock. A frame may
// slip out after the cancel, so a test compares its reads against the frames
// the body carries rather than against webtest.StreamedFrames.
func streamFrames(t *testing.T, deps webserver.Deps) string {
	t.Helper()

	return webtest.StreamPaced(t, deps, config.Default(), 0)
}

func TestStreamAsksTheForgeWhoTheAuthorIsOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	asked := 0
	deps := webtest.FilledDeps()
	deps.Forge.Author = func(context.Context) (string, error) {
		asked++

		return webtest.TestAuthor, nil
	}

	// Act
	pushed := webtest.Snapshots(t, streamFrames(t, deps))

	// Assert
	if len(pushed) < webtest.StreamedFrames {
		t.Fatalf("the stream pushed %d snapshots, want at least %d", len(pushed), webtest.StreamedFrames)
	}

	if asked != 1 {
		t.Errorf("asked the forge for the author %d times across %d frames, want once", asked, len(pushed))
	}

	if last := pushed[len(pushed)-1]; last.Messaging.Author != webtest.TestAuthor {
		t.Errorf("last frame's author = %q, want %q kept from the first read", last.Messaging.Author, webtest.TestAuthor)
	}
}

func TestStreamHoldsAFailedAuthorReadForTheForgeInterval(t *testing.T) {
	t.Parallel()

	// Arrange
	asked := 0
	deps := webtest.FilledDeps()
	deps.Forge.Author = func(context.Context) (string, error) {
		asked++

		return "", webtest.ErrSeam
	}

	// Act
	pushed := webtest.Snapshots(t, streamFrames(t, deps))

	// Assert
	if asked != 1 {
		t.Errorf("asked a forge that cannot say who the author is %d times across %d frames within the interval, want once",
			asked, len(pushed))
	}
}

func TestStreamAsksForTheAuthorAgainAnIntervalAfterAFailedRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// The first read fails and the second answers: the failure is asked again
	// once the forge interval has passed, and the answer is kept from then on.
	asked := 0
	deps := webtest.FilledDeps()
	deps.Forge.Author = func(context.Context) (string, error) {
		asked++
		if asked == 1 {
			return "", webtest.ErrSeam
		}

		return webtest.TestAuthor, nil
	}

	// Act
	pushed := webtest.Snapshots(t, webtest.StreamPaced(t, deps, config.Default(), 20*time.Second))

	// Assert
	if len(pushed) < webtest.StreamedFrames {
		t.Fatalf("the stream pushed %d snapshots, want at least %d", len(pushed), webtest.StreamedFrames)
	}

	if first := pushed[0]; first.Messaging.Author != "" {
		t.Errorf("first frame's author = %q, want empty while the forge cannot say", first.Messaging.Author)
	}

	if last := pushed[len(pushed)-1]; last.Messaging.Author != webtest.TestAuthor {
		t.Errorf("last frame's author = %q, want %q once the forge answers", last.Messaging.Author, webtest.TestAuthor)
	}

	if asked != 2 {
		t.Errorf("asked the forge for the author %d times across %d frames, want twice: the failure, then the answer",
			asked, len(pushed))
	}
}

func TestStreamReadsTheBranchOncePerFrame(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branches panel is listed too, so its marker for the checked-out
	// branch is one of the frame's reads.
	reads := 0
	deps := webtest.FilledDeps()
	readBranch := deps.Git.Branch
	deps.Git.Branch = func(ctx context.Context) (gitrepo.Branch, error) {
		reads++

		return readBranch(ctx)
	}
	deps.Git.Branches = func(context.Context) ([]string, error) { return []string{webtest.TestBranchName}, nil }

	// Act
	pushed := webtest.Snapshots(t, streamFrames(t, deps))

	// Assert
	if len(pushed) < webtest.StreamedFrames {
		t.Fatalf("the stream pushed %d snapshots, want at least %d", len(pushed), webtest.StreamedFrames)
	}

	if reads != len(pushed) {
		t.Errorf("read the branch %d times across %d frames, want once a frame", reads, len(pushed))
	}
}

func TestStreamFrameDescribesOneBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	// Every read finds another branch checked out, as a checkout landing
	// between two reads would: the branch panel, the review and the in-flight
	// marker must all describe the one the frame read.
	checkedOut := []string{"fix/PROJ-1", "fix/PROJ-2", "fix/PROJ-3"}
	reads := 0
	deps := webtest.FilledDeps()
	deps.Git.Branch = func(context.Context) (gitrepo.Branch, error) {
		name := checkedOut[reads%len(checkedOut)]
		reads++

		return gitrepo.Branch{Name: name, Head: webtest.TestCommitHash}, nil
	}
	deps.Git.Branches = func(context.Context) ([]string, error) { return checkedOut, nil }

	askedAbout := ""
	findPull := deps.Forge.FindPullRequest
	deps.Forge.FindPullRequest = func(ctx context.Context, branch string) (forge.PullRequest, bool, error) {
		askedAbout = branch

		return findPull(ctx, branch)
	}

	cfg := config.Default()
	cfg.Jira.Project = webtest.TestProject

	// Act
	snap := webtest.FirstSnapshot(t, webtest.StreamOnce(t, webtest.Serve(t, deps, cfg), "/api/events").Body.String())

	// Assert
	if len(snap.Branches) != len(checkedOut) {
		t.Fatalf("task branches = %+v, want the %d listed", snap.Branches, len(checkedOut))
	}

	if askedAbout != snap.Branch.Name {
		t.Errorf("the review asked about %q, want %q, the branch the frame shows", askedAbout, snap.Branch.Name)
	}

	for _, branch := range snap.Branches {
		if branch.Current != (branch.Name == snap.Branch.Name) {
			t.Errorf("task branch %q current = %t, want only %q, the branch the frame shows, marked",
				branch.Name, branch.Current, snap.Branch.Name)
		}
	}
}
