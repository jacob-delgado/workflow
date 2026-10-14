// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// errRefusedWhereWritten is why the fake's save refuses a setting as the file
// is written, after Settings' own check let it through.
var errRefusedWhereWritten = errors.New("refused where the file is written")

// The rows of messaging's client ID and webhook URL.
const (
	clientIDRow   = messagingKindRow + 1
	webhookURLRow = messagingKindRow + 4
)

// The labels of the rows the refusal tests read under.
const (
	projectLabel  = "Project"
	templateLabel = "Name template"
)

// The words a save refused for the Settings tests' values says of each.
const (
	templateRefused = "invalid branch template: must contain {key}"
	timeoutRefused  = "invalid timing: not a duration"
)

// templateAndTimeoutRefused opens Settings, types a branch template without
// {key} and a request timeout that is no duration, and saves from the
// timeout's row.
func templateAndTimeoutRefused() []string {
	keys := append(edited(branchTemplateRow, "no-key"), slices.Repeat([]string{"j"},
		requestTimeoutRow-branchTemplateRow)...)

	return append(keys, keyEnter, keyClearLine, pasted("soon"), saveKey, saveKey)
}

// fileRefusing is a save's refusal of each setting key names, as the file is
// written.
func fileRefusing(keys ...string) error {
	refusals := make([]error, 0, len(keys))
	for _, key := range keys {
		refusals = append(refusals, config.FieldError{Key: key, Err: errRefusedWhereWritten})
	}

	return fmt.Errorf("%w: %w", config.ErrInvalid, errors.Join(refusals...))
}

// styledLineUnder is the line of the screen under the first that shows text,
// as drawn, color escapes and all, failing the test when no line but the last
// does.
func styledLineUnder(t *testing.T, view, text string) string {
	t.Helper()

	lines := strings.Split(view, "\n")
	for index, line := range lines[:len(lines)-1] {
		if strings.Contains(plain(line), text) {
			return lines[index+1]
		}
	}

	t.Fatalf("no line of the screen but the last shows %q:\n%s", text, plain(view))

	return ""
}

// lineUnder is the line of the screen under the first that shows text, its
// color escapes stripped, failing the test when no line but the last does.
func lineUnder(t *testing.T, view, text string) string {
	t.Helper()

	return plain(styledLineUnder(t, view, text))
}

func TestARefusedSettingSaysWhyUnderItsOwnRow(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 80, 24), editing(branchTemplateRow, "no-key")...).View().Content

	// Assert
	if under := lineUnder(t, view, templateLabel); !strings.Contains(under, "✗ "+templateRefused) {
		t.Errorf("under the template's row is %q, want why it was refused:\n%s", under, plain(view))
	}

	requireScreen(t, view, `"no-key"`)
	refuseScreen(t, view, "the configuration is not valid")
}

func TestARefusedSaveMovesToTheFirstSettingItRefuses(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 80, 24), templateAndTimeoutRefused()...).View().Content

	// Assert
	if row := lineShowing(t, view, templateLabel); !strings.Contains(row, "▸ "+templateLabel) {
		t.Errorf("the template's row is %q, want the cursor on it", row)
	}

	if under := lineUnder(t, view, templateLabel); !strings.Contains(under, templateRefused) {
		t.Errorf("under the template's row is %q, want why it was refused:\n%s", under, plain(view))
	}
}

func TestASaveTheFileRefusesMarksEachSettingItNames(t *testing.T) {
	t.Parallel()

	twoViews := []config.JiraView{{Name: "Incidents", JQL: "type = Incident"}, {Name: "On call", JQL: "labels = oncall"}}
	cases := map[string]struct {
		key, row string
		views    []config.JiraView
	}{
		"a setting":                        {key: "jira.project", row: projectLabel},
		"an empty list, on its add row":    {key: "jira.views", row: "Add a view"},
		"a list, under its first row only": {key: "jira.views", row: "View Incidents", views: twoViews},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := newWorld()
			repo.settings.Jira.Views = tt.views
			repo.saveSettingsErr = fileRefusing(tt.key)

			// Act
			view := typing(t, repo.live(t, 80, 24), editing(projectRow, editedProject)...).View().Content

			// Assert
			reason := "✗ " + errRefusedWhereWritten.Error()
			if under := lineUnder(t, view, tt.row); !strings.Contains(under, reason) {
				t.Errorf("under the %s row is %q, want why it was refused:\n%s", tt.row, under, plain(view))
			}

			if said := strings.Count(plain(view), reason); said != 1 {
				t.Errorf("the screen says why %d times, want once:\n%s", said, plain(view))
			}

			refuseScreen(t, view, "invalid "+config.FileName)
		})
	}
}

func TestARefusalOfASettingNoRowShowsIsAlsoSaidWhole(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.saveSettingsErr = fileRefusing("jira.project", "messaging.expires_at")

	// Act
	view := typing(t, repo.live(t, 80, 24), editing(projectRow, editedProject)...).View().Content

	// Assert
	if under := lineUnder(t, view, projectLabel); !strings.Contains(under, errRefusedWhereWritten.Error()) {
		t.Errorf("under the project's row is %q, want why it was refused:\n%s", under, plain(view))
	}

	requireScreen(t, view, "invalid "+config.FileName)
}

func TestASaveHeldBackLeavesWhatTheWriteRefusedMarked(t *testing.T) {
	t.Parallel()

	// Each writes the file, which refuses the project, and leaves the cursor
	// on the project's row.
	cases := map[string][]string{
		"a save's write":    editing(projectRow, editedProject),
		"a removal's write": append(toRow(tokenRow), removeKey, keyEnter),
	}

	for name, refusing := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := newWorld()
			repo.saveSettingsErr = fileRefusing("jira.project")
			// Tall enough to show every row, wherever the cursor is.
			refusedWrite := typing(t, repo.live(t, 120, 80), refusing...)
			toTemplate := slices.Repeat([]string{"j"}, branchTemplateRow-projectRow)

			// Act
			view := typing(t, refusedWrite,
				append(toTemplate, keyEnter, keyClearLine, pasted("no-key"), saveKey, saveKey)...).View().Content

			// Assert
			if under := lineUnder(t, view, projectLabel); !strings.Contains(under, "✗ "+errRefusedWhereWritten.Error()) {
				t.Errorf("under the project's row is %q, want the write's refusal kept:\n%s", under, plain(view))
			}

			if under := lineUnder(t, view, templateLabel); !strings.Contains(under, "✗ "+templateRefused) {
				t.Errorf("under the template's row is %q, want why the save was held back:\n%s", under, plain(view))
			}

			if row := lineShowing(t, view, templateLabel); !strings.Contains(row, "▸ "+templateLabel) {
				t.Errorf("the template's row is %q, want the cursor on what the save was held back for", row)
			}
		})
	}
}

func TestASaveHeldBackJudgesWhatTheCheckRefusedBeforeAfresh(t *testing.T) {
	t.Parallel()

	// Arrange
	// A user token and a webhook URL together refuse the webhook URL; taking
	// the client ID away settles it without editing the webhook URL's row.
	repo := newWorld()
	repo.settings.Messaging.WebhookURL = "https://hooks.example.com/services/T000/B000/XXXX"

	keys := append(editing(clientIDRow, "123.456"), slices.Repeat([]string{"k"}, webhookURLRow-clientIDRow)...)
	keys = append(append(keys, keyEnter, keyClearLine, saveKey),
		slices.Repeat([]string{"j"}, branchTemplateRow-clientIDRow)...)
	settled := typing(t, repo.live(t, 120, 80), keys...)

	// Act
	view := typing(t, settled, keyEnter, keyClearLine, pasted("no-key"), saveKey, saveKey).View().Content

	// Assert
	if under := lineUnder(t, view, "Webhook URL"); strings.Contains(under, "not both") {
		t.Errorf("under the webhook URL's row is %q, want the check's earlier refusal judged again", under)
	}

	if under := lineUnder(t, view, templateLabel); !strings.Contains(under, "✗ "+templateRefused) {
		t.Errorf("under the template's row is %q, want why the save was held back:\n%s", under, plain(view))
	}
}

func TestARefusalStaysAsTheCursorMoves(t *testing.T) {
	t.Parallel()

	// Arrange
	refused := typing(t, newWorld().live(t, 80, 24), editing(branchTemplateRow, "no-key")...)

	// Act
	view := typing(t, refused, "j").View().Content

	// Assert
	if row := lineShowing(t, view, "Default prefix"); !strings.Contains(row, "▸ Default prefix") {
		t.Errorf("the row after the template's is %q, want the cursor moved onto it", row)
	}

	if under := lineUnder(t, view, templateLabel); !strings.Contains(under, templateRefused) {
		t.Errorf("under the template's row is %q, want its refusal kept:\n%s", under, plain(view))
	}
}

func TestASaveSentTakesEveryMarkAwayWhileTheWriteJudgesIt(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.saveSettingsErr = fileRefusing("jira.project")
	refused := typing(t, repo.live(t, 80, 24), editing(projectRow, editedProject)...)

	// Act
	// The write's command is left unrun, so the save is still in flight.
	sending, _ := pressed(t, refused, saveKey)

	// Assert
	view := sending.View().Content
	requireScreen(t, view, "saving")
	refuseScreen(t, view, errRefusedWhereWritten.Error())
}

func TestEditingARefusedSettingTakesAwayItsMarkAlone(t *testing.T) {
	t.Parallel()

	written := errRefusedWhereWritten.Error()
	cases := map[string]struct {
		saveErr            error
		refusing           []string
		typed              string
		editedRow, refused string
		keptRow, kept      string
	}{
		"one Settings' own check refused": {
			refusing: templateAndTimeoutRefused(), typed: "{key}",
			editedRow: templateLabel, refused: templateRefused, keptRow: "Request timeout", kept: timeoutRefused,
		},
		"one the write refused": {
			saveErr: fileRefusing("jira.project", "branch.template"), refusing: editing(projectRow, editedProject),
			typed: "ABC", editedRow: projectLabel, refused: written, keptRow: templateLabel, kept: written,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := newWorld()
			repo.saveSettingsErr = tt.saveErr
			// Tall enough to show every row, wherever the cursor is.
			refused := typing(t, repo.live(t, 120, 80), tt.refusing...)

			// Act
			view := typing(t, refused, keyEnter, keyClearLine, pasted(tt.typed), saveKey).View().Content

			// Assert
			if under := lineUnder(t, view, tt.editedRow); strings.Contains(under, tt.refused) {
				t.Errorf("under the edited %s row is %q, want its refusal taken away", tt.editedRow, under)
			}

			if under := lineUnder(t, view, tt.keptRow); !strings.Contains(under, "✗ "+tt.kept) {
				t.Errorf("under the %s row is %q, want its refusal kept:\n%s", tt.keptRow, under, plain(view))
			}
		})
	}
}

func TestARefusedSettingIsMarkedInASCII(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, asciiInterface(t, newWorld(), 80, 24), editing(branchTemplateRow, "no-key")...).View().Content

	// Assert
	if under := lineUnder(t, view, templateLabel); !strings.Contains(under, "x "+templateRefused) {
		t.Errorf("under the template's row is %q, want why it was refused behind the ASCII mark:\n%s",
			under, plain(view))
	}
}

func TestARefusedSettingsReasonWearsTheFailureStyle(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 80, 24), editing(branchTemplateRow, "no-key")...).View().Content

	// Assert
	if under := styledLineUnder(t, view, templateLabel); !strings.Contains(under, redOpen()+failGlyph) {
		t.Errorf("under the template's row is %q, want why it was refused after a red %q", under, failGlyph)
	}
}

func TestARefusedWriteMovesToTheFirstSettingItRefuses(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.saveSettingsErr = fileRefusing("branch.template")

	// Act
	view := typing(t, repo.live(t, 80, 24), editing(projectRow, editedProject)...).View().Content

	// Assert
	if under := lineUnder(t, view, templateLabel); !strings.Contains(under, "✗ "+errRefusedWhereWritten.Error()) {
		t.Errorf("under the template's row is %q, want why the write refused it:\n%s", under, plain(view))
	}
}

// writeRefusingTheWebhookURL is a world holding a webhook URL, whose file
// refuses it as it is written.
func writeRefusingTheWebhookURL() *world {
	repo := newWorld()
	repo.settings.Messaging.WebhookURL = "https://hooks.example.com/services/T000/B000/XXXX"
	repo.saveSettingsErr = fileRefusing("messaging.webhook_url")

	return repo
}

// clientIDBesideARefusedWebhookURL saves an edited project, which the write
// refuses for the webhook URL, then types a client ID, which Settings' own
// check refuses a webhook URL beside, and keeps it unsaved.
func clientIDBesideARefusedWebhookURL() []string {
	keys := append(editing(projectRow, editedProject), slices.Repeat([]string{"k"}, webhookURLRow-clientIDRow)...)

	return append(keys, keyEnter, keyClearLine, pasted("123.456"), saveKey)
}

func TestTheWritesReasonForASettingIsSaidOverTheChecks(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := writeRefusingTheWebhookURL()
	refused := typing(t, repo.live(t, 120, 80), clientIDBesideARefusedWebhookURL()...)

	// Act
	view := typing(t, refused, saveKey).View().Content

	// Assert
	if under := lineUnder(t, view, "Webhook URL"); !strings.Contains(under, "✗ "+errRefusedWhereWritten.Error()) {
		t.Errorf("under the webhook URL's row is %q, want why the write refused it:\n%s", under, plain(view))
	}

	refuseScreen(t, view, "not both")

	if row := lineShowing(t, view, "Webhook URL"); !strings.Contains(row, "▸ Webhook URL") {
		t.Errorf("the webhook URL's row is %q, want the cursor moved onto it from the client ID's", row)
	}

	if saves := repo.asked("save-settings"); len(saves) != 1 {
		t.Errorf("saved %d times, want the second save held back by Settings' own check", len(saves))
	}
}

func TestASaveHeldBackMovesToWhatTheWriteHadNotRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	toTemplate := slices.Repeat([]string{"j"}, branchTemplateRow-clientIDRow)
	refused := typing(t, writeRefusingTheWebhookURL().live(t, 120, 80),
		append(clientIDBesideARefusedWebhookURL(), toTemplate...)...)

	// Act
	view := typing(t, refused, keyEnter, keyClearLine, pasted("no-key"), saveKey, saveKey).View().Content

	// Assert
	if row := lineShowing(t, view, templateLabel); !strings.Contains(row, "▸ "+templateLabel) {
		t.Errorf("the template's row is %q, want the cursor on it, past the webhook URL the write refused:\n%s",
			row, plain(view))
	}

	if under := lineUnder(t, view, templateLabel); !strings.Contains(under, "✗ "+templateRefused) {
		t.Errorf("under the template's row is %q, want why the save was held back:\n%s", under, plain(view))
	}
}

func TestASaveHeldBackForNoRowMovesToTheFirstRefusalStanding(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.saveSettingsErr = fileRefusing("jira.project")
	toCommentKey := slices.Repeat([]string{"j"}, commentKeyRow(t)-projectRow)
	refusedWrite := typing(t, repo.live(t, 80, 24), append(editing(projectRow, editedProject), toCommentKey...)...)

	// Act
	// The comment key on t, change-status's, is a keymap no row stands for.
	view := typing(t, refusedWrite, keyEnter, keyClearLine, pasted("t"), saveKey, saveKey).View().Content

	// Assert
	if row := lineShowing(t, view, projectLabel); !strings.Contains(row, "▸ "+projectLabel) {
		t.Errorf("the project's row is %q, want the cursor on the refusal still standing:\n%s", row, plain(view))
	}
}
