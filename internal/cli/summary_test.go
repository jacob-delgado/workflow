// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// summaryDay is the one day the cases read, and workedOn a time on it.
const (
	summaryDay = "2026-10-01"
	workedOn   = summaryDay + "T10:00:00"
	asJSON     = "--json"
)

// summaryCommand is the command these tests run.
const summaryCommand = "summary"

// datedCommit makes a commit in dir authored at when, as the identity the
// repository is configured with.
func datedCommit(t *testing.T, dir, message, when string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", "-C", dir, "commit", "--quiet", "--allow-empty", "-m", message)

	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_DATE="+when, "GIT_COMMITTER_DATE="+when)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit: %v (%s)", err, out)
	}
}

// workedRepository is a repository with your identity and one commit of
// yours on summaryDay, subject.
func workedRepository(t *testing.T, subject string) string {
	t.Helper()

	dir := t.TempDir()
	gitInit(t, dir)
	git(t, dir, "config", "user.email", "me@example.com")
	git(t, dir, "config", "user.name", "Me")
	datedCommit(t, dir, subject, workedOn)

	return dir
}

// capturingWebhook is a local https webhook that accepts every post and keeps
// each body's text.
type capturingWebhook struct {
	mu    sync.Mutex
	texts []string
	url   string
}

// newCapturingWebhook serves a capturing webhook for the test's length.
func newCapturingWebhook(t *testing.T) *capturingWebhook {
	t.Helper()

	hook := &capturingWebhook{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Text string `json:"text"`
		}

		_ = json.NewDecoder(request.Body).Decode(&body)

		hook.mu.Lock()
		hook.texts = append(hook.texts, body.Text)
		hook.mu.Unlock()

		_, _ = writer.Write([]byte("ok"))
	}))
	t.Cleanup(server.Close)

	hook.url = server.URL + "/services/x"

	return hook
}

// posted is every text posted so far.
func (h *capturingWebhook) posted() []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return append([]string(nil), h.texts...)
}

func TestSummaryReadsYourCommitsInThePeriod(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := workedRepository(t, "Add the widget")

	// Act
	printed, err := clitest.RunStreams(
		t, repo, clitest.UnusedPrompt(t), "summary", "--from", summaryDay, "--to", summaryDay,
	)

	// Assert
	if !strings.Contains(printed.Stdout, "# "+summaryDay) || !strings.Contains(printed.Stdout, "Add the widget") {
		t.Errorf("summary = %v and printed:\n%s\nwant the period's commit", err, printed.Stdout)
	}
}

func TestSummaryAsJSONIsWhatTheWebAnswersForThePeriod(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := workedRepository(t, "Add the widget")
	query := "?from=" + summaryDay + "&to=2026-10-02"
	web := webActivity(t, repo, query)

	// Act
	printed, _ := clitest.RunStreams(
		t, repo, clitest.UnusedPrompt(t), "summary", "--from", summaryDay, "--to", "2026-10-02", asJSON,
	)

	// Assert
	var got api.Activity

	err := json.Unmarshal([]byte(printed.Stdout), &got)
	if err != nil {
		t.Fatalf("summary --json printed what is not JSON: %v\n%s", err, printed.Stdout)
	}

	if !reflect.DeepEqual(got, web) || !strings.Contains(got.Text, "Add the widget") {
		t.Errorf("summary --json =\n%+v\nwant what GET /api/activity answers:\n%+v", got, web)
	}
}

// webActivity is what GET /api/activity answers with query, the web server
// wired in repo as workflow --web wires it.
func webActivity(t *testing.T, repo, query string) api.Activity {
	t.Helper()

	ran := runRoot(t, repo, "--web")
	if ran.err != nil {
		t.Fatalf("workflow --web: %v", ran.err)
	}

	session := webserver.NewSession()

	world := webserver.World{Deps: ran.deps, Config: ran.cfg, Info: ran.info}

	handler, err := webserver.Handler(t.Context(), world, fstest.MapFS{}, session)
	if err != nil {
		t.Fatalf("webserver.Handler: %v", err)
	}

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	request, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/api/activity"+query, nil)
	request.Header.Set("Authorization", servedAt(t, session.Address(server.Listener.Addr().String())).authorization)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("GET /api/activity: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	body, _ := io.ReadAll(response.Body)

	var answer api.Activity

	err = json.Unmarshal(body, &answer)
	if err != nil {
		t.Fatalf("GET /api/activity answered %d: %s", response.StatusCode, body)
	}

	return answer
}

func TestSummaryRefusesAPeriodItCannotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := workedRepository(t, "Add the widget")

	// Act
	_, err := clitest.Run(t, repo, "summary", "--from", "yesterday")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Errorf("summary --from yesterday = %v, want the date refused, saying how to write one", err)
	}

	wantExit(t, err, 2)
}

func TestSummaryRefusesFlagsThatDoNotGoTogether(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"--json with --post":   {summaryCommand, asJSON, "--post"},
		"--yes without it":     {summaryCommand, "--yes"},
		"--channel without it": {summaryCommand, "--channel", "#x"},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := workedRepository(t, "Add the widget")

			// Act
			_, err := clitest.Run(t, repo, args...)

			// Assert
			if err == nil || !strings.Contains(err.Error(), "--post") {
				t.Errorf("%s = %v, want it refused, naming --post", strings.Join(args, " "), err)
			}

			wantExit(t, err, 2)
		})
	}
}

func TestSummaryPostYesPostsTheTextRenderedForTheService(t *testing.T) {
	t.Parallel()

	// Arrange
	hook := newCapturingWebhook(t)
	repo := workedRepository(t, "Add the widget")
	clitest.WriteFile(t, repo, `{"messaging":{"webhook_url":"`+hook.url+`"}}`)

	// Act
	printed, _ := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "summary", "--from", summaryDay, "--to", summaryDay,
		"--post", "--yes")

	// Assert
	posts := hook.posted()
	if len(posts) != 1 || !strings.Contains(posts[0], "*"+summaryDay+"*") ||
		!strings.Contains(posts[0], "Add the widget") {
		t.Errorf("summary --post --yes posted %q, want the summary once, its headings Slack's bold lines", posts)
	}

	if !strings.Contains(printed.Stderr, "Posted to the channel its webhook is bound to.") {
		t.Errorf("summary --post --yes said:\nstdout:\n%s\nstderr:\n%s\nwant that it was posted",
			printed.Stdout, printed.Stderr)
	}
}

func TestSummaryPostPrintsOnlyTheSummaryOnStdout(t *testing.T) {
	t.Parallel()

	// Arrange
	hook := newCapturingWebhook(t)
	repo := workedRepository(t, "Add the widget")
	clitest.WriteFile(t, repo, `{"messaging":{"webhook_url":"`+hook.url+`"}}`)
	read, _ := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "summary", "--from", summaryDay, "--to", summaryDay)

	// Act
	printed, _ := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "summary", "--from", summaryDay, "--to", summaryDay,
		"--post", "--yes")

	// Assert
	if printed.Stdout != read.Stdout || !strings.Contains(printed.Stderr, "to the channel its webhook is bound to") {
		t.Errorf("summary --post --yes printed:\nstdout:\n%s\nstderr:\n%s\nwant the summary alone on stdout, "+
			"as without --post:\n%s", printed.Stdout, printed.Stderr, read.Stdout)
	}
}

func TestSummaryPostRefusesASummaryTooLongForTheServiceBeforeAsking(t *testing.T) {
	t.Parallel()

	// Arrange
	hook := newCapturingWebhook(t)
	repo := workedRepository(t, strings.Repeat("x", 2100))
	clitest.WriteFile(t, repo, `{"messaging":{"kind":"discord","webhook_url":"`+hook.url+`"}}`)

	// Act
	printed, err := clitest.RunStreams(
		t, repo, clitest.UnusedPrompt(t), "summary", "--from", summaryDay, "--to", summaryDay, "--post",
	)

	// Assert
	if posts := hook.posted(); len(posts) != 0 || err == nil ||
		!strings.Contains(err.Error(), "too long for Discord (") ||
		!strings.Contains(err.Error(), " of 2000 characters); pick a shorter period") {
		t.Errorf("summary --post = %v after %d posts, said:\n%s\nwant it refused, naming the length, nothing posted",
			err, len(posts), printed.Stderr)
	}

	wantExit(t, err, 4)
}

func TestSummaryPostSaysHowLongItIsAgainstTheServicesLimit(t *testing.T) {
	t.Parallel()

	// Arrange
	hook := newCapturingWebhook(t)
	repo := workedRepository(t, "Add the widget")
	clitest.WriteFile(t, repo, `{"messaging":{"kind":"discord","webhook_url":"`+hook.url+`"}}`)

	// Act
	printed, _ := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "summary", "--from", summaryDay, "--to", summaryDay,
		"--post", "--dry-run")

	// Assert
	if !strings.Contains(printed.Stderr, " of 2000 characters") {
		t.Errorf("summary --post --dry-run said:\n%s\nwant the length against Discord's 2000 characters",
			printed.Stderr)
	}
}

func TestSummaryPostDryRunPostsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	hook := newCapturingWebhook(t)
	repo := workedRepository(t, "Add the widget")
	clitest.WriteFile(t, repo, `{"messaging":{"webhook_url":"`+hook.url+`"}}`)

	// Act
	printed, _ := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "summary", "--post", "--yes", "--dry-run")

	// Assert
	if posts := hook.posted(); len(posts) != 0 ||
		!strings.Contains(printed.Stderr, "dry run: would post to the channel its webhook is bound to") {
		t.Errorf("summary --post --yes --dry-run posted %q and said:\n%s\nwant nothing posted, and what would be",
			posts, printed.Stderr)
	}
}

func TestSummaryPostIsNotPostedWhenDeclined(t *testing.T) {
	t.Parallel()

	// Arrange
	hook := newCapturingWebhook(t)
	repo := workedRepository(t, "Add the widget")
	clitest.WriteFile(t, repo, `{"messaging":{"kind":"teams","webhook_url":"`+hook.url+`"}}`)

	// Act
	printed, _ := clitest.RunStreams(t, repo, scripted([]string{"n"}, nil), "summary", "--post")

	// Assert
	if posts := hook.posted(); len(posts) != 0 || !strings.Contains(printed.Stderr, "Not posted.") {
		t.Errorf("a declined summary --post posted %q and said:\n%s\nwant nothing posted, and that it was not",
			posts, printed.Stderr)
	}
}

func TestSummaryPostWithoutMessagingSaysHowToSetItUp(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := workedRepository(t, "Add the widget")

	// Act
	_, err := clitest.Run(t, repo, "summary", "--post", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "no messaging transport is configured") {
		t.Errorf("summary --post with no messaging = %v, want it refused, saying so", err)
	}

	wantExit(t, err, 3)
}

func TestSummaryNamesASourceItCouldNotReadAfterPrintingTheRest(t *testing.T) {
	t.Parallel()

	// Arrange
	jira := clitest.JiraServer(t, http.StatusUnauthorized, `{}`, new(atomic.Bool))
	repo := workedRepository(t, "Add the widget")
	clitest.WriteFile(t, repo, `{"jira":{"base_url":"`+jira.URL+`","token":"t"}}`)

	// Act
	printed, err := clitest.RunStreams(
		t, repo, clitest.UnusedPrompt(t), "summary", "--from", summaryDay, "--to", summaryDay,
	)

	// Assert
	if !strings.Contains(printed.Stdout, "Add the widget") || err == nil ||
		!strings.Contains(err.Error(), "Jira could not be read") {
		t.Errorf("summary = %v, printing:\n%s\nwant the commit printed, then Jira named as unread", err, printed.Stdout)
	}

	wantExit(t, err, 3)
}

func TestSummaryNeutralizesWhatASourceSaysBeforeTheTerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := workedRepository(t, "Fix \x1b]0;owned\x07 the title")

	// Act
	printed, _ := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "summary", "--from", summaryDay, "--to", summaryDay)

	// Assert
	if strings.ContainsRune(printed.Stdout, '\x1b') || strings.ContainsRune(printed.Stdout, '\x07') {
		t.Errorf("summary printed a terminal control from a commit subject:\n%q", printed.Stdout)
	}
}

func TestStandupIsNoLongerACommand(t *testing.T) {
	t.Parallel()

	// Act
	_, err := clitest.Run(t, t.TempDir(), "standup")

	// Assert
	if err == nil || !strings.Contains(err.Error(), `unknown command "standup"`) {
		t.Errorf("workflow standup = %v, want it unknown, summary --post having replaced it", err)
	}

	wantExit(t, err, 2)
}

func TestSummaryPostReportsAFailedPostByItsService(t *testing.T) {
	t.Parallel()

	// Arrange
	// The webhook cannot be reached, so the confirmed post fails.
	repo := workedRepository(t, "Add the widget")
	clitest.WriteFile(t, repo, `{"messaging":{"kind":"teams","webhook_url":"https://hooks.teams.example/services/x"}}`)

	// Act
	_, err := clitest.RunGuided(t, repo, scripted([]string{"y"}, nil), "summary", "--post")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "posting to Teams") {
		t.Errorf("summary --post = %v, want the failed post named by its service", err)
	}
}

func TestSummaryPostStopsAtTheQuestionWhenNothingCanAnswer(t *testing.T) {
	t.Parallel()

	// Arrange
	hook := newCapturingWebhook(t)
	repo := workedRepository(t, "Add the widget")
	clitest.WriteFile(t, repo, `{"messaging":{"webhook_url":"`+hook.url+`"}}`)

	// Act
	_, err := clitest.RunStreams(t, repo, cli.Prompt{Line: answersThenEnds()}, "summary", "--post")

	// Assert
	if posts := hook.posted(); err == nil || !strings.Contains(err.Error(), "pass --yes") || len(posts) != 0 {
		t.Errorf("summary --post = %v after posting %q, want it to stop at the question, saying to pass --yes",
			err, posts)
	}

	wantExit(t, err, 2)
}
