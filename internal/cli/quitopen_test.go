// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// Opening a page in the browser changes nothing workflow or a forge holds, so
// a quit does not wait for it: an opener that runs the browser in the
// foreground goes on until the browser closes, and workflow exits under it.

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// heldOpener opens a page in the browser and returns only once it is
// released, whatever becomes of its context, as an opener running the
// browser in the foreground does: began closes once a page is opened.
type heldOpener struct {
	began   chan struct{}
	release chan struct{}
	opening sync.Once
	letGo   sync.Once
}

// newHeldOpener is an opener nothing has asked yet, released when the test
// ends at the latest. A test that runs workflow over it releases it again
// once running has registered its own cleanup, which waits for workflow to
// return, so a failure while workflow waits for the opener ends the test at
// once.
func newHeldOpener(t *testing.T) *heldOpener {
	t.Helper()

	held := &heldOpener{began: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(held.letReturn)

	return held
}

// open is the opener's body.
func (h *heldOpener) open(context.Context, string) error {
	h.opening.Do(func() { close(h.began) })
	<-h.release

	return nil
}

// letReturn lets every page the opener is opening return.
func (h *heldOpener) letReturn() {
	h.letGo.Do(func() { close(h.release) })
}

// interfaceOpening is an interface over one issue, whose page it opens
// through open, reading keys from keys: listed is called once the issue is
// on screen, as its list is cached.
func interfaceOpening(open func(context.Context, string) error, listed func(), keys io.Reader) cli.RunInterface {
	return func(ctx context.Context, _ tui.Model, _ io.Reader, _ io.Writer) (tui.Next, error) {
		deps := tui.Deps{
			Jira: seams.Jira{
				Search: func(context.Context, string, int) (jira.SearchResult, error) {
					return jira.SearchResult{Issues: []jira.Issue{{Key: "PROJ-1", Summary: "Fix the login"}}, Total: 1}, nil
				},
				BrowseURL: func(key jira.Key) string { return "https://jira.example.com/browse/" + string(key) },
			},
			Store: seams.Store{
				CacheIssues: func(context.Context, string, []jira.Issue) error {
					listed()

					return nil
				},
			},
			OpenURL: open,
		}

		return tui.Run(ctx, tui.New(config.Default(), nil, deps), keys, io.Discard)
	}
}

// openingThenQuitting is an interface whose issue's page is opened through
// opener once the issue is on screen, and which quits once the page has begun
// to open.
func openingThenQuitting(t *testing.T, opener *heldOpener) cli.RunInterface {
	t.Helper()

	keys, typing := io.Pipe()
	listed := make(chan struct{})
	shown := sync.OnceFunc(func() { close(listed) })

	t.Cleanup(func() { _ = keys.Close() })

	go func() {
		for _, step := range []struct {
			after <-chan struct{}
			key   string
		}{{listed, "o"}, {opener.began, "q"}} {
			select {
			case <-step.after:
				_, _ = typing.Write([]byte(step.key))
			case <-t.Context().Done():
				return
			}
		}
	}()

	return interfaceOpening(opener.open, shown, keys)
}

func TestAQuitDoesNotWaitForAPageOpeningInTheBrowser(t *testing.T) {
	t.Parallel()

	// Arrange
	opener, said := newHeldOpener(t), &lockedBuffer{}
	root := quittingRoot(t, openingThenQuitting(t, opener), "1m", said)

	// Act
	returned := running(t.Context(), t, root)

	t.Cleanup(opener.letReturn)

	// Assert
	err := returnedWith(t, returned)
	if err != nil || strings.Contains(said.String(), "finishing") {
		t.Errorf("workflow = %v, said %q; want it to exit at once and cleanly under the page opening",
			err, said.String())
	}
}
