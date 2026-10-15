// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package quietinput_test

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/tui/quietinput"
)

// shown is what field draws, without its styles.
func shown(field *quietinput.Input) string {
	return ansi.Strip(field.View())
}

// upperFocusedTheme is Huh's base theme with a focused field's title in
// capitals, so a test can tell which of its styles drew a field.
func upperFocusedTheme() huh.ThemeFunc {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		styles := huh.ThemeBase(isDark)
		styles.Focused.Title = styles.Focused.Title.Transform(strings.ToUpper)

		return styles
	})
}

func TestViewDrawsTheHintUnderTheTitleOnlyWhenThereIsOne(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		hint     string
		wantRows int
	}{
		"hinted":     {hint: "The branch it merges into.", wantRows: 3},
		"not hinted": {hint: "", wantRows: 2},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var value string

			field := quietinput.New(baseTitle, "", &value).Hinted(tt.hint)

			// Act
			rows := strings.Split(shown(field), "\n")

			// Assert
			if len(rows) != tt.wantRows || !strings.Contains(rows[0], baseTitle) ||
				!strings.Contains(strings.Join(rows, "\n"), tt.hint) {
				t.Errorf("drew %d rows, want %d: the title, the hint %q if any, and the line:\n%s",
					len(rows), tt.wantRows, tt.hint, strings.Join(rows, "\n"))
			}
		})
	}
}

func TestViewDrawsInTheThemesStylesForAFieldFocusedOrNot(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		theme     huh.Theme
		focus     bool
		wantTitle string
	}{
		"a field given no theme, in Huh's base theme": {theme: nil, focus: true, wantTitle: baseTitle},
		"a focused field, in the focused styles":      {theme: upperFocusedTheme(), focus: true, wantTitle: "BASE"},
		"a blurred field, in the blurred styles":      {theme: upperFocusedTheme(), focus: false, wantTitle: baseTitle},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var value string

			field := quietinput.New(baseTitle, "", &value)
			if tt.theme != nil {
				field.WithTheme(tt.theme)
			}

			if tt.focus {
				field.Focus()
			}

			// Act
			title, _, _ := strings.Cut(shown(field), "\n")

			// Assert
			if !strings.Contains(title, tt.wantTitle) {
				t.Errorf("drew the title %q, want %q", title, tt.wantTitle)
			}
		})
	}
}

func TestSecretMasksWhatIsTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	var value string

	field := focused(quietinput.New("Token", "", &value).Secret(), defaultKeys())

	// Act
	field.Update(tea.PasteMsg{Content: "hunter2"})

	// Assert
	if drawn := shown(field); strings.Contains(drawn, "hunter2") || !strings.Contains(drawn, "******") ||
		value != "hunter2" {
		t.Errorf("a secret field kept %q and drew:\n%s\nwant hunter2 kept and drawn masked", value, drawn)
	}
}

func TestWithPositionOffersEachKeyOnlyWhereItGoesSomewhere(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		position                       huh.FieldPosition
		wantPrev, wantSubmit, wantNext bool
	}{
		"the first of two fields": {
			position: huh.FieldPosition{Field: 0, FirstField: 0, LastField: 1},
			wantPrev: false, wantSubmit: false, wantNext: true,
		},
		"the last of two fields": {
			position: huh.FieldPosition{Field: 1, FirstField: 0, LastField: 1},
			wantPrev: true, wantSubmit: true, wantNext: false,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var value string

			field := quietinput.New(baseTitle, "", &value)
			field.WithKeyMap(defaultKeys())

			// Act
			field.WithPosition(tt.position)

			// Assert
			binds := field.KeyBinds()
			if len(binds) != 3 || binds[0].Enabled() != tt.wantPrev || binds[1].Enabled() != tt.wantSubmit ||
				binds[2].Enabled() != tt.wantNext {
				t.Errorf("offered previous, submit, next = %v, want %v, %v, %v",
					enabled(binds), tt.wantPrev, tt.wantSubmit, tt.wantNext)
			}
		})
	}
}

// enabled is whether each of binds is offered.
func enabled(binds []key.Binding) []bool {
	offered := make([]bool, len(binds))
	for index, bind := range binds {
		offered[index] = bind.Enabled()
	}

	return offered
}

func TestWithWidthFitsTheFieldToItsColumns(t *testing.T) {
	t.Parallel()

	// Arrange
	value := strings.Repeat("a", 60)
	field := focused(quietinput.New(baseTitle, "", &value), defaultKeys())

	// Act
	field.WithWidth(20).WithHeight(1)

	// Assert
	for row := range strings.SplitSeq(field.View(), "\n") {
		if width := lipgloss.Width(row); width > 20 {
			t.Errorf("a row is %d columns wide, want at most 20: %q", width, ansi.Strip(row))
		}
	}
}
