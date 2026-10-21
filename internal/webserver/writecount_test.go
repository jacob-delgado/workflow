// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// writeNumberSent is the answer's WriteNumberHeader as its client receives
// it: the headers as they stood when the status went, since net/http sends
// none set after that.
func writeNumberSent(answer *httptest.ResponseRecorder) string {
	return answer.Result().Header.Get(webserver.WriteNumberHeader)
}

// writeNumber is the number a write's answer carries, or a failure when it
// carries none that reads as one.
func writeNumber(t *testing.T, answer *httptest.ResponseRecorder) int {
	t.Helper()

	number, err := strconv.Atoi(writeNumberSent(answer))
	if err != nil {
		t.Fatalf("the answer's %s header %q is no number: %v", webserver.WriteNumberHeader,
			writeNumberSent(answer), err)
	}

	return number
}

func TestAFrameReadBeforeAWriteCountsFewerWritesThanTheWritesNumber(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		tree := &liveTree{}
		handler, stream := streamOver(t, tree.wire(webtest.FilledDeps()), config.Default())

		// Act
		staged := webtest.Send(t, handler, http.MethodPost, stagePath, stageNotes)

		synctest.Wait()

		// Assert
		number := writeNumber(t, staged)

		pushed := webtest.Snapshots(t, stream.Close())
		if first := pushed[0]; first.Writes >= number || notesStaged(first) {
			t.Errorf("the frame read before the stage counts %d writes, staged %v; want fewer than its number %d",
				first.Writes, notesStaged(first), number)
		}

		if last := pushed[len(pushed)-1]; last.Writes < number || !notesStaged(last) {
			t.Errorf("the frame the stage woke counts %d writes, staged %v; want at least its number %d",
				last.Writes, notesStaged(last), number)
		}
	})
}

func TestEveryWritesAnswerIsNumberedARefusalAlike(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := &liveTree{}
	handler := webtest.Serve(t, tree.wire(webtest.FilledDeps()), config.Default())
	staged := webtest.Send(t, handler, http.MethodPost, stagePath, stageNotes)

	// Act
	refused := webtest.Send(t, handler, http.MethodPost, stagePath, `{"path":"nope.txt"}`)

	// Assert
	if refused.Code != http.StatusNotFound {
		t.Fatalf("the stage of a path the tree does not list answered %d, want the server's 404: %s",
			refused.Code, refused.Body.String())
	}

	if first, second := writeNumber(t, staged), writeNumber(t, refused); first != 1 || second != 2 {
		t.Errorf("the writes were numbered %d and %d, want 1 and 2", first, second)
	}
}

func TestARequestTheContractRefusesIsNotNumbered(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := webtest.Serve(t, webtest.FilledDeps(), config.Default())

	// Act
	refused := webtest.Send(t, handler, http.MethodPost, stagePath, `{"path":7}`)

	// Assert
	if refused.Code != http.StatusBadRequest || writeNumberSent(refused) != "" {
		t.Errorf("status %d with %s %q, want 400 with none: the server never took it",
			refused.Code, webserver.WriteNumberHeader, writeNumberSent(refused))
	}
}

// runEnd is the last event a run's stream wrote.
func runEnd(t *testing.T, run *httptest.ResponseRecorder) api.RunEvent {
	t.Helper()

	var last string

	lines := bufio.NewScanner(strings.NewReader(run.Body.String()))
	for lines.Scan() {
		if strings.TrimSpace(lines.Text()) != "" {
			last = lines.Text()
		}
	}

	var event api.RunEvent

	err := json.Unmarshal([]byte(last), &event)
	if err != nil {
		t.Fatalf("the run's last line %q is no event: %v", last, err)
	}

	return event
}

func TestARunsEndIsNumberedAfterItsStart(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		tree := newRebasingTree()
		handler, stream := streamOver(t, tree.wire(webtest.FilledDeps()), config.Default())
		run := httptest.NewRecorder()
		done := make(chan struct{})

		go func() {
			defer close(done)

			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/runs",
				strings.NewReader(`{"kind":"rebase"}`))
			request.Host = webtest.LoopbackHost
			request.Header.Set("Content-Type", "application/json")
			handler.ServeHTTP(run, request)
		}()

		synctest.Wait()

		// Act
		tree.end()
		<-done
		synctest.Wait()

		// Assert
		started, ended := writeNumber(t, run), runEnd(t, run).Write
		if ended == nil || *ended <= started {
			t.Fatalf("the run's start is numbered %d and its end %v, want its end numbered after its start",
				started, ended)
		}

		if last := lastSnapshot(t, stream.Close()); last.Writes < *ended || last.Branch.Behind != 0 {
			t.Errorf("the last frame counts %d writes, %d behind; want at least the run's end, %d, rebased",
				last.Writes, last.Branch.Behind, *ended)
		}
	})
}

func TestAFrameOfTheDirectorySwitchedToCountsTheSwitch(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached []string

	handler := webtest.Serve(t, reachable(repositoryDeps(favoritesKept()), &reached), config.Default())
	webtest.Send(t, handler, http.MethodPost, stagePath, stageNotes)

	// Act
	switched := webtest.Send(t, handler, http.MethodPut, reposPath+"/here", `{"dir":"`+webRoot+`"}`)

	// Assert
	number := writeNumber(t, switched)

	frame := webtest.FirstSnapshot(t, webtest.StreamOnce(t, handler, "/api/events").Body.String())
	if number != 2 || frame.Writes < number {
		t.Errorf("the switch is numbered %d and the next frame counts %d writes; want 2, and at least 2",
			number, frame.Writes)
	}
}

func TestARetiredStreamSendsNoFrameCountingTheSwitch(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// The old stream's first read is held on its branch, so the switch
		// lands while it reads the directory switched from.
		var reached []string

		held := webtest.NewHeld()
		deps := reachable(repositoryDeps(favoritesKept()), &reached)
		branch, holdFirst := deps.Git.Branch, heldOnCall(1, held)
		deps.Git.Branch = func(ctx context.Context) (gitrepo.Branch, error) {
			err := holdFirst(ctx)
			if err != nil {
				return gitrepo.Branch{}, err
			}

			return branch(ctx)
		}
		handler, stream := streamOver(t, deps, config.Default())

		// Act
		switched := webtest.Send(t, handler, http.MethodPut, reposPath+"/here", `{"dir":"`+webRoot+`"}`)

		close(held.Release)
		synctest.Wait()

		// Assert
		number := writeNumber(t, switched)

		for _, frame := range sentSnapshots(t, stream.Close()) {
			if frame.Writes >= number && frame.Here != webRoot {
				t.Errorf("a frame counting %d writes, at least the switch's %d, shows %s; want none but of %s",
					frame.Writes, number, frame.Here, webRoot)
			}
		}
	})
}

// sentSnapshots decodes every snapshot event's data from an event-stream
// body, in the order sent: none, for a stream that sent none.
func sentSnapshots(t *testing.T, body string) []api.Snapshot {
	t.Helper()

	var sent []api.Snapshot

	for _, message := range webtest.Messages(body) {
		if message.Event != webtest.SnapshotMessage {
			continue
		}

		var frame api.Snapshot

		err := json.Unmarshal([]byte(message.Data), &frame)
		if err != nil {
			t.Fatalf("decoding the snapshot from %q: %v", message.Data, err)
		}

		sent = append(sent, frame)
	}

	return sent
}
