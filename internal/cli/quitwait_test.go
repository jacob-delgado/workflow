// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// Quitting the last interface waits for the writes it left in flight, so the
// process does not exit under them: it says how many it is finishing, gives up
// after timing.request_timeout naming what it gave up on, and exits at once on
// a signal, which ends every write's context, without waiting for a write that
// goes on past its own.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// cachingLabel is what the issue list's cache write is called where workflow
// says what it gave up on.
const cachingLabel = "caching the issue list"

// stillWaiting is how long a test watches workflow go on waiting for a write.
const stillWaiting = 300 * time.Millisecond

// heldCache is the issue list's cache write, held until it is released or
// its context ends: entered closes once it begins, and landed says how it
// ended, nil when it was released.
type heldCache struct {
	entered  chan struct{}
	release  chan struct{}
	landed   chan error
	entering sync.Once
}

// newHeldCache is a cache write nothing has begun yet, released when the test
// ends at the latest.
func newHeldCache(t *testing.T) *heldCache {
	t.Helper()

	held := &heldCache{entered: make(chan struct{}), release: make(chan struct{}), landed: make(chan error, 1)}
	t.Cleanup(sync.OnceFunc(held.letLand))

	return held
}

// letLand releases the write.
func (h *heldCache) letLand() {
	select {
	case <-h.release:
	default:
		close(h.release)
	}
}

// write is the cache write's body.
func (h *heldCache) write(ctx context.Context, _ string, _ []jira.Issue) error {
	h.entering.Do(func() { close(h.entered) })

	select {
	case <-h.release:
		h.landed <- nil

		return nil
	case <-ctx.Done():
		h.landed <- ctx.Err()

		return fmt.Errorf("caching the issue list: %w", context.Cause(ctx))
	}
}

// writeIgnoringItsContext is the cache write's body held until it is released
// whatever becomes of its context, as a write detached from it is.
func (h *heldCache) writeIgnoringItsContext(ctx context.Context, repo string, issues []jira.Issue) error {
	return h.write(context.WithoutCancel(ctx), repo, issues)
}

// awaitBegun waits for the write to begin, failing the test once patience runs
// out.
func (h *heldCache) awaitBegun(t *testing.T) {
	t.Helper()

	select {
	case <-h.entered:
	case <-time.After(signalPatience):
		t.Fatal("the interface never began caching its issue list")
	}
}

// landedWith is how the write ended, or a failure once patience runs out.
func (h *heldCache) landedWith(t *testing.T) error {
	t.Helper()

	select {
	case err := <-h.landed:
		return err
	case <-time.After(signalPatience):
		t.Fatal("the cache write never ended")

		return nil
	}
}

// lockedBuffer is what workflow says on standard error, read while it runs.
type lockedBuffer struct {
	lock sync.Mutex
	said bytes.Buffer
}

// Write keeps what was said.
func (b *lockedBuffer) Write(said []byte) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	written, err := b.said.Write(said)
	if err != nil {
		return written, fmt.Errorf("keeping what was said: %w", err)
	}

	return written, nil
}

// String is everything said so far.
func (b *lockedBuffer) String() string {
	b.lock.Lock()
	defer b.lock.Unlock()

	return b.said.String()
}

// interfaceCaching is an interface whose issue list's first page is cached
// through cacheIssues, if it is not nil, read keys from keys, run under the
// context the command hands it.
func interfaceCaching(cacheIssues func(context.Context, string, []jira.Issue) error, keys io.Reader) cli.RunInterface {
	return func(ctx context.Context, _ tui.Model, _ io.Reader, _ io.Writer) (tui.Next, error) {
		deps := tui.Deps{Jira: seams.Jira{
			Search: func(context.Context, string, int) (jira.SearchResult, error) {
				return jira.SearchResult{Issues: []jira.Issue{{Key: "PROJ-1", Summary: "Fix the login"}}, Total: 1}, nil
			},
		}}
		if cacheIssues != nil {
			deps.Store.CacheIssues = cacheIssues
		}

		return tui.Run(ctx, tui.New(config.Default(), nil, deps), keys, io.Discard)
	}
}

// quittingRoot is bare workflow over run, in a directory whose file bounds
// each request by timeout, saying what it says on stderr into said.
func quittingRoot(t *testing.T, run cli.RunInterface, timeout string, said io.Writer) *cobra.Command {
	t.Helper()

	dir := t.TempDir()
	writeFile(t, dir, `{"timing":{"request_timeout":"`+timeout+`"}}`)

	root := cli.NewRootCmdOver(unusedPrompt(t), run, nil, environmentFor(t, place{dir: dir, home: t.TempDir()}))
	root.SetArgs([]string{})
	root.SetIn(strings.NewReader(""))
	root.SetOut(io.Discard)
	root.SetErr(said)

	return root
}

// running executes root under command until it returns, which the channel
// it answers says, and which the test waits for before its directories go.
func running(command context.Context, t *testing.T, root *cobra.Command) <-chan error {
	t.Helper()

	returned, ended := make(chan error, 1), make(chan struct{})

	t.Cleanup(func() { <-ended })

	go func() {
		defer close(ended)

		returned <- root.ExecuteContext(command)
	}()

	return returned
}

// quit types q, which ends the interface.
func quit(typing io.Writer) {
	go func() { _, _ = typing.Write([]byte("q")) }()
}

// returnedWith is what workflow returned, or a failure once patience runs
// out.
func returnedWith(t *testing.T, returned <-chan error) error {
	t.Helper()

	select {
	case err := <-returned:
		return err
	case <-time.After(signalPatience):
		t.Fatal("workflow had not returned")

		return nil
	}
}

func TestAQuitWaitsForAWriteInFlightToLand(t *testing.T) {
	t.Parallel()

	// Arrange
	cache, said := newHeldCache(t), &lockedBuffer{}
	keys, typing := io.Pipe()

	t.Cleanup(func() { _ = keys.Close() })

	returned := running(t.Context(), t, quittingRoot(t, interfaceCaching(cache.write, keys), "1m", said))
	cache.awaitBegun(t)

	// Act: quit with the cache write in flight
	quit(typing)

	// Assert: workflow waits for it
	select {
	case err := <-returned:
		t.Fatalf("workflow returned %v with the cache write in flight, want it to wait for the write", err)
	case <-time.After(stillWaiting):
	}

	// Act: let the write land
	cache.letLand()

	// Assert: the write landed, and workflow exited once it had
	err := returnedWith(t, returned)
	if err != nil || cache.landedWith(t) != nil {
		t.Errorf("workflow = %v, the write ended with %v; want both to finish cleanly", err, cache.landedWith(t))
	}

	if !strings.Contains(said.String(), "finishing 1 write…") {
		t.Errorf("workflow said %q, want it to say it was finishing the write", said.String())
	}
}

func TestAQuitGivesUpOnAWriteThatDoesNotLandInTime(t *testing.T) {
	t.Parallel()

	// Arrange
	cache, said := newHeldCache(t), &lockedBuffer{}
	keys, typing := io.Pipe()

	t.Cleanup(func() { _ = keys.Close() })

	returned := running(t.Context(), t, quittingRoot(t, interfaceCaching(cache.write, keys), "200ms", said))
	cache.awaitBegun(t)

	// Act
	quit(typing)

	// Assert
	err := returnedWith(t, returned)
	if err == nil || !strings.Contains(err.Error(), "gave up") || !strings.Contains(err.Error(), cachingLabel) {
		t.Errorf("workflow = %v, want it to give up on %s", err, cachingLabel)
	}

	if !strings.Contains(said.String(), "finishing 1 write…") {
		t.Errorf("workflow said %q, want it to say it was finishing the write", said.String())
	}
}

func TestASignalWhileWaitingForWritesExitsAtOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	cache, said := newHeldCache(t), &lockedBuffer{}
	keys, typing := io.Pipe()

	t.Cleanup(func() { _ = keys.Close() })

	command, signaled := context.WithCancelCause(t.Context())
	returned := running(command, t, quittingRoot(t, interfaceCaching(cache.write, keys), "1m", said))
	cache.awaitBegun(t)
	quit(typing)
	awaitSaid(t, said, "finishing 1 write…")

	// Act
	signaled(errSignaled)

	// Assert
	err := returnedWith(t, returned)
	if !errors.Is(err, context.Canceled) || cli.ExitStatus(err) != 130 {
		t.Errorf("workflow = %v (status %d), want it interrupted, status 130", err, cli.ExitStatus(err))
	}

	landed := cache.landedWith(t)
	if !errors.Is(landed, context.Canceled) {
		t.Errorf("the cache write ended with %v, want the signal to have ended it", landed)
	}
}

// A signal ends the wait itself, not only the writes it waits for: a write
// that goes on past its context, as a Slack token's refresh or the store's
// open does, is left to the exit.
func TestASignalWhileWaitingExitsUnderAWriteThatIgnoresIt(t *testing.T) {
	t.Parallel()

	// Arrange
	cache, said := newHeldCache(t), &lockedBuffer{}
	keys, typing := io.Pipe()

	t.Cleanup(func() { _ = keys.Close() })

	command, signaled := context.WithCancelCause(t.Context())
	root := quittingRoot(t, interfaceCaching(cache.writeIgnoringItsContext, keys), "1m", said)
	returned := running(command, t, root)
	cache.awaitBegun(t)
	quit(typing)
	awaitSaid(t, said, "finishing 1 write…")

	// Act
	signaled(errSignaled)

	// Assert
	err := returnedWith(t, returned)
	if !errors.Is(err, context.Canceled) || cli.ExitStatus(err) != 130 {
		t.Errorf("workflow = %v (status %d), want it interrupted, status 130", err, cli.ExitStatus(err))
	}

	select {
	case landed := <-cache.landed:
		t.Errorf("the cache write ended with %v before workflow returned, want workflow to exit under it", landed)
	default:
	}
}

func TestAQuitWithNoWriteInFlightExitsAtOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	said := &lockedBuffer{}
	root := quittingRoot(t, interfaceCaching(nil, strings.NewReader("q")), "1m", said)

	// Act
	returned := running(t.Context(), t, root)

	// Assert
	select {
	case err := <-returned:
		if err != nil || said.String() != "" {
			t.Errorf("workflow = %v, said %q; want it to exit cleanly, saying nothing", err, said.String())
		}
	case <-time.After(signalPatience):
		t.Fatal("workflow had not returned after a quit with nothing in flight")
	}
}

// quittingOnceBegun is an interface whose issue list is cached through cache,
// and which quits once that write has begun.
func quittingOnceBegun(t *testing.T, cache *heldCache) cli.RunInterface {
	t.Helper()

	keys, typing := io.Pipe()

	t.Cleanup(func() { _ = keys.Close() })

	go func() {
		select {
		case <-cache.entered:
			quit(typing)
		case <-t.Context().Done():
		}
	}()

	return interfaceCaching(cache.write, keys)
}

// A switch does not wait for the writes the interface it leaves sent; the
// quit that ends the next one waits for them too.
func TestAQuitWaitsForTheWritesSentBeforeASwitch(t *testing.T) {
	t.Parallel()

	// Arrange
	start, other := twoDirectories(t)
	left, stayed, said := newHeldCache(t), newHeldCache(t), &lockedBuffer{}
	interfaces := []cli.RunInterface{quittingOnceBegun(t, left), quittingOnceBegun(t, stayed)}
	opened := 0
	switchingOnce := func(ctx context.Context, model tui.Model, in io.Reader, out io.Writer) (tui.Next, error) {
		opened++

		next, err := interfaces[opened-1](ctx, model, in, out)
		if opened == 1 {
			next = tui.Next{Dir: other}
		}

		return next, err
	}
	env := environmentFor(t, place{dir: start, home: t.TempDir()})
	root := cli.NewRootCmdOver(unusedPrompt(t), switchingOnce, nil, env)
	root.SetArgs([]string{})
	root.SetIn(strings.NewReader(""))
	root.SetOut(io.Discard)
	root.SetErr(said)

	// Act: switch with a write in flight, then quit with another
	returned := running(t.Context(), t, root)

	// Assert: the quit waits for both
	awaitSaid(t, said, "finishing 2 writes…")

	// Act: let both land
	left.letLand()
	stayed.letLand()

	// Assert: workflow exits once they have
	err := returnedWith(t, returned)
	if err != nil || left.landedWith(t) != nil || stayed.landedWith(t) != nil {
		t.Errorf("workflow = %v, want it to exit cleanly once both writes landed", err)
	}
}

// awaitSaid waits for workflow to say want on stderr, failing the test once
// patience runs out.
func awaitSaid(t *testing.T, said *lockedBuffer, want string) {
	t.Helper()

	deadline := time.Now().Add(signalPatience)
	for !strings.Contains(said.String(), want) {
		if time.Now().After(deadline) {
			t.Fatalf("workflow said %q, never %q", said.String(), want)
		}

		time.Sleep(10 * time.Millisecond)
	}
}
