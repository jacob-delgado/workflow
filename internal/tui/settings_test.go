// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// The keys that open Settings from the Repositories pane and save it.
const (
	settingsKey = "S"
	saveKey     = "ctrl+s"
)

// The rows of Settings the tests edit, counted from the first, Jira's base URL.
const (
	tokenRow          = 1
	projectRow        = 3
	subjectLimitRow   = 23
	branchTemplateRow = 25
	storeDisabledRow  = 30
)

// editedProject is the Jira project a test types in place of the one read.
const editedProject = "OSS"

// typedSecret is a credential typed into Settings.
const typedSecret = "typed-secret-4242"

// toRow opens Settings and moves down to row.
func toRow(row int) []string {
	return append([]string{reposKey, settingsKey}, slices.Repeat([]string{"j"}, row)...)
}

// edited opens Settings and edits row to text, unsaved.
func edited(row int, text string) []string {
	keys := append(toRow(row), keyEnter, "ctrl+u")

	return append(append(keys, letters(text)...), keyEnter)
}

// editing opens Settings, edits row to text and saves.
func editing(row int, text string) []string {
	return append(edited(row, text), saveKey)
}

func TestSettingsShowsTheConfigurationWithItsCredentialsMasked(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), reposKey, settingsKey).View().Content

	// Assert
	requireScreen(t, view, "Settings", "Base URL", "https://jira.example.com", "Project", "PROJ", "****9999")
	refuseScreen(t, view, settingsToken)
}

func TestATypedSecretNeverReachesTheScreen(t *testing.T) {
	t.Parallel()

	// Arrange
	model := typing(t, newWorld().live(t, 120, 40), append(toRow(tokenRow), keyEnter)...)

	// Act: type the token
	typed := typing(t, model, letters(typedSecret)...)

	// Assert: it is not echoed
	refuseScreen(t, typed.View().Content, typedSecret)

	// Act: keep it
	kept := typing(t, typed, keyEnter)

	// Assert: the row does not show it either
	refuseScreen(t, kept.View().Content, typedSecret)
}

func TestSavingSendsTheEditAndLeavesTheStoredTokenMasked(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), editing(projectRow, editedProject)...).View().Content

	// Assert
	if saves := repo.asked("save-settings"); len(saves) != 1 || repo.settings.Jira.Project != editedProject {
		t.Fatalf("saved %d times, project %q; want OSS saved once", len(saves), repo.settings.Jira.Project)
	}

	if token := repo.settings.Jira.Token.Reveal(); token != config.Redact(settingsToken) {
		t.Errorf("saved the token as %q, want it sent back masked so the stored one is kept", token)
	}

	requireScreen(t, view, "saved")
}

func TestSavingSendsATypedSecret(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), editing(tokenRow, typedSecret)...)

	// Assert
	if token := repo.settings.Jira.Token.Reveal(); token != typedSecret {
		t.Errorf("saved the token as %q, want the one typed", config.Redact(token))
	}
}

func TestAnInvalidSettingIsRefusedBeforeItIsSaved(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), editing(branchTemplateRow, "no-key")...).View().Content

	// Assert
	if saves := repo.asked("save-settings"); len(saves) != 0 {
		t.Errorf("saved %d times, want an invalid configuration refused", len(saves))
	}

	requireScreen(t, view, "not valid")
}

func TestACountMustBeAWholeNumber(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40),
		append(toRow(subjectLimitRow), keyEnter, "ctrl+u", "x", keyEnter)...).View().Content

	// Assert
	requireScreen(t, view, "a whole number")
}

func TestEnterOnASettingThatIsOnOrOffTurnsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), append(toRow(storeDisabledRow), keyEnter, saveKey)...)

	// Assert
	if !repo.settings.Store.Disabled {
		t.Errorf("store.disabled saved as false, want it turned on")
	}
}

func TestASaveOverAChangedFileSaysSoAndOffersAReload(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.saveSettingsErr = config.ErrChangedOnDisk

	// Act
	view := typing(t, repo.live(t, 120, 40), editing(projectRow, editedProject)...).View().Content

	// Assert
	requireScreen(t, view, "changed after Settings read it")
	requireScreen(t, footerLine(view), "r reload")
}

func TestReloadReadsTheFileAgainInPlaceOfTheEdits(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.saveSettingsErr = config.ErrChangedOnDisk
	refused := typing(t, repo.live(t, 120, 40), editing(projectRow, editedProject)...)

	// Act
	view := typing(t, refused, "r").View().Content

	// Assert
	if reads := repo.asked("read-settings"); len(reads) != 2 {
		t.Errorf("read %d times, want the file read again", len(reads))
	}

	refuseScreen(t, view, editedProject, "changed after Settings read it")
}

func TestADryRunSavesNoSettings(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	model := sized(t, tui.New(repo.cfg, nil, repo.deps()).WithDryRun(), 120, 40)

	// Act
	view := typing(t, drain(t, model, model.Init()), editing(projectRow, editedProject)...).View().Content

	// Assert
	if saves := repo.asked("save-settings"); len(saves) != 0 {
		t.Errorf("saved %d times under a dry run", len(saves))
	}

	requireScreen(t, view, "dry run: would save")
}

func TestEscWithEditsDiscardsThem(t *testing.T) {
	t.Parallel()

	// Arrange
	keys := append(toRow(projectRow), keyEnter, "ctrl+u", "O", keyEnter)

	// Act
	view := typing(t, newWorld().live(t, 120, 40), keys...).View().Content

	// Assert
	requireScreen(t, footerLine(view), "esc discard")
}

// The rows of Settings the coverage of each kind of setting edits.
const (
	baseURLRow       = 0
	markdownRow      = 7
	messagingKindRow = 9
	commitTypesRow   = 22
)

// errSettingsUnreadable is a configuration file that cannot be read.
var errSettingsUnreadable = errors.New("permission denied")

func TestSettingsThatCannotBeReadSayWhyAndOfferAnotherRead(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.readSettingsErr = errSettingsUnreadable

	// Act
	view := typing(t, repo.live(t, 120, 40), reposKey, settingsKey, "j", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "permission denied")
	requireScreen(t, footerLine(view), "r try again")
}

func TestTryingAgainReadsTheSettingsAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.readSettingsErr = errSettingsUnreadable
	refused := typing(t, repo.live(t, 120, 40), reposKey, settingsKey)
	repo.mu.Lock()
	repo.readSettingsErr = nil
	repo.mu.Unlock()

	// Act
	view := typing(t, refused, "r").View().Content

	// Assert
	requireScreen(t, view, "Base URL")
}

func TestSlackRefusingTypedSecretsSaysWhichToCheck(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.saveSettingsErr = messaging.ErrRejected

	// Act
	view := typing(t, repo.live(t, 120, 40), editing(projectRow, editedProject)...).View().Content

	// Assert
	requireScreen(t, view, "refresh token")
}

func TestARefusedSaveStaysInSettingsWithItsReason(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.saveSettingsErr = errSettingsUnreadable

	// Act
	view := typing(t, repo.live(t, 120, 40), editing(projectRow, editedProject)...).View().Content

	// Assert
	requireScreen(t, view, "permission denied", "Base URL")
	refuseScreen(t, footerLine(view), "reload")
}

func TestAChoiceMovesOnAndBack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		keys []string
		want config.MessagingKind
	}{
		{"by enter", []string{keyEnter}, config.KindTeams},
		{"forward", []string{keyRight}, config.KindTeams},
		{"back", []string{keyLeft}, config.KindWebhook},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := newWorld()

			keys := append(append(toRow(messagingKindRow), test.keys...), saveKey)

			// Act
			typing(t, repo.live(t, 120, 40), keys...)

			// Assert
			if got := repo.settings.Messaging.Kind; got != test.want {
				t.Errorf("saved messaging.kind %q, want %q", got, test.want)
			}
		})
	}
}

func TestAListIsTypedCommaSeparated(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), editing(commitTypesRow, "feat, fix,")...)

	// Assert
	if got := repo.settings.Commit.Types; !slices.Equal(got, []string{"feat", "fix"}) {
		t.Errorf("saved commit.types %q, want feat and fix", got)
	}
}

func TestAnAddressIsEditedAsTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), editing(baseURLRow, "https://jira.example.org")...)

	// Assert
	if got := repo.settings.Jira.BaseURL; got != "https://jira.example.org" {
		t.Errorf("saved jira.base_url %q, want the address typed", got)
	}
}

func TestASettingThatIsOnTurnsOff(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), append(toRow(markdownRow), "space", saveKey)...)

	// Assert
	if repo.settings.Jira.MarkdownComments {
		t.Errorf("jira.markdown_comments saved as on, want it turned off")
	}
}

func TestACredentialLeftEmptyKeepsTheStoredOne(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), editing(tokenRow, "")...)

	// Assert
	if token := repo.settings.Jira.Token.Reveal(); token != config.Redact(settingsToken) {
		t.Errorf("saved the token as %q, want the masked one sent back", config.Redact(token))
	}
}

func TestACountBelowZeroIsRefused(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40),
		append(toRow(subjectLimitRow), keyEnter, "ctrl+u", "-", "1", keyEnter)...).View().Content

	// Assert
	requireScreen(t, view, "a whole number")
}

func TestEscBacksOutOfAFieldAndKeepsTheValue(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	keys := append(toRow(projectRow), keyEnter, "ctrl+u", "O", keyEsc, saveKey)

	// Act
	typing(t, repo.live(t, 120, 40), keys...)

	// Assert
	if repo.settings.Jira.Project != readProject {
		t.Errorf("saved project %q, want the field backed out of", repo.settings.Jira.Project)
	}
}

func TestUpMovesBackAndEscClosesSettings(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), reposKey, settingsKey, "j", "j", "k", keyEsc).View().Content

	// Assert
	refuseScreen(t, view, "Saves to")
}

func TestAPasteTypesIntoTheFieldBeingEdited(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	model := typing(t, repo.live(t, 120, 40), append(toRow(projectRow), keyEnter, "ctrl+u")...)

	// Act
	typing(t, pasting(t, model, editedProject), keyEnter, saveKey)

	// Assert
	if repo.settings.Jira.Project != editedProject {
		t.Errorf("saved project %q, want the paste", repo.settings.Jira.Project)
	}
}

func TestAPasteOutsideAFieldChangesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	model := typing(t, repo.live(t, 120, 40), toRow(projectRow)...)

	// Act
	typing(t, pasting(t, model, editedProject), saveKey)

	// Assert
	if repo.settings.Jira.Project != readProject {
		t.Errorf("saved project %q, want it left as read", repo.settings.Jira.Project)
	}
}

func TestAKeymapTheInterfaceWouldRefuseIsNotSaved(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.settings.UI.Keys = map[string]string{"quit": "tab"}

	// Act
	view := typing(t, repo.live(t, 120, 40), editing(projectRow, editedProject)...).View().Content

	// Assert
	if saves := repo.asked("save-settings"); len(saves) != 0 {
		t.Errorf("saved %d times, want a refused keymap held back", len(saves))
	}

	requireScreen(t, view, "would not start on this keymap")
}

func TestTheFooterNamesWhatEnterDoesToEachKindOfSetting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		row  int
		want string
	}{
		{"on or off", markdownRow, "enter turn on or off"},
		{"a choice", messagingKindRow, "enter change"},
		{"a list", commitTypesRow, "enter edit"},
		{"a count", subjectLimitRow, "enter edit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := typing(t, newWorld().live(t, 120, 40), toRow(test.row)...).View().Content

			// Assert
			requireScreen(t, footerLine(view), test.want)
		})
	}
}

func TestAKeyASettingDoesNotTakeChangesNothing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		row  int
	}{
		{"on or off", markdownRow},
		{"a choice", messagingKindRow},
		{"text", projectRow},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := newWorld()

			// Act
			typing(t, repo.live(t, 120, 40), append(toRow(test.row), "x", "r", saveKey)...)

			// Assert
			if repo.settings.Jira.Project != readProject || !repo.settings.Jira.MarkdownComments ||
				repo.settings.Messaging.Kind != "" {
				t.Errorf("saved %+v, want nothing changed", repo.settings.Jira)
			}
		})
	}
}

func TestAChoiceMovesOnFromOneItOffers(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), append(toRow(messagingKindRow), keyEnter, keyEnter, saveKey)...)

	// Assert
	if got := repo.settings.Messaging.Kind; got != config.KindDiscord {
		t.Errorf("saved messaging.kind %q, want discord", got)
	}
}

func TestATypedListAndCountShowAsKept(t *testing.T) {
	t.Parallel()

	// Arrange
	keys := append(toRow(commitTypesRow), keyEnter, "ctrl+u")
	keys = append(append(keys, letters("feat,fix")...), keyEnter, "j", keyEnter, "ctrl+u", "5", "0", keyEnter)

	// Act
	view := typing(t, newWorld().live(t, 120, 40), keys...).View().Content

	// Assert
	requireScreen(t, view, "feat, fix", "50")
}
