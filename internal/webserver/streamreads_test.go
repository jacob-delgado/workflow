// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// streamedFrames is how many snapshots the tests here let a stream push: enough
// for a read made once to show against one made on every frame.
const streamedFrames = 3

// cancelingFlusher records the stream and cancels its request from inside the
// flush that completes the last frame wanted, so a test sees a known number of
// pushes without timing them. It embeds the recorder for everything else a
// ResponseWriter does.
type cancelingFlusher struct {
	*httptest.ResponseRecorder

	cancel  context.CancelFunc
	frames  int
	flushed int
}

func (c *cancelingFlusher) Flush() {
	c.ResponseRecorder.Flush()

	c.flushed++
	if c.flushed == c.frames {
		c.cancel()
	}
}

// streamFrames runs the events handler on a millisecond interval until it has
// pushed streamedFrames snapshots, and returns the body. The handler runs on the
// test's goroutine, so the seams it calls need no lock. A frame may slip out
// after the cancel, so a test compares its reads against the frames the body
// carries rather than against streamedFrames.
func streamFrames(t *testing.T, deps webserver.Deps) string {
	t.Helper()

	info := webserver.Info{Version: testVersion, StreamInterval: time.Millisecond}
	handler := serveWith(t, deps, config.Default(), info)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/events", nil)
	request.Host = loopbackHost
	writer := &cancelingFlusher{ResponseRecorder: httptest.NewRecorder(), cancel: cancel, frames: streamedFrames}
	handler.ServeHTTP(writer, request)

	return writer.Body.String()
}

func TestStreamAsksTheForgeWhoTheAuthorIsOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	asked := 0
	deps := filledDeps()
	deps.Author = func() (string, error) {
		asked++

		return testAuthor, nil
	}

	// Act
	pushed := snapshots(t, streamFrames(t, deps))

	// Assert
	if len(pushed) < streamedFrames {
		t.Fatalf("the stream pushed %d snapshots, want at least %d", len(pushed), streamedFrames)
	}

	if asked != 1 {
		t.Errorf("asked the forge for the author %d times across %d frames, want once", asked, len(pushed))
	}

	if last := pushed[len(pushed)-1]; last.Messaging.Author != testAuthor {
		t.Errorf("last frame's author = %q, want %q kept from the first read", last.Messaging.Author, testAuthor)
	}
}

func TestStreamAsksForTheAuthorAgainAfterAFailedRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// The first read fails and the second answers: the failure is asked again on
	// the next frame, and the answer is kept from then on.
	asked := 0
	deps := filledDeps()
	deps.Author = func() (string, error) {
		asked++
		if asked == 1 {
			return "", errSeam
		}

		return testAuthor, nil
	}

	// Act
	pushed := snapshots(t, streamFrames(t, deps))

	// Assert
	if len(pushed) < streamedFrames {
		t.Fatalf("the stream pushed %d snapshots, want at least %d", len(pushed), streamedFrames)
	}

	if first := pushed[0]; first.Messaging.Author != "" {
		t.Errorf("first frame's author = %q, want empty while the forge cannot say", first.Messaging.Author)
	}

	if last := pushed[len(pushed)-1]; last.Messaging.Author != testAuthor {
		t.Errorf("last frame's author = %q, want %q once the forge answers", last.Messaging.Author, testAuthor)
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
	deps := filledDeps()
	readBranch := deps.Branch
	deps.Branch = func() (gitrepo.Branch, error) {
		reads++

		return readBranch()
	}
	deps.Branches = func() ([]string, error) { return []string{testBranchName}, nil }

	// Act
	pushed := snapshots(t, streamFrames(t, deps))

	// Assert
	if len(pushed) < streamedFrames {
		t.Fatalf("the stream pushed %d snapshots, want at least %d", len(pushed), streamedFrames)
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
	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) {
		name := checkedOut[reads%len(checkedOut)]
		reads++

		return gitrepo.Branch{Name: name, Head: testCommitHash}, nil
	}
	deps.Branches = func() ([]string, error) { return checkedOut, nil }

	askedAbout := ""
	findPull := deps.FindPull
	deps.FindPull = func(branch string) (forge.PullRequest, bool, error) {
		askedAbout = branch

		return findPull(branch)
	}

	cfg := config.Default()
	cfg.Jira.Project = testProject

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, deps, cfg), "/api/events").Body.String())

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
