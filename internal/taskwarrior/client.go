// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// How long each kind of call may take before it is stopped.
const (
	// readTimeout bounds a read: the whole pending set exports in milliseconds.
	readTimeout = 10 * time.Second
	// writeTimeout bounds a write, generously: the taskrc's hooks run inside it.
	writeTimeout = 30 * time.Second
	// syncTimeout bounds a sync, which waits on a server or a bucket.
	syncTimeout = 2 * time.Minute
)

// Client drives one Taskwarrior.
type Client struct {
	run     Runner
	install Install
}

// New builds a client for the Taskwarrior install describes, run through run.
func New(run Runner, install Install) Client {
	return Client{run: run, install: install}
}

// List is the pending tasks of the active context, most urgent first, and the
// context's name ("" for none). Pending applies the context itself, read from
// _show, since export ignores it.
type List struct {
	Tasks   []Task
	Context string
}

// pendingOrWaiting is the status filter Pending reads by, as one word:
// status:pending alone leaves out a task that waits, though export writes its
// status as pending.
const pendingOrWaiting = "( status:pending or status:waiting )"

// Pending is the pending tasks the active context shows, waiting ones
// included, and its name. Export ignores the context, so Pending reads the
// context and its read filter from _show and puts the filter before the status
// filter, in parentheses as one word, so an or inside it stays inside it.
func (c Client) Pending(ctx context.Context) (List, error) {
	name, filter, err := c.contextFilter(ctx)
	if err != nil {
		return List{}, err
	}

	tasks, err := c.export(ctx, append(filter, pendingOrWaiting)...)
	if err != nil {
		return List{}, err
	}

	return List{Tasks: ByUrgency(tasks), Context: name}, nil
}

// Touched is every task the active context shows that changed after since,
// whatever its status but deleted: a task completed is work done. Taskwarrior
// keeps a task's times to the second and reads after as strictly after, so
// the second before since is asked for.
func (c Client) Touched(ctx context.Context, since time.Time) ([]Task, error) {
	_, filter, err := c.contextFilter(ctx)
	if err != nil {
		return nil, err
	}

	after := since.UTC().Truncate(time.Second).Add(-time.Second).Format(time.RFC3339)

	return c.export(ctx, append(filter, "modified.after:"+after, "status.not:deleted")...)
}

// Linked is every task that names a Jira issue and is not deleted, whatever
// the active context hides.
func (c Client) Linked(ctx context.Context) ([]Task, error) {
	return c.export(ctx, "rc.context=", LinkUDA+".any:", "status.not:deleted")
}

// contextFilter is the active context's name and its read filter as a filter
// word to put first, in parentheses as one word, so an or inside it stays
// inside it; or no word for no context.
func (c Client) contextFilter(ctx context.Context) (string, []string, error) {
	name, readFilter, err := c.activeContext(ctx)
	if err != nil || readFilter == "" {
		return name, nil, err
	}

	return name, []string{"( " + readFilter + " )"}, nil
}

// activeContext is the active context's name, made safe to show on one line,
// and its read filter as Taskwarrior printed it, since it goes back to
// Taskwarrior and is never shown; or "" for none. They come from one _show run
// with the base read overrides alone, and only the lines that name them are
// read: _show prints every setting, secrets included.
func (c Client) activeContext(ctx context.Context) (string, string, error) {
	out, err := c.run(ctx, readTimeout, proc.Command{
		Name: c.install.Program, Args: append(baseReadOverrides(), "_show"),
	}, nil)
	if err != nil {
		return "", "", readFailure(c.install.Program, err)
	}

	show := string(out)

	name, _ := shownSetting(show, "context")
	if name == "" {
		return "", "", nil
	}

	filter, found := shownSetting(show, "context."+name+".read")
	if !found {
		// Taskwarrior 3.5.0 still honors a context defined the legacy way, but
		// only with no .read line at all: an empty one filters nothing.
		filter, _ = shownSetting(show, "context."+name)
	}

	return sanitize.Line(name), filter, nil
}

// shownSetting is the value _show prints for the setting name, and whether it
// prints a line for it at all: an empty value is not an absent one.
func shownSetting(show, name string) (string, bool) {
	for line := range strings.Lines(show) {
		setting, value, _ := strings.Cut(strings.TrimSpace(line), "=")
		if setting == name {
			return strings.TrimSpace(value), true
		}
	}

	return "", false
}

// export reads the tasks filter matches, as export writes them.
func (c Client) export(ctx context.Context, filter ...string) ([]Task, error) {
	args := append(readOverrides(), filter...)

	out, err := c.run(ctx, readTimeout, proc.Command{Name: c.install.Program, Args: append(args, "export")}, nil)
	if err != nil {
		return nil, readFailure(c.install.Program, err)
	}

	var wire []wireTask

	err = json.Unmarshal(sanitize.JSON(out), &wire)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadOutput, err)
	}

	return decodeTasks(wire)
}

// readOverrides go first on every read: the base read overrides, and the link
// UDAs known.
func readOverrides() []string {
	return append(baseReadOverrides(), linkUDAOverrides()...)
}

// baseReadOverrides are nothing printed but the answer, no hook or garbage
// collection or recurrence to change anything, the JSON as one array, and no
// color. _show runs with these alone.
func baseReadOverrides() []string {
	return []string{
		"rc.verbose=nothing", "rc.hooks=off", "rc.gc=off", "rc.recurrence=off",
		"rc.json.array=on", "rc.color=off", "rc.detection=off", "rc.confirmation=off",
	}
}

// writeOverrides go first on every write: nothing printed but the answer and
// nothing to confirm, since a prompt on a captured stdin waits out the bound.
// Hooks stay as the taskrc has them, so a timewarrior hook still fires.
func writeOverrides() []string {
	return append([]string{
		"rc.verbose=nothing", "rc.confirmation=off", "rc.recurrence.confirmation=no",
		"rc.dependency.confirmation=off", "rc.bulk=0", "rc.color=off", "rc.detection=off", "rc.json.array=on",
	}, linkUDAOverrides()...)
}

// linkUDAOverrides define the link UDAs for one call, since a UDA Taskwarrior
// does not know turns jiraid:PROJ-42 into a search word that matches nothing.
func linkUDAOverrides() []string {
	return []string{
		"rc.uda." + LinkUDA + ".type=string", "rc.uda." + LinkUDA + ".label=Jira",
		"rc.uda." + LinkURLUDA + ".type=string", "rc.uda." + LinkURLUDA + ".label=Jira URL",
	}
}

// readFailure words a read of the Taskwarrior at program that ended badly: one
// never run is told apart by the one line it prints, and named, and any other
// exit is a refusal — a read changes nothing, so an exit 1 is not a write
// declined.
func readFailure(program string, err error) error {
	_, stderr, exited := proc.Failure(err)

	switch {
	case exited && !malformedTaskrc(stderr) && lacksTaskrc(stderr):
		return NeverRunError{Program: program}
	case exited:
		return refusal(stderr)
	default:
		return err // proc.ErrNotFound, proc.ErrTimedOut, a canceled context
	}
}

// outcome reads how a write ended: exit 1 is Taskwarrior declining to change
// anything — a task not pending, already started, or no such uuid — and
// anything else non-zero is a refusal.
func outcome(err error) error {
	if err == nil {
		return nil
	}

	code, stderr, exited := proc.Failure(err)

	switch {
	case exited && code == 1 && !malformedTaskrc(stderr):
		return ErrNothingChanged
	case exited:
		return refusal(stderr)
	default:
		return err // proc.ErrNotFound, proc.ErrTimedOut, a canceled context
	}
}

// refusal is a command Taskwarrior refused, in its own words, but for a taskrc
// line it cannot parse, which is refused in fixed words.
func refusal(stderr string) error {
	if malformedTaskrc(stderr) {
		return fmt.Errorf("%w: taskrc has a malformed line", ErrRefused)
	}

	return fmt.Errorf("%w: %s", ErrRefused, stderr)
}

// malformedTaskrc reports stderr carrying Taskwarrior's complaint about a taskrc
// line it cannot parse. The complaint quotes the whole line, and a secret in it,
// so none of it may reach an error.
func malformedTaskrc(stderr string) bool {
	return strings.Contains(stderr, "Malformed entry '")
}
