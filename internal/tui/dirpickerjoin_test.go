// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"regexp"
	"testing"

	"github.com/jacob-delgado/workflow/internal/workdirs"
)

func TestAListingAskedAgainByTheNameKeepsWhatTheWholeOneMatched(t *testing.T) {
	t.Parallel()

	// A listing leaves a hidden directory out until the name typed starts
	// with its dot, and one cut short leaves out the directories past its
	// limit, so the picker asks again with the name; what the whole listing
	// holds that letter by letter, a.cache for .c, is listed beside it.
	cases := map[string]struct {
		dir   string
		names []string
		typed string
		want  []string
	}{
		"a hidden directory's dot": {
			dir: anaHome, names: []string{srcName, oldName, configName, "a.cache"},
			typed: "~/.c", want: []string{configName, "a.cache"},
		},
		"a directory cut short": {
			dir: srcDir, names: append(manyDirs(workdirs.ListLimit+200), "r1-notes"),
			typed: "~/src/r1", want: []string{"r1-notes", "dir1000"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			working := pickingWorld()
			working.dirs.listings[tt.dir] = tt.names

			// Act
			view := picking(t, working, tt.typed).View().Content

			// Assert
			for _, listed := range tt.want {
				if pickerLine(view, listed) == "" {
					t.Errorf("typing %s does not list %s:\n%s", tt.typed, listed, plain(view))
				}
			}
		})
	}
}

// hiddenWorld is pickingWorld with a hidden directory in your home, which a
// listing holds only once the name typed starts with its dot.
func hiddenWorld() *world {
	working := pickingWorld()
	working.dirs.listings[anaHome] = []string{srcName, oldName, configName}

	return working
}

func TestWhileAListingIsAskedAgainByTheNameThePickerSaysItIsReading(t *testing.T) {
	t.Parallel()

	// A key typed asks for the listing narrowed by the name it makes; until
	// that answers, what the last name typed found there stands in for it,
	// as on the web, and the picker says it is reading rather than that
	// nothing matches.
	cases := map[string]struct {
		world   func() *world
		typed   string
		key     string
		reading string
		listed  []string
	}{
		"a directory cut short": {
			world: truncatedWorld, typed: "~/src/wo", key: "r", reading: "reading ~/src", listed: []string{notesDir},
		},
		"a hidden directory's dot, after it": {
			world: hiddenWorld, typed: "~/.c", key: "o", reading: "reading ~", listed: []string{configName},
		},
		"a hidden directory's dot, with nothing found yet": {
			world: hiddenWorld, typed: "~/", key: ".", reading: "reading ~",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			settled := picking(t, tt.world(), tt.typed)

			// Act
			asked, _ := pressed(t, settled, tt.key)

			// Assert
			view := asked.View().Content
			requireScreen(t, view, tt.reading)
			refuseScreen(t, view, "nothing in")

			for _, listed := range tt.listed {
				if pickerLine(view, listed) == "" {
					t.Errorf("%s, found for %s, is not listed while %s%s is read:\n%s",
						listed, tt.typed, tt.typed, tt.key, plain(view))
				}
			}
		})
	}
}

// countedMatches is the picker's count of the directories matching what was
// typed, out of those listed.
var countedMatches = regexp.MustCompile(`\d+ of \d+`)

func TestANarrowedListingAnsweredBeforeTheWholeOneLeavesThePickerReading(t *testing.T) {
	t.Parallel()

	// Arrange
	// ~/src's whole listing is asked for and still out when the dot asks for
	// the one narrowed by it, which answers first with .config, as the
	// quicker read in a big directory does. x.y matches the dot letter by
	// letter, and only the whole listing holds it, so the picker cannot yet
	// count what matches.
	working := pickingWorld()
	working.dirs.listings[srcDir] = []string{apiDir, "x.y", configName}
	typed, _ := pressed(t, picking(t, working, "~/src"), "/")
	asked, narrowed := pressed(t, typed, ".")

	// Act
	answered, _ := finish(t, asked, narrowed)

	// Assert
	view := answered.View().Content
	requireScreen(t, view, "reading ~/src")

	if counted := countedMatches.FindString(pickerText(t, view)); counted != "" {
		t.Errorf("the picker counts %s while the whole listing is out:\n%s", counted, plain(view))
	}

	if pickerLine(view, configName) == "" {
		t.Errorf("%s, in the narrowed listing that answered, is not listed:\n%s", configName, plain(view))
	}
}

// errListingCutOff is a read of a whole directory cut off partway through.
var errListingCutOff = errors.New("reading the directory was cut off")

func TestAWholeListingThatFailedLeavesWhatTheNarrowedOneFoundToGoInto(t *testing.T) {
	t.Parallel()

	// Your home's whole listing fails, and the one narrowed by ~/.c answers
	// with .config: the picker lists it beside the failure, and goes into it.
	cases := map[string]struct {
		keys []string
		want []string
	}{
		"listed beside the failure": {want: []string{"▸ " + configName, errListingCutOff.Error()}},
		"gone into":                 {keys: []string{"right"}, want: []string{"› ~/.config/", "in ~/.config"}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			working := hiddenWorld()
			working.dirs.wholeErr = errListingCutOff

			// Act
			view := picking(t, working, "~/.c", tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want...)
		})
	}
}

func TestAListingThatFailedListsNoneOfWhatItReadBeforeItWasCutOff(t *testing.T) {
	t.Parallel()

	// Arrange
	// Your home's listing fails, handing back src and old, which it read
	// before it was cut off: the picker says why, and lists neither.
	working := pickingWorld()
	working.dirs.wholeErr = errListingCutOff

	// Act
	view := picking(t, working, "~/").View().Content

	// Assert
	requireScreen(t, view, errListingCutOff.Error())

	for _, name := range []string{srcName, oldName} {
		if line := pickerLine(pickerText(t, view), name); line != "" {
			t.Errorf("%s, read before the listing failed, is listed as %q:\n%s", name, line, plain(view))
		}
	}
}

func TestANarrowedListingThatFailedIsShownInPlaceOfTheRows(t *testing.T) {
	t.Parallel()

	// Arrange
	// Your home's whole listing answers with a.cache, which holds .c letter
	// by letter, and the one narrowed by ~/.c fails.
	working := hiddenWorld()
	working.dirs.listings[anaHome] = append(working.dirs.listings[anaHome], "a.cache")
	working.dirs.narrowedErr = errListingCutOff

	// Act
	view := picking(t, working, "~/.c").View().Content

	// Assert
	requireScreen(t, view, errListingCutOff.Error())

	if pickerLine(view, "a.cache") != "" {
		t.Errorf("a.cache is listed though the listing narrowed by .c failed:\n%s", plain(view))
	}
}

func TestANarrowedListingOfAnotherDirectoryNeverStandsIn(t *testing.T) {
	t.Parallel()

	// Arrange
	// ~/.c found .config in your home; the field then goes to ~/src/, whose
	// whole listing answers and holds no .config.
	settled := picking(t, hiddenWorld(), "~/.c", keyBackspace, keyBackspace, "s", "r", "c", "/")

	// Act
	asked, _ := pressed(t, settled, ".")

	// Assert
	view := asked.View().Content
	requireScreen(t, view, "reading ~/src")

	if pickerLine(view, configName) != "" {
		t.Errorf("%s, found in your home, is listed in ~/src while its listing narrowed by the dot is read:\n%s",
			configName, plain(view))
	}
}
