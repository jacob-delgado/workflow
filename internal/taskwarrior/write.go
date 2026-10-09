// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// createdTaskLine matches the line add prints under rc.verbose=new-uuid,
// capturing the new task's uuid within it.
var createdTaskLine = regexp.MustCompile(`Created task ([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`)

// revertedOperationsLine matches the first line undo prints when it reverts,
// capturing how many operations it reverted.
var revertedOperationsLine = regexp.MustCompile(`^The following (\d+) operations would be reverted:$`)

// Add creates a task from line, in Taskwarrior's own grammar, and returns the
// new task's uuid.
func (c Client) Add(ctx context.Context, line string) (string, error) {
	words := strings.Fields(line)
	if len(words) == 0 {
		return "", fmt.Errorf("%w: nothing to add", ErrRefused)
	}

	out, err := c.write(ctx, writeTimeout, append([]string{"rc.verbose=new-uuid", "add"}, words...)...)
	if err != nil {
		return "", err
	}

	created := createdTaskLine.FindSubmatch(out)
	if created == nil {
		return "", ErrBadOutput
	}

	return string(created[1]), nil
}

// Start starts the task uuid names, whatever the active context hides.
func (c Client) Start(ctx context.Context, uuid string) error {
	return c.change(ctx, uuid, "start")
}

// Stop stops the task uuid names, whatever the active context hides.
func (c Client) Stop(ctx context.Context, uuid string) error {
	return c.change(ctx, uuid, "stop")
}

// Done completes the task uuid names, whatever the active context hides.
func (c Client) Done(ctx context.Context, uuid string) error {
	return c.change(ctx, uuid, "done")
}

// Annotate adds text to the task uuid names as an annotation, whatever the
// active context hides. The text follows --, so a word in it that looks like an
// attribute stays a word.
func (c Client) Annotate(ctx context.Context, uuid, text string) error {
	return c.change(ctx, uuid, append([]string{"annotate", "--"}, strings.Fields(text)...)...)
}

// Modify changes the task uuid names by line, in Taskwarrior's own grammar,
// whatever the active context hides.
func (c Client) Modify(ctx context.Context, uuid, line string) error {
	return c.change(ctx, uuid, append([]string{"modify"}, strings.Fields(line)...)...)
}

// Undo reverts Taskwarrior's last change and says how many operations that
// was: "reverted 3 operations", "reverted 1 operation", or "" when Taskwarrior
// did not count them. ErrNothingChanged when there was nothing to undo, which
// Taskwarrior reports with exit 0.
func (c Client) Undo(ctx context.Context) (string, error) {
	out, err := c.write(ctx, writeTimeout, "undo")
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(string(out), "No operations to undo") {
		return "", ErrNothingChanged
	}

	return reverted(out), nil
}

// reverted reads how many operations undo reverted from the first line it
// printed.
func reverted(out []byte) string {
	firstLine, _, _ := strings.Cut(string(out), "\n")

	count := revertedOperationsLine.FindStringSubmatch(strings.TrimSpace(firstLine))

	switch {
	case count == nil:
		return ""
	case count[1] == "1":
		return "reverted 1 operation"
	default:
		return "reverted " + count[1] + " operations"
	}
}

// Sync syncs with the backend taskrc names and returns what Taskwarrior
// printed; ErrNoSync when it names none.
func (c Client) Sync(ctx context.Context) (string, error) {
	if !c.install.SyncConfigured {
		return "", ErrNoSync
	}

	out, err := c.write(ctx, syncTimeout, "sync")
	if err != nil {
		return "", err
	}

	return said(out), nil
}

// change runs one write on the task uuid names with the context bypassed:
// Taskwarrior applies the context's read filter to a uuid: filter too, so a
// task the context hides would not be found.
func (c Client) change(ctx context.Context, uuid string, words ...string) error {
	_, err := c.write(ctx, writeTimeout, append([]string{"rc.context=", "uuid:" + uuid}, words...)...)

	return err
}

// write runs one command after the write overrides and reads how it ended.
func (c Client) write(ctx context.Context, timeout time.Duration, words ...string) ([]byte, error) {
	out, err := c.run(ctx, timeout, proc.Command{Name: c.install.Program, Args: append(writeOverrides(), words...)}, nil)

	return out, outcome(err)
}

// said is what Taskwarrior printed, safe to show.
func said(out []byte) string {
	return strings.TrimSpace(sanitize.Text(string(out)))
}
