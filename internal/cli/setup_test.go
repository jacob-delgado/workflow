// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// setupToken is the Jira token typed into the interface's first run.
const setupToken = "typed-in-the-interface-7731"

// commandWait is how long a test waits on one command the interface returns
// before dropping it: a timer the interface sets runs on the real clock, and
// what it would deliver is not what the test is about.
const commandWait = 2 * time.Second

// keysFor is each character of text as a key press, as a terminal sends it.
func keysFor(text string) []tea.KeyPressMsg {
	keys := make([]tea.KeyPressMsg, 0, len(text))
	for _, character := range text {
		keys = append(keys, tea.KeyPressMsg{Code: character, Text: string(character)})
	}

	return keys
}

// enter is the enter key.
func enter() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEnter}
}

// firstRunKeys open the form on the no-file screen, choose the home
// directory, the one file the keychain can keep the token for, type Jira's
// address and the token, keep it in the keychain, skip the webhook and write.
func firstRunKeys(jiraURL string) []tea.KeyPressMsg {
	keys := []tea.KeyPressMsg{enter(), {Code: tea.KeyDown}, enter()}
	keys = append(append(keys, keysFor(jiraURL)...), enter())
	keys = append(append(keys, keysFor(setupToken)...), enter())

	return append(keys, enter(), enter(), enter())
}

// drive presses keys in turn, running whatever each starts as Bubble Tea
// would, and returns the interface they leave.
func drive(t *testing.T, model tui.Model, keys ...tea.KeyPressMsg) tui.Model {
	t.Helper()

	for _, key := range keys {
		updated, cmd := model.Update(key)
		model = settle(t, asModel(t, updated), cmd)
	}

	return model
}

// settle runs cmd and every command it leads to, delivering each message.
func settle(t *testing.T, model tui.Model, cmd tea.Cmd) tui.Model {
	t.Helper()

	pending := []tea.Cmd{cmd}

	for len(pending) > 0 {
		next := pending[0]
		pending = pending[1:]

		switch msg := answerOf(next).(type) {
		case nil, tea.QuitMsg:
		case tea.BatchMsg:
			pending = append(pending, msg...)
		default:
			updated, follow := model.Update(msg)
			model = asModel(t, updated)

			pending = append(pending, follow)
		}
	}

	return model
}

// answerOf is cmd's message, or nil for no command or one that did not answer in
// time.
//
//nolint:ireturn // tea.Msg is Bubble Tea's type for any message at all
func answerOf(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}

	answer := make(chan tea.Msg, 1)

	go func() { answer <- cmd() }()

	select {
	case msg := <-answer:
		return msg
	case <-time.After(commandWait):
		return nil
	}
}

// asModel is a model Update returned, as the interface's own type.
func asModel(t *testing.T, model tea.Model) tui.Model {
	t.Helper()

	concrete, ok := model.(tui.Model)
	if !ok {
		t.Fatalf("Update returned a %T, want a tui.Model", model)
	}

	return concrete
}

// firstRunOutcome is what a first run through the interface left: every
// interface opened, in order, and what the keychain was handed, under which
// item.
type firstRunOutcome struct {
	opened  []tui.Model
	service string
	stored  string
	err     error
}

// setUpThroughTheInterface runs bare workflow where the test chose, logging
// requests to logPath, over an interface that answers the first run's form
// against the Jira at jiraURL, then quits the interface reopened.
func setUpThroughTheInterface(t *testing.T, where place, jiraURL, logPath string) *firstRunOutcome {
	t.Helper()

	outcome := &firstRunOutcome{}
	prompt := unusedPrompt(t)
	prompt.StoreSecret = func(service, secret string) error {
		outcome.service, outcome.stored = service, secret

		return nil
	}

	runInterface := func(_ context.Context, model tui.Model, _ io.Reader, _ io.Writer) (tui.Next, error) {
		outcome.opened = append(outcome.opened, model)
		if len(outcome.opened) > 1 {
			return tui.Next{}, nil
		}

		return drive(t, model, firstRunKeys(jiraURL)...).Destination(), nil
	}

	for name, value := range isolatedEnvironment(where.home) {
		t.Setenv(name, value)
	}

	t.Chdir(where.dir)

	root := cli.NewRootCmdOver(prompt, runInterface, func(string) cli.RunWeb {
		return func(context.Context, config.Config, webserver.Deps, webserver.Info, io.Writer) error { return nil }
	})
	root.SetArgs([]string{"--log=" + logPath})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	outcome.err = root.ExecuteContext(t.Context())

	return outcome
}

func TestTheInterfaceSetsUpAFirstFileAndReopensWithIt(t *testing.T) {
	// Arrange
	where := place{dir: t.TempDir(), home: t.TempDir()}
	logPath := filepath.Join(t.TempDir(), "requests.log")

	// Act
	outcome := setUpThroughTheInterface(t, where, workingJira(t), logPath)

	// Assert
	if outcome.err != nil || len(outcome.opened) != 2 {
		t.Fatalf("workflow = %v, opened %d interfaces; want it reopened once the file was written",
			outcome.err, len(outcome.opened))
	}

	reopened := ansi.Strip(outcome.opened[1].View().Content)
	if strings.Contains(reopened, config.NoConfigHeadline) || !strings.Contains(reopened, "set up with") {
		t.Errorf("the interface reopened without the file:\n%s", reopened)
	}

	contents, err := os.ReadFile(filepath.Join(where.home, config.FileName))
	if err != nil || strings.Contains(string(contents), setupToken) || outcome.stored != setupToken ||
		!strings.HasPrefix(outcome.service, "workflow-jira http") {
		t.Errorf("wrote %q (%v), keychain %q under %q; want the token in the keychain item for its address alone",
			contents, err, outcome.stored, outcome.service)
	}
}

func TestTheInterfacesFirstRunLogsTheCheckWithoutTheToken(t *testing.T) {
	// Arrange
	where := place{dir: t.TempDir(), home: t.TempDir()}
	logPath := filepath.Join(t.TempDir(), "requests.log")

	// Act
	outcome := setUpThroughTheInterface(t, where, workingJira(t), logPath)

	// Assert
	logged, err := os.ReadFile(logPath)
	if outcome.err != nil || err != nil || !strings.Contains(string(logged), "myself") ||
		strings.Contains(string(logged), setupToken) {
		t.Errorf("workflow = %v; the request log = %q (%v), want the check outlined without the token",
			outcome.err, logged, err)
	}
}
