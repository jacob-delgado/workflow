// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// ctrl+c quits the interface at once, and an announcement waiting for CI is
// stopped with it, unposted: once the terminal is back, workflow says so on
// stderr, so the loss is not silent.

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// heldForge is a forge whose one pull request's CI runs, telling found and
// read each time a find or a read of CI begins.
type heldForge struct {
	found, read chan struct{}
}

// newHeldForge is a forge no find or read has begun on yet.
func newHeldForge() *heldForge {
	return &heldForge{found: make(chan struct{}, 8), read: make(chan struct{}, 8)}
}

// deps is a branch with an open pull request whose CI runs, and a webhook
// that takes posts.
func (f *heldForge) deps() tui.Deps {
	branch := gitrepo.Branch{Name: "fix/PROJ-1-fix-the-login", Head: "abc123", Base: "origin/main"}
	pull := forge.PullRequest{Number: 42, URL: "https://github.com/example/repo/pull/42", HeadCommit: "abc123"}

	return tui.Deps{
		Git: seams.Git{Branch: func(context.Context) (gitrepo.Branch, error) { return branch, nil }},
		Forge: seams.Forge{
			FindPullRequest: func(context.Context, string) (forge.PullRequest, bool, error) {
				f.found <- struct{}{}

				return pull, true, nil
			},
			CheckStatus: func(context.Context, forge.PullRequest, string) (forge.CI, error) {
				f.read <- struct{}{}

				return forge.CI{State: forge.CIRunning, Total: 1}, nil
			},
		},
		Messaging: seams.Messaging{Post: func(context.Context, string, string) error { return nil }},
	}
}

// awaitBegun waits for begun to be told once, reporting false once patience
// runs out.
func awaitBegun(begun <-chan struct{}) bool {
	select {
	case <-begun:
		return true
	case <-time.After(signalPatience):
		return false
	}
}

// interruptedWhileWaiting is an interface over held that arms an announcement
// to go once CI passes and is interrupted while it waits.
func interruptedWhileWaiting(t *testing.T, held *heldForge) cli.RunInterface {
	t.Helper()

	keys, typing := io.Pipe()

	t.Cleanup(func() { _ = keys.Close() })

	var once sync.Once

	return func(ctx context.Context, _ tui.Model, _ io.Reader, _ io.Writer) (tui.Next, error) {
		cfg := config.Default()
		cfg.Messaging.WebhookURL = "https://hooks.example.com/services/T0/B0/x"

		once.Do(func() {
			go func() {
				// The pull request is found and its CI read before p offers a
				// post; the announcement's own find follows w.
				if !awaitBegun(held.found) || !awaitBegun(held.read) {
					t.Error("the interface never found the pull request and read its CI")

					return
				}

				_, _ = typing.Write([]byte("5pw"))

				if !awaitBegun(held.found) {
					t.Error("the announcement armed never found its pull request")
				}

				_, _ = typing.Write([]byte{0x03})
			}()
		})

		return tui.Run(ctx, tui.New(cfg, nil, held.deps()), keys, io.Discard)
	}
}

func TestCtrlCWithAnAnnouncementWaitingSaysItWasStopped(t *testing.T) {
	t.Parallel()

	// Arrange
	said := &lockedBuffer{}
	root := quittingRoot(t, interruptedWhileWaiting(t, newHeldForge()), "1m", said)

	// Act
	err := returnedWith(t, running(t.Context(), t, root))
	// Assert
	if err != nil {
		t.Errorf("workflow = %v, want a clean exit", err)
	}

	awaitSaid(t, said, "Stopped waiting for CI on #42; nothing was announced.")
}
