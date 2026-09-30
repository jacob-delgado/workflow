// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// The issues placesWorld lists, each in a status a team's own Jira workflow
// might name.
const (
	intakeIssue      = "PROJ-501"
	leakIssue        = "PROJ-502"
	retryIssue       = "PROJ-503"
	searchIssue      = "PROJ-504"
	renameIssue      = "PROJ-505"
	statusIntake     = "Intake"
	statusFixing     = "Fixing"
	statusInDev      = "In development"
	searchBranch     = "fix/PROJ-504-speed-up-search"
	placeKey         = "p"
	categoryNew      = "new"
	placesViewWidth  = 120
	placesViewHeight = 40
	// wideFooterWidth leaves the footer room for the list's own keys after the
	// selected issue's verbs.
	wideFooterWidth = 220
)

// placesWorld lists five issues across three statuses: one Intake, two Fixing
// and two In development. A branch names PROJ-504, and a started task is linked
// to PROJ-503.
func placesWorld() *world {
	repo := withTasks()
	repo.issues = []jira.Issue{
		{Key: intakeIssue, Summary: "Triage the crash", Status: statusIntake, StatusCategory: categoryNew},
		{Key: leakIssue, Summary: "Patch the leak", Status: statusFixing, StatusCategory: categoryIndeterminate},
		{Key: retryIssue, Summary: "Retry uploads", Status: statusFixing, StatusCategory: categoryIndeterminate},
		{Key: searchIssue, Summary: "Speed up search", Status: statusInDev, StatusCategory: categoryIndeterminate},
		{Key: renameIssue, Summary: "Rename flags", Status: statusInDev, StatusCategory: categoryIndeterminate},
	}
	repo.branches = []string{baseName, featureName, searchBranch}
	repo.tasks.linked = []taskwarrior.Task{{
		UUID: activeTaskUUID, ID: 12, Description: retryIssue + ": Retry uploads", Status: taskwarrior.Pending,
		Start: testNow().Add(-72 * time.Minute), IssueKey: retryIssue,
	}}

	return repo
}

// listedKeys is which of placesWorld's issues the Issues rail lists.
func listedKeys(view string) []string {
	var listed []string

	for _, candidate := range []string{intakeIssue, leakIssue, retryIssue, searchIssue, renameIssue} {
		if strings.Contains(plain(view), candidate+" ") {
			listed = append(listed, candidate)
		}
	}

	return listed
}

// railLine is the rail's row for an issue key, with its color escapes stripped.
func railLine(view, issueKey string) string {
	for line := range strings.SplitSeq(plain(view), "\n") {
		if strings.Contains(line, issueKey+" ") {
			return line
		}
	}

	return ""
}

func TestPWithIssuesOpensTheWherePickerWithCounts(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := placesWorld()

	// Act
	view := typing(t, repo.live(t, placesViewWidth, placesViewHeight), placeKey).View().Content

	// Assert
	requireScreen(t, view, "Where", "Intake  1", "Fixing  2", "In development  2", "in flight  1", "task active  1")
}

func TestPickingAStatusNarrowsTheList(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := placesWorld()

	// Act
	view := typing(t, repo.live(t, placesViewWidth, placesViewHeight),
		placeKey, downAction, keySpace, keyEnter).View().Content

	// Assert
	if got := strings.Join(listedKeys(view), " "); got != leakIssue+" "+retryIssue {
		t.Errorf("listed %q, want only the Fixing issues:\n%s", got, plain(view))
	}

	requireScreen(t, view, "places: Fixing")
}

func TestStatusesOrAndMarksAnd(t *testing.T) {
	t.Parallel()

	// The picker lists Intake, Fixing, In development, in flight, task active.
	cases := map[string]struct {
		keys []string
		want []string
	}{
		"two statuses list either": {
			keys: []string{keySpace, downAction, keySpace},
			want: []string{intakeIssue, leakIssue, retryIssue},
		},
		"a status and a mark list both": {
			keys: []string{downAction, downAction, keySpace, downAction, keySpace},
			want: []string{searchIssue},
		},
		"two marks list either": {
			keys: []string{downAction, downAction, downAction, keySpace, downAction, keySpace},
			want: []string{retryIssue, searchIssue},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			keys := append(append([]string{placeKey}, tt.keys...), keyEnter)

			// Act
			view := typing(t, placesWorld().live(t, placesViewWidth, placesViewHeight), keys...).View().Content

			// Assert
			if got, want := strings.Join(listedKeys(view), " "), strings.Join(tt.want, " "); got != want {
				t.Errorf("listed %q, want %q:\n%s", got, want, plain(view))
			}
		})
	}
}

func TestPlacesAndTextFilterCombine(t *testing.T) {
	t.Parallel()

	// Arrange
	keys := append([]string{placeKey, downAction, keySpace, keyEnter, "/"}, letters("leak")...)

	// Act
	view := typing(t, placesWorld().live(t, placesViewWidth, placesViewHeight), append(keys, keyEnter)...).View().Content

	// Assert
	if got := strings.Join(listedKeys(view), " "); got != leakIssue {
		t.Errorf("listed %q, want only the Fixing issue about the leak:\n%s", got, plain(view))
	}
}

func TestEscClosesTheWherePickerWithoutApplying(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, placesWorld().live(t, placesViewWidth, placesViewHeight),
		placeKey, downAction, keySpace, keyEsc).View().Content

	// Assert
	if got := len(listedKeys(view)); got != 5 {
		t.Errorf("listed %d issues, want all five:\n%s", got, plain(view))
	}

	refuseScreen(t, view, "places:")
}

func TestAPickedPlaceWithNoIssuesStaysOfferedAtZero(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := placesWorld()
	model := typing(t, repo.live(t, placesViewWidth, placesViewHeight), placeKey, downAction, keySpace, keyEnter)
	repo.issues = repo.issues[:1]

	// Act
	view := typing(t, model, "r", placeKey).View().Content

	// Assert
	requireScreen(t, view, "Fixing  0")
}

func TestRowsShowTheStatusNameAndInFlight(t *testing.T) {
	t.Parallel()

	// Act
	view := placesWorld().live(t, placesViewWidth, placesViewHeight).View().Content

	// Assert
	requireScreen(t, view, leakIssue+" Fixing Patch")

	if line := railLine(view, searchIssue); !strings.Contains(line, "·◐ "+searchIssue) {
		t.Errorf("the branch-named issue's row is %q, want the in-flight mark before its key", line)
	}

	if line := railLine(view, renameIssue); !strings.Contains(line, "·· "+renameIssue) {
		t.Errorf("the issue without a branch reads %q, want no in-flight mark", line)
	}
}

func TestWithoutTaskwarriorNoTaskPlacesAreOffered(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := placesWorld()
	repo.tasks = nil

	// Act
	view := typing(t, repo.live(t, placesViewWidth, placesViewHeight), placeKey).View().Content

	// Assert
	requireScreen(t, view, "in flight  1")
	refuseScreen(t, view, "task active")
}

func TestAFailedBranchListingOffersNoInFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := placesWorld()
	repo.branchesErr = errListRemotes

	// Act
	view := typing(t, repo.live(t, placesViewWidth, placesViewHeight), placeKey).View().Content

	// Assert
	requireScreen(t, view, "task active  1")
	refuseScreen(t, view, "in flight")
}

func TestTheFooterOffersWhereWithIssuesLoaded(t *testing.T) {
	t.Parallel()

	// Act
	view := placesWorld().live(t, wideFooterWidth, placesViewHeight).View().Content

	// Assert
	requireScreen(t, footerLine(view), "p where")
}

func TestSwitchingViewDropsThePlaces(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := placesWorld()
	repo.cfg.Jira.Views = twoViewConfig().Jira.Views
	model := typing(t, repo.live(t, placesViewWidth, placesViewHeight), placeKey, downAction, keySpace, keyEnter)

	// Act
	view := typing(t, model, "v").View().Content

	// Assert
	refuseScreen(t, view, "places:")
}

func TestAMarkChangeThatMovesTheSelectionReadsTheIssueNowSelected(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := placesWorld()
	started := repo.tasks.linked[0]
	other := started
	other.UUID, other.ID, other.IssueKey = trackedTaskUUID, 3, leakIssue
	repo.tasks.linked = []taskwarrior.Task{other, started}
	model := typing(t, repo.live(t, placesViewWidth, placesViewHeight),
		placeKey, downAction, downAction, downAction, downAction, keySpace, keyEnter, downAction)
	stopped := started
	stopped.Start = time.Time{}
	repo.tasks.linked = []taskwarrior.Task{other, stopped}
	readsBefore := len(repo.asked("issue " + leakIssue))

	// Act
	typing(t, model, "7", "r")

	// Assert
	if reads := len(repo.asked("issue " + leakIssue)); reads == readsBefore {
		t.Errorf("the selection moved to %s and its detail was not read again", leakIssue)
	}
}

func TestAStatusOutsideTheThreeCategoriesIsStillOffered(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := placesWorld()
	repo.issues = append(repo.issues, jira.Issue{
		Key: "PROJ-506", Summary: "Park the idea", Status: "Parked", StatusCategory: "undefined",
	})

	// Act
	view := typing(t, repo.live(t, placesViewWidth, placesViewHeight), placeKey).View().Content

	// Assert
	requireScreen(t, view, "Parked  1")
}

func TestAFailedBranchRelistingKeepsWhatWasInFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := placesWorld()
	model := typing(t, repo.live(t, placesViewWidth, placesViewHeight),
		placeKey, downAction, downAction, downAction, keySpace, keyEnter)
	repo.branchesErr = errListRemotes

	// Act
	view := typing(t, model, "r").View().Content

	// Assert
	if got := strings.Join(listedKeys(view), " "); got != searchIssue {
		t.Errorf("listed %q after a failed relisting, want the issue still in flight:\n%s", got, plain(view))
	}
}

func TestPlacesThatEmptyTheListSaySo(t *testing.T) {
	t.Parallel()

	// Arrange
	keys := append([]string{placeKey, keySpace, keyEnter, "/"}, letters("leak")...)

	// Act
	view := typing(t, placesWorld().live(t, placesViewWidth, placesViewHeight), keys...).View().Content

	// Assert
	requireScreen(t, view, "no match for filter and places")
}
