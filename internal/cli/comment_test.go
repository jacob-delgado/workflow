// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/iotest"

	"github.com/jacob-delgado/workflow/internal/cli"
)

// errUnreadableInput is standard input failing part way through a read.
var errUnreadableInput = errors.New("input: device not ready")

// commentsPosted records the body of each comment a fake Jira was asked to
// post. A mutex guards it because `task test` runs -race and the handler
// answers on its own goroutine.
type commentsPosted struct {
	mu     sync.Mutex
	bodies []string
}

// all is every comment body posted, in order.
func (c *commentsPosted) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]string(nil), c.bodies...)
}

// commentJira is a Jira that takes comments on PROJ-7 and records them, with
// a configuration pointing at it in a new directory, which it returns.
func commentJira(t *testing.T, markdown bool) (string, *commentsPosted) {
	t.Helper()

	posted := &commentsPosted{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/rest/api/2/issue/PROJ-7/comment" {
			writer.WriteHeader(http.StatusNotFound)

			return
		}

		var sent struct {
			Body string `json:"body"`
		}

		raw, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(raw, &sent)

		posted.mu.Lock()
		posted.bodies = append(posted.bodies, sent.Body)
		posted.mu.Unlock()

		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"author":{"displayName":"Ana"},"body":"posted"}`))
	}))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	writeFile(t, dir, `{"jira":{"base_url":"`+server.URL+`","token":"t","markdown_comments":`+
		strconv.FormatBool(markdown)+`}}`)

	return dir, posted
}

// typed is a prompt that asks nothing and reads text from standard input.
func typed(t *testing.T, text string) cli.Prompt {
	t.Helper()

	prompt := unusedPrompt(t)
	prompt.Input = strings.NewReader(text)

	return prompt
}

func TestCommentPostsTheTextFromStandardInput(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, posted := commentJira(t, false)

	// Act
	printed, err := runStreams(t, dir, typed(t, "Looks good; it's \"quoted\" & all.\n"), "comment", "PROJ-7", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("comment --yes: %v (%+v)", err, printed)
	}

	if got := posted.all(); len(got) != 1 || got[0] != `Looks good; it's "quoted" & all.` {
		t.Errorf("comments posted = %q, want the text read, as typed, once", got)
	}

	if !strings.Contains(printed.stdout, `Looks good; it's "quoted" & all.`) ||
		!strings.Contains(printed.stderr, "Commented on PROJ-7.") {
		t.Errorf("comment did not preview on stdout and say it posted on stderr:\nstdout:\n%s\nstderr:\n%s",
			printed.stdout, printed.stderr)
	}
}

func TestCommentPostsMarkdownAsJirasMarkup(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, posted := commentJira(t, true)

	// Act
	printed, err := runStreams(t, dir, typed(t, "**bold**"), "comment", "PROJ-7", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("comment --yes: %v (%+v)", err, printed)
	}

	// Converted as the terminal's and the web's comments are, and previewed as
	// it will be stored.
	if got := posted.all(); len(got) != 1 || got[0] != "*bold*" || !strings.Contains(printed.stdout, "*bold*") {
		t.Errorf("comments posted = %q, previewed %q; want Markdown posted as Jira's *bold*", got, printed.stdout)
	}
}

func TestCommentDryRunPostsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, posted := commentJira(t, false)

	// Act
	printed, err := runStreams(t, dir, typed(t, "later"), "comment", "PROJ-7", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("comment --dry-run: %v (%+v)", err, printed)
	}

	if got := posted.all(); len(got) != 0 || !strings.Contains(printed.stderr, "dry run: would comment on PROJ-7") {
		t.Errorf("comment --dry-run posted %q and said:\n%s", got, printed.stderr)
	}
}

func TestCommentRefusesBlankText(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, posted := commentJira(t, false)

	// Act
	_, err := runStreams(t, dir, typed(t, " \n\n"), "comment", "PROJ-7", "--yes")

	// Assert
	wantExit(t, err, 2)

	if got := posted.all(); len(got) != 0 {
		t.Errorf("a blank comment was posted: %q", got)
	}
}

func TestCommentWithNothingToAnswerSaysToPassYes(t *testing.T) {
	t.Parallel()

	// Arrange
	// The text took standard input to its end, so the question has nothing
	// left to read an answer from.
	dir, posted := commentJira(t, false)
	prompt := cli.Prompt{
		Line:  func(string) (string, error) { return "", io.EOF },
		Input: strings.NewReader("hello"),
	}

	// Act
	_, err := runStreams(t, dir, prompt, "comment", "PROJ-7")

	// Assert
	wantExit(t, err, 2)

	if err == nil || !strings.Contains(err.Error(), "--yes") || len(posted.all()) != 0 {
		t.Errorf("comment = %v, posted %q; want nothing posted and --yes named", err, posted.all())
	}
}

func TestCommentWithNoStandardInputRefusesBlankText(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, posted := commentJira(t, false)
	prompt := unusedPrompt(t)
	prompt.Input = nil

	// Act
	_, err := runStreams(t, dir, prompt, "comment", "PROJ-7", "--yes")

	// Assert
	wantExit(t, err, 2)

	if got := posted.all(); len(got) != 0 {
		t.Errorf("a comment was posted with no input: %q", got)
	}
}

func TestCommentWhoseInputCannotBeReadFails(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, posted := commentJira(t, false)
	prompt := unusedPrompt(t)
	prompt.Input = iotest.ErrReader(errUnreadableInput)

	// Act
	_, err := runStreams(t, dir, prompt, "comment", "PROJ-7", "--yes")

	// Assert
	if !errors.Is(err, errUnreadableInput) || len(posted.all()) != 0 {
		t.Errorf("comment = %v, posted %q; want the read's error and nothing posted", err, posted.all())
	}
}

func TestCommentOnAnIssueJiraLacksSaysWhichIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, posted := commentJira(t, false)

	// Act
	printed, err := runStreams(t, dir, typed(t, "hello"), "comment", "PROJ-8", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "commenting on PROJ-8") || len(posted.all()) != 0 ||
		strings.Contains(printed.stderr, "Commented on") {
		t.Errorf("comment = %v, posted %q, said %q; want PROJ-8's failure and no success", err, posted.all(),
			printed.stderr)
	}
}

func TestCommentWithAConfigurationItCannotReadPostsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, posted := commentJira(t, false)
	writeFile(t, dir, `{`)

	// Act
	_, err := runStreams(t, dir, typed(t, "hello"), "comment", "PROJ-7", "--yes")

	// Assert
	if err == nil || len(posted.all()) != 0 {
		t.Errorf("comment = %v, posted %q; want an error and nothing posted", err, posted.all())
	}
}
