// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/setup"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// Answers a first run types.
const (
	firstRunJira    = "https://jira.example.com"
	firstRunToken   = "first-run-token-4242"
	firstRunWebhook = "https://hooks.slack.com/services/T0/B0/firstrun"
	keychainCommand = "security find-generic-password -s workflow-jira -w"
)

// firstRun is a session with no configuration file and no issues, over a
// setup that writes where the test chose and checks the token with a Jira
// that answers with status, keeping a token in a fake keychain.
type firstRun struct {
	where  setup.Where
	stored string
	status int
}

// jira is a Jira that answers who the token is with the run's status.
func (r *firstRun) jira(request *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: r.status, Header: http.Header{"Content-Type": {"application/json"}},
		Body:    io.NopCloser(strings.NewReader(`{"displayName":"Fred F. User","name":"fred"}`)),
		Request: request,
	}, nil
}

// keep is the fake keychain.
func (r *firstRun) keep(secret string) (string, error) {
	r.stored = secret

	return keychainCommand, nil
}

// newFirstRun is a first run over a Jira that answers with status.
func newFirstRun(t *testing.T, status int) *firstRun {
	t.Helper()

	return &firstRun{where: setup.Where{WorkDir: t.TempDir(), HomeDir: t.TempDir()}, status: status}
}

// model is the session, sized and loaded, and dry run when asked.
func (r *firstRun) model(t *testing.T, dryRun bool) tui.Model {
	t.Helper()

	working := reposWorld()
	working.issues = nil
	deps := working.deps()
	guide := setup.Guide{Where: r.where, Doer: jira.Doer(r.jira), StoreSecret: r.keep}
	deps.Settings.Setup = seams.Setup{
		Offer: guide.Offer,
		Check: func(settings config.Jira) (string, error) { return guide.Check(t.Context(), settings) },
		Write: func(request setup.Request) (setup.Written, error) { return guide.Write(t.Context(), request) },
	}

	model := tui.New(config.Default(), config.ErrNotFound, deps)
	if dryRun {
		model = model.WithDryRun()
	}

	model = sized(t, model, 120, 40)

	return drain(t, model, model.Init())
}

// toTheToken is the keys that open the form, keep the repository, and type
// Jira's address and token, up to the check.
func toTheToken() []string {
	keys := []string{keyEnter, keyEnter}
	keys = append(append(keys, letters(firstRunJira)...), keyEnter)

	return append(append(keys, letters(firstRunToken)...), keyEnter)
}

// throughEveryQuestion is toTheToken, then the keychain row moved to with
// the key given and chosen, and the webhook, up to the last look before the
// write.
func throughEveryQuestion(keychain string) []string {
	keys := append(toTheToken(), keychain, keyEnter)

	return append(append(keys, letters(firstRunWebhook)...), keyEnter)
}

func TestTheNoFileScreenOffersToSetOneUpHere(t *testing.T) {
	t.Parallel()

	// Act
	view := newFirstRun(t, http.StatusOK).model(t, false).View().Content

	// Assert
	requireScreen(t, view, config.NoConfigHeadline, "enter sets one up here.",
		"Or create one with `workflow config init`.")

	if footer := footerLine(view); !strings.Contains(footer, "enter set up") {
		t.Errorf("the footer = %q, want it to offer enter to set up", footer)
	}
}

func TestSetUpChecksTheTokenAndNeverShowsIt(t *testing.T) {
	t.Parallel()

	// Act
	asked := typing(t, newFirstRun(t, http.StatusOK).model(t, false), toTheToken()...)

	// Assert
	view := asked.View().Content
	requireScreen(t, view, "authenticates as Fred F. User (fred)", "Where should the token be kept?")
	refuseScreen(t, view, firstRunToken)
}

func TestSetUpWritesTheFileWithTheTokenInTheKeychainAndReopens(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	answered := typing(t, run.model(t, false), throughEveryQuestion("up")...)

	// Act
	written, cmd := pressed(t, answered, keyEnter)
	written = drain(t, written, cmd)

	// Assert
	if written.Destination().Dir != apiCmd {
		t.Fatalf("the write ended for %q, want workflow reopened in %s", written.Destination().Dir, apiCmd)
	}

	contents, err := os.ReadFile(run.where.Path(setup.Repository))
	if err != nil || strings.Contains(string(contents), firstRunToken) || run.stored != firstRunToken {
		t.Errorf("wrote %q (%v), keychain %q; want the token in the keychain alone", contents, err, run.stored)
	}

	cfg, err := config.LoadFile(run.where.Path(setup.Repository))
	if err != nil || cfg.Jira.BaseURL != firstRunJira || cfg.Messaging.WebhookURL != firstRunWebhook {
		t.Errorf("wrote %+v (%v), want Jira's address and the webhook", cfg, err)
	}
}

func TestSetUpKeepsTheTokenInTheFileWhenTheKeychainIsDeclined(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	answered := typing(t, run.model(t, false), throughEveryQuestion("down")...)

	// Act
	written, cmd := pressed(t, answered, keyEnter)
	drain(t, written, cmd)

	// Assert
	cfg, err := config.LoadFile(run.where.Path(setup.Repository))
	if err != nil || cfg.Jira.Token.Reveal() != firstRunToken || run.stored != "" {
		t.Errorf("wrote %+v (%v), keychain %q; want the token in the file alone", cfg.Jira, err, run.stored)
	}
}

func TestSetUpArrivesOnTheIssuesPaneSayingSo(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	answered := typing(t, run.model(t, false), throughEveryQuestion("up")...)
	written, cmd := pressed(t, answered, keyEnter)
	written = drain(t, written, cmd)
	reopened := sized(t, tui.New(completeConfig(), nil, reposWorld().deps()), 120, 40)

	// Act
	arrived := reopened.Arrived(written.Destination())

	// Assert
	requireScreen(t, arrived.View().Content, "set up with", "Issues")
	refuseScreen(t, arrived.View().Content, "Where should the file go?")
}

func TestSetUpAsksWhatToDoWhenJiraRefusesTheToken(t *testing.T) {
	t.Parallel()

	// Act
	asked := typing(t, newFirstRun(t, http.StatusUnauthorized).model(t, false), toTheToken()...)

	// Assert
	requireScreen(t, asked.View().Content, "The Jira check did not pass", "Jira did not accept the token",
		"Type the token again", "Keep them anyway", "Leave Jira out")
}

func TestSetUpLeavingJiraOutWritesNoJira(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusUnauthorized)

	keys := append(toTheToken(), "down", "down", keyEnter, keyEnter)
	answered := typing(t, run.model(t, false), keys...)

	// Act
	written, cmd := pressed(t, answered, keyEnter)
	drain(t, written, cmd)

	// Assert
	cfg, err := config.LoadFile(run.where.Path(setup.Repository))
	if err != nil || cfg.Jira.BaseURL != "" || cfg.Jira.Token != "" {
		t.Errorf("wrote Jira %+v (%v), want Jira left out", cfg.Jira, err)
	}
}

func TestSetUpGoesToTheHomeDirectoryWhenChosen(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	answered := typing(t, run.model(t, false), keyEnter, "down", keyEnter, keyEnter, keyEnter)

	// Act
	written, cmd := pressed(t, answered, keyEnter)
	drain(t, written, cmd)

	// Assert
	_, err := os.Stat(run.where.Path(setup.Home))
	if err != nil {
		t.Errorf("nothing written to the home directory: %v", err)
	}
}

func TestSetUpEscGoesBackAQuestion(t *testing.T) {
	t.Parallel()

	// Arrange
	atURL := typing(t, newFirstRun(t, http.StatusOK).model(t, false), keyEnter, keyEnter)

	// Act
	back := typing(t, atURL, keyEsc)

	// Assert
	requireScreen(t, back.View().Content, "Where should the file go?")
}

func TestSetUpUnderDryRunWritesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	answered := typing(t, run.model(t, true), keyEnter, keyEnter, keyEnter, keyEnter)

	// Act
	held := typing(t, answered, keyEnter)

	// Assert
	_, err := os.Stat(run.where.Path(setup.Repository))
	if !os.IsNotExist(err) || held.Destination().Dir != "" {
		t.Errorf("a dry run wrote the file (%v) or reopened (%q)", err, held.Destination().Dir)
	}

	requireScreen(t, held.View().Content, "dry run: would write")
}

func TestSettingsWithNoFileOpensTheSetUp(t *testing.T) {
	t.Parallel()

	// Act
	opened := typing(t, newFirstRun(t, http.StatusOK).model(t, false), reposKey, "S")

	// Assert
	requireScreen(t, opened.View().Content, "Set up workflow", "Where should the file go?")
}

func TestSetUpInARepositorySaysToIgnoreTheFile(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)

	err := exec.CommandContext(t.Context(), "git", "init", "-q", run.where.WorkDir).Run()
	if err != nil {
		t.Fatalf("making the repository: %v", err)
	}

	answered := typing(t, run.model(t, false), throughEveryQuestion("up")...)
	written, cmd := pressed(t, answered, keyEnter)
	written = drain(t, written, cmd)
	reopened := sized(t, tui.New(completeConfig(), nil, reposWorld().deps()), 120, 40)

	// Act
	arrived := reopened.Arrived(written.Destination())

	// Assert
	requireScreen(t, arrived.View().Content, "set up; add it to .gitignore")
}

func TestSetUpsLastLookNamesEveryAnswerButTheCredentials(t *testing.T) {
	t.Parallel()

	// Act
	asked := typing(t, newFirstRun(t, http.StatusOK).model(t, false), throughEveryQuestion("up")...)

	// Assert
	view := asked.View().Content
	requireScreen(t, view, "Write ", "Jira   "+firstRunJira, "Token  in your keychain", "Slack  a webhook")
	refuseScreen(t, view, firstRunToken, firstRunWebhook)

	if footer := footerLine(view); !strings.Contains(footer, "enter write") || !strings.Contains(footer, "esc back") {
		t.Errorf("the footer = %q, want write and back", footer)
	}
}

func TestSetUpSaysTheCheckRunsWhileJiraIsAsked(t *testing.T) {
	t.Parallel()

	// Arrange
	keys := toTheToken()
	typed := typing(t, newFirstRun(t, http.StatusOK).model(t, false), keys[:len(keys)-1]...)

	// Act
	asking, _ := pressed(t, typed, keyEnter)

	// Assert
	requireScreen(t, asking.View().Content, "checking the token with Jira")
}

func TestSetUpTypesAPasteIntoTheField(t *testing.T) {
	t.Parallel()

	// Arrange
	atURL := typing(t, newFirstRun(t, http.StatusOK).model(t, false), keyEnter, keyEnter)

	// Act
	updated, _ := atURL.Update(tea.PasteMsg{Content: firstRunJira})

	// Assert
	requireScreen(t, concrete(t, updated).View().Content, "> "+firstRunJira)
}

func TestSetUpTypesTheTokenAgainWhenAsked(t *testing.T) {
	t.Parallel()

	// Act
	again := typing(t, newFirstRun(t, http.StatusUnauthorized).model(t, false), append(toTheToken(), keyEnter)...)

	// Assert
	requireScreen(t, again.View().Content, "Your Jira personal access token")
}

func TestSetUpKeepsAFailedCheckWhenAsked(t *testing.T) {
	t.Parallel()

	// Act
	kept := typing(t, newFirstRun(t, http.StatusUnauthorized).model(t, false), append(toTheToken(), "down", keyEnter)...)

	// Assert
	requireScreen(t, kept.View().Content, "Where should the token be kept?")
}

func TestSetUpEscWalksBackThroughTheQuestions(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys []string
		want string
	}{
		"from the write":           {keys: throughEveryQuestion("up"), want: "A Slack incoming webhook URL"},
		"from the webhook":         {keys: append(toTheToken(), keyEnter), want: "Where should the token be kept?"},
		"from the keychain":        {keys: toTheToken(), want: "Your Jira personal access token"},
		"from a check":             {keys: toTheToken(), want: "Your Jira personal access token"},
		"with Jira left out":       {keys: []string{keyEnter, keyEnter, keyEnter}, want: "Jira's address"},
		"from the token":           {keys: []string{keyEnter, keyEnter, "a", keyEnter}, want: "Jira's address"},
		"cancels at the first one": {keys: []string{keyEnter}, want: "enter sets one up here."},
	}

	for name, each := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			status := http.StatusOK
			if name == "from a check" {
				status = http.StatusUnauthorized
			}

			asked := typing(t, newFirstRun(t, status).model(t, false), each.keys...)

			// Act
			back := typing(t, asked, keyEsc)

			// Assert
			requireScreen(t, back.View().Content, each.want)
		})
	}
}

func TestSetUpSaysWhyAWriteWasRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	answered := typing(t, run.model(t, false), throughEveryQuestion("up")...)

	err := os.WriteFile(run.where.Path(setup.Repository), []byte(`{}`), config.FileMode)
	if err != nil {
		t.Fatalf("writing a file meanwhile: %v", err)
	}

	// Act
	refused, cmd := pressed(t, answered, keyEnter)
	refused = drain(t, refused, cmd)

	// Assert
	requireScreen(t, refused.View().Content, "already exists")
}

func TestTheNoFileScreenNamesConfigInitWhereSetUpIsNotWired(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	working.issues = nil
	model := sized(t, tui.New(config.Default(), config.ErrNotFound, working.deps()), 120, 40)

	// Act
	view := drain(t, model, model.Init()).View().Content

	// Assert
	requireScreen(t, view, config.InitStep)
	refuseScreen(t, view, "sets one up here")
}

func TestSetUpWithoutAHomeDirectoryOffersTheRepositoryAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	run.where.HomeDir = ""

	// Act
	asked := typing(t, run.model(t, false), keyEnter)

	// Assert
	view := asked.View().Content
	requireScreen(t, view, "Where should the file go?", "this repository")

	if rows := strings.Count(view, "your home directory"); rows != 1 {
		t.Errorf("the screen names your home directory %d times, want only in the hint:\n%s", rows, view)
	}
}

func TestSetUpNeverOffersToKeepAnAddressThatIsNotOne(t *testing.T) {
	t.Parallel()

	// Arrange
	keys := []string{keyEnter, keyEnter}
	keys = append(append(keys, letters("https://fred:hunter2@jira.example.com")...), keyEnter)
	keys = append(append(keys, letters(firstRunToken)...), keyEnter)

	// Act
	asked := typing(t, newFirstRun(t, http.StatusOK).model(t, false), keys...)

	// Assert
	view := asked.View().Content
	requireScreen(t, view, "The Jira check did not pass", "Type the address again", "Leave Jira out")
	refuseScreen(t, view, "Keep them anyway")
}

func TestSetUpTypesTheAddressAgainWhenItIsNotOne(t *testing.T) {
	t.Parallel()

	// Arrange
	keys := []string{keyEnter, keyEnter}
	keys = append(append(keys, letters("jira.example.com")...), keyEnter)
	keys = append(append(keys, letters(firstRunToken)...), keyEnter, keyEnter)

	// Act
	again := typing(t, newFirstRun(t, http.StatusOK).model(t, false), keys...)

	// Assert
	requireScreen(t, again.View().Content, "Jira's address, such as")
}
