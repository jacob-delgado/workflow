// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// errCommentBlank refuses a comment with no text: standard input held nothing
// but white space.
var errCommentBlank = errors.New("a comment needs text on standard input")

// commentSeams are what `workflow comment` reads and does, so a test can
// answer without a tracker.
type commentSeams struct {
	Comment func(issueKey jira.Key, text string) (jira.Comment, error)
	// Markup is what a comment on an issue is written in, which decides what
	// it is stored as.
	Markup  func(issueKey jira.Key) loop.CommentMarkup
	Confirm func(question string) (bool, error)
}

// newCommentCmd builds `workflow comment <issue>`.
func newCommentCmd(prompt Prompt) *cobra.Command {
	var opts writeOptions

	cmd := &cobra.Command{
		Use:   "comment <issue>",
		Short: "Comment on an issue, in Jira or on the forge, with the text on standard input",
		Long: "Comment on an issue with the text read from standard input to its end — as\n" +
			"`git commit -F -` reads a message — so a script never quotes it; at a terminal,\n" +
			"end the text with Ctrl+D. A Jira issue's comment is written in Markdown and\n" +
			"posted as Jira's markup when jira.markdown_comments is on, as the interface\n" +
			"posts one; a forge issue's is Markdown. The comment, as it will be stored, is\n" +
			"printed and confirmed before it is posted.",
		Example: examples(
			`git log -1 --format=%B | workflow comment PROJ-7 --yes   # the text on stdin, never quoted`,
		),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommentCommand(cmd, prompt, args[0], opts)
		},
	}

	opts.addFlags(cmd, confirmationHelp)

	return cmd
}

// runCommentCommand wires the real tracker to the comment flow.
func runCommentCommand(cmd *cobra.Command, prompt Prompt, issueKey string, opts writeOptions) error {
	text, err := readInput(prompt.Input)
	if err != nil {
		return err
	}

	conn, err := connect(cmd)
	if err != nil {
		return err
	}
	defer conn.closeLog()

	seams := commentSeams{
		Comment: conn.deps.Jira.Comment,
		Markup:  func(issueKey jira.Key) loop.CommentMarkup { return loop.CommentMarkupOf(conn.cfg.Jira, issueKey) },
		Confirm: func(question string) (bool, error) { return confirm(prompt, question) },
	}

	return runComment(outputOf(cmd), seams, jira.Key(issueKey), text, opts)
}

// readInput is the text on input, to its end, without the white space a
// shell's echo or a heredoc leaves after it. Text that is only white space is
// refused before anything is asked.
func readInput(input io.Reader) (string, error) {
	if input == nil {
		return "", errCommentBlank
	}

	read, err := io.ReadAll(input)
	if err != nil {
		return "", fmt.Errorf("reading the comment: %w", err)
	}

	text := strings.TrimRight(string(read), " \t\r\n")
	if strings.TrimSpace(text) == "" {
		return "", errCommentBlank
	}

	return text, nil
}

// runComment previews the comment as its tracker will store it, and posts it
// once confirmed.
func runComment(out output, seams commentSeams, issueKey jira.Key, text string, opts writeOptions) error {
	stored := seams.Markup(issueKey).Stored(text)
	key := string(issueKey)

	fmt.Fprintln(out.artifact, "Comment on "+key+":")
	fmt.Fprintln(out.artifact, stored)

	proceed, err := opts.proceed(out.notes, seams.Confirm, writePrompt{
		question: "Post this comment on " + key + "?",
		dryRun:   "dry run: would comment on " + key,
		declined: "Not posted.",
	})
	if err != nil || !proceed {
		return err
	}

	_, err = seams.Comment(issueKey, stored)
	if err != nil {
		return fmt.Errorf("commenting on %s: %w", key, err)
	}

	fmt.Fprintln(out.notes, "Commented on "+key+".")

	return nil
}
