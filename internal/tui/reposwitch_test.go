// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// quits reports a command that ends the program.
func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}

	_, isQuit := cmd().(tea.QuitMsg)

	return isQuit
}

// onWeb is the Repositories pane opened with the cursor on web, the first
// favorite after where you work.
func onWeb(t *testing.T, working *world) tui.Model {
	t.Helper()

	return typing(t, working.live(t, 120, 40), reposKey, "j")
}

func TestConfirmingTheSwitchToAFavoriteLeavesForIt(t *testing.T) {
	t.Parallel()

	// Arrange
	asked := typing(t, onWeb(t, reposWorld()), keyEnter)

	// Act
	left, cmd := pressed(t, asked, keyEnter)

	// Assert
	// The interface ends, and says where to start the next one: the command
	// line opens it there, wired to that directory.
	if !quits(cmd) || left.Destination().Dir != webRoot {
		t.Errorf("enter on web: quit %v, destination %q; want the program ended for %s",
			quits(cmd), left.Destination().Dir, webRoot)
	}
}

func TestEnterWhereYouWorkGoesNowhere(t *testing.T) {
	t.Parallel()

	// Arrange
	opened := typing(t, reposWorld().live(t, 120, 40), reposKey)

	// Act
	stayed, cmd := pressed(t, opened, keyEnter)

	// Assert
	if quits(cmd) || stayed.Destination().Dir != "" {
		t.Errorf("enter where you work left for %q", stayed.Destination().Dir)
	}

	requireScreen(t, stayed.View().Content, "you already work in ~/src/api/cmd")
}

func TestEnterOnAFavoriteNoLongerThereGoesNowhere(t *testing.T) {
	t.Parallel()

	// Arrange
	opened := typing(t, reposWorld().live(t, 120, 40), reposKey, "j", "j")

	// Act
	stayed, cmd := pressed(t, opened, keyEnter)

	// Assert
	if quits(cmd) || stayed.Destination().Dir != "" {
		t.Errorf("enter on a directory not there left for %q", stayed.Destination().Dir)
	}

	requireScreen(t, stayed.View().Content, "~/old is not there")
}

func TestLeavingAsksFirstWhenACommitMessageWouldBeLost(t *testing.T) {
	t.Parallel()

	// Arrange
	// The commit message is kept for this repository's branch; a switch ends
	// the session that keeps it.
	working := reposWorld()
	drafted := typing(t, working.live(t, 120, 40), append(append([]string{"3", "c"}, letters("wip")...), keyEsc)...)
	onWeb := typing(t, drafted, reposKey, "j")

	// Act
	asked, cmd := pressed(t, onWeb, keyEnter)

	// Assert
	if quits(cmd) {
		t.Fatal("left at once, want to be asked first")
	}

	requireScreen(t, asked.View().Content, "Switch to ~/src/web", "the commit message you were writing")
}

func TestAskedFirstEnterLeavesAnyway(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	drafted := typing(t, working.live(t, 120, 40), append(append([]string{"3", "c"}, letters("wip")...), keyEsc)...)
	asked := typing(t, drafted, reposKey, "j", keyEnter)

	// Act
	left, cmd := pressed(t, asked, keyEnter)

	// Assert
	if !quits(cmd) || left.Destination().Dir != webRoot {
		t.Errorf("enter on the question: quit %v, destination %q; want %s", quits(cmd), left.Destination().Dir, webRoot)
	}
}

func TestAskedFirstEscStays(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	drafted := typing(t, working.live(t, 120, 40), append(append([]string{"3", "c"}, letters("wip")...), keyEsc)...)
	asked := typing(t, drafted, reposKey, "j", keyEnter)

	// Act
	stayed, cmd := pressed(t, asked, keyEsc)

	// Assert
	if quits(cmd) || stayed.Destination().Dir != "" {
		t.Errorf("esc on the question left for %q", stayed.Destination().Dir)
	}
}

func TestArrivingSaysWhereAndKeepsTheSessionsChoices(t *testing.T) {
	t.Parallel()

	// Arrange
	// The Summary's period is the session's, not the repository's.
	working := reposWorld()
	moved := typing(t, working.live(t, 120, 40), summaryKey, "[", reposKey, "j", keyEnter)
	left, _ := pressed(t, moved, keyEnter)
	arriving := newWorld()
	arriving.dirs = &dirsWorld{here: webPlace()}
	arriving.done = summaryWorld().done

	// Act
	arrived := typing(t, arriving.live(t, 120, 40).Arrived(left.Destination()), summaryKey)

	// Assert
	requireScreen(t, arrived.View().Content, "switched to ~/src/web", "2026-09-14")
}

func TestAFailedSwitchStaysAndSaysWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	left, _ := pressed(t, typing(t, onWeb(t, reposWorld()), keyEnter), keyEnter)

	// Act
	stayed := reposWorld().live(t, 120, 40).StayedAfter(left.Destination(), errJiraDown)

	// Assert
	requireScreen(t, stayed.View().Content, "could not switch to ~/src/web", "jira is down")
}

// pressedAndAnswered presses key and delivers the answer of the command it
// started, as Bubble Tea does once the work behind it returns.
func pressedAndAnswered(t *testing.T, model tui.Model, key string) (tui.Model, tea.Cmd) {
	t.Helper()

	asked, cmd := pressed(t, model, key)

	return finish(t, asked, cmd)
}

// goingTo is the Repositories pane with the go-to prompt open and path typed
// into it.
func goingTo(t *testing.T, working *world, path string) tui.Model {
	t.Helper()

	return typing(t, working.live(t, 120, 40), append([]string{reposKey, "g"}, letters(path)...)...)
}

func TestGoToLeavesForTheDirectoryTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	// A path is read from where you work, or your home after a ~.
	typed := goingTo(t, reposWorld(), "~/src/web")
	asked, _ := pressedAndAnswered(t, typed, keyEnter)

	// Act
	left, cmd := pressed(t, asked, keyEnter)

	// Assert
	if !quits(cmd) || left.Destination().Dir != webRoot {
		t.Errorf("go to ~/src/web: quit %v, destination %q; want %s", quits(cmd), left.Destination().Dir, webRoot)
	}
}

func TestGoToADirectoryNotThereSaysSoAndStaysOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	typed := goingTo(t, reposWorld(), "../../nowhere")

	// Act
	answered, cmd := pressedAndAnswered(t, typed, keyEnter)

	// Assert
	if quits(cmd) || answered.Destination().Dir != "" {
		t.Fatalf("left for %q, want to stay", answered.Destination().Dir)
	}

	requireScreen(t, answered.View().Content, "Go to a directory", "there is no such directory")
}

func TestTabCompletesTheOneDirectoryThatFits(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	working.dirs.listings = map[string][]string{"/home/ana/src": {"cli", "www"}}
	typed := goingTo(t, working, "~/src/w")

	// Act
	completed, _ := pressedAndAnswered(t, typed, keyTab)

	// Assert
	requireScreen(t, completed.View().Content, "> ~/src/www/")
}

func TestTabNamesTheDirectoriesThatFitWhenThereAreSeveral(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	working.dirs.listings = map[string][]string{anaHome: {"notes", "notebooks", "srv"}}
	typed := goingTo(t, working, "~/n")

	// Act
	completed, _ := pressedAndAnswered(t, typed, keyTab)

	// Assert
	// What they share is typed, and each is named, so the next letter picks.
	requireScreen(t, completed.View().Content, "> ~/note", "notebooks", "notes")
}

func TestTabSaysWhenNothingFits(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	working.dirs.listings = map[string][]string{anaHome: {"src"}}
	typed := goingTo(t, working, "~/z")

	// Act
	completed, _ := pressedAndAnswered(t, typed, keyTab)

	// Assert
	requireScreen(t, completed.View().Content, "no directory there starts with z")
}

func TestTheGoToPromptTakesEveryLetter(t *testing.T) {
	t.Parallel()

	// Act
	// q quits and j moves everywhere else; here they are a path's letters.
	typed := goingTo(t, reposWorld(), "jq")

	// Assert
	requireScreen(t, typed.View().Content, "> jq")
}

func TestTabTypesWhatSeveralShareWithoutSplittingALetter(t *testing.T) {
	t.Parallel()

	// Arrange
	// v1é and v1è share v1 and the first byte of their last letters.
	working := reposWorld()
	working.dirs.listings = map[string][]string{anaHome: {"v1é", "v1è"}}
	typed := goingTo(t, working, "~/v")

	// Act
	completed, _ := pressedAndAnswered(t, typed, keyTab)

	// Assert
	requireScreen(t, completed.View().Content, "> ~/v1 ", "v1é", "v1è")
	refuseScreen(t, completed.View().Content, "�")
}

func TestEscLeavesWhileAPathIsBeingLookedAt(t *testing.T) {
	t.Parallel()

	// Arrange
	// A directory on a mount that does not answer is not waited on.
	typed := goingTo(t, reposWorld(), "~/src/web")
	looking, look := pressed(t, typed, keyEnter)
	closed, _ := pressed(t, looking, keyEsc)

	// Act
	answered, cmd := finish(t, closed, look)

	// Assert
	if quits(cmd) || answered.Destination().Dir != "" {
		t.Errorf("an answer after esc left for %q", answered.Destination().Dir)
	}

	refuseScreen(t, answered.View().Content, "Go to a directory")
}

func TestGoToSaysWhenADirectoryCannotBeLookedAt(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	working.dirs.cannotLook = true
	typed := goingTo(t, working, "~/src/web")

	// Act
	answered, _ := pressed(t, typed, keyEnter)

	// Assert
	requireScreen(t, answered.View().Content, "cannot look at directories here")
}

func TestACommentBeingWrittenGoesWithASwitchOnlyToTheSameJira(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		jira string
		kept bool
	}{
		"the same Jira":    {jira: "", kept: true},
		"another instance": {jira: "https://jira.other.example", kept: false},
	}

	for name, arriving := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// A Jira key names an issue on one instance; on another it is
			// someone else's issue.
			working := reposWorld()
			drafted := typing(t, working.live(t, 120, 40),
				append(append([]string{"c", "i"}, letters("half done")...), keyEsc, keyEsc)...)
			left, _ := pressed(t, typing(t, drafted, reposKey, "j", keyEnter), keyEnter)

			other := newWorld()
			if arriving.jira != "" {
				other.cfg.Jira.BaseURL = arriving.jira
			}

			// Act
			arrived := typing(t, other.live(t, 120, 40).Arrived(left.Destination()), "1", "c")

			// Assert
			if shown := strings.Contains(arrived.View().Content, "half done"); shown != arriving.kept {
				t.Errorf("the draft shown: %v, want %v", shown, arriving.kept)
			}
		})
	}
}

func TestLeavingNamesACommentOnAForgeIssueAsLost(t *testing.T) {
	t.Parallel()

	// Arrange
	// A forge issue's number names an issue in the repository left.
	working := reposWorld()
	working.issues = []jira.Issue{{Key: "42", Summary: "the forge's issue", StatusCategory: categoryNew}}
	drafted := typing(t, working.live(t, 120, 40),
		append(append([]string{"c", "i"}, letters("half done")...), keyEsc, keyEsc)...)
	onWeb := typing(t, drafted, reposKey, "j")

	// Act
	asked, _ := pressed(t, onWeb, keyEnter)

	// Assert
	requireScreen(t, asked.View().Content, "your comment on #42")
}

func TestGoToWhereYouWorkGoesNowhere(t *testing.T) {
	t.Parallel()

	// Arrange
	typed := goingTo(t, reposWorld(), ".")

	// Act
	answered, cmd := pressedAndAnswered(t, typed, keyEnter)

	// Assert
	if quits(cmd) || answered.Destination().Dir != "" {
		t.Errorf("going where you work left for %q", answered.Destination().Dir)
	}

	requireScreen(t, answered.View().Content, "you already work in ~/src/api/cmd")
}

func TestTabAfterATildeAloneListsYourHome(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	working.dirs.listings = map[string][]string{anaHome: {"src"}}
	typed := goingTo(t, working, "~")

	// Act
	completed, _ := pressedAndAnswered(t, typed, keyTab)

	// Assert
	requireScreen(t, completed.View().Content, "> ~/src/")
}

func TestTabWhereThereIsNoSuchDirectorySaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	typed := goingTo(t, reposWorld(), "~/nowhere/x")

	// Act
	completed, _ := pressedAndAnswered(t, typed, keyTab)

	// Assert
	requireScreen(t, completed.View().Content, "there is no such directory")
}

func TestAnAnswerForAPathSinceChangedIsDropped(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	working.dirs.listings = map[string][]string{"/home/ana/src": {"web"}}
	typed := goingTo(t, working, "~/src/w")
	asked, complete := pressed(t, typed, keyTab)
	retyped := typing(t, asked, "x")

	// Act
	answered, _ := finish(t, retyped, complete)

	// Assert
	requireScreen(t, answered.View().Content, "> ~/src/wx")
	refuseScreen(t, answered.View().Content, "~/src/web/")
}
