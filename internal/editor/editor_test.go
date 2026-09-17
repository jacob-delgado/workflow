// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package editor_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/editor"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// environment answers only the variables it is given.
func environment(values map[string]string) editor.Getenv {
	return func(name string) string { return values[name] }
}

// sourceFile and sourceAtLine are the file the line tests open.
const (
	sourceFile   = "main.go"
	sourceAtLine = "main.go:12"
)

// editorVariable is the environment variable most people set.
const editorVariable = "EDITOR"

// vim and nano are the editors the tests reach for first.
const (
	vim  = "vim"
	nano = "nano"
)

// installedEditor is a program every machine running these tests has. Nothing
// here runs it: a command is only built, never executed.
const installedEditor = "go"

// missingEditor is an editor no machine has.
const missingEditor = "workflow-editor-that-does-not-exist"

// tmpdirVariable is where Edit writes its drafts.
const tmpdirVariable = "TMPDIR"

// draftPattern matches the drafts Edit writes.
const draftPattern = "workflow-*.md"

// errEditorCrashed stands in for an editor that exited non-zero.
var errEditorCrashed = errors.New("exit status 1")

// failure is what a finished edit reports back, captured for a test to read.
type failure struct{ err error }

// saved is a finished edit's text.
type saved struct{ text string }

// repositoryWith is a directory holding the named files, empty.
func repositoryWith(t *testing.T, names ...string) string {
	t.Helper()

	dir := t.TempDir()

	for _, name := range names {
		err := os.WriteFile(filepath.Join(dir, name), nil, 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

// drafts lists the drafts in dir.
func drafts(t *testing.T, dir string) []string {
	t.Helper()

	found, err := filepath.Glob(filepath.Join(dir, draftPattern))
	if err != nil {
		t.Fatal(err)
	}

	return found
}

func TestInvocationPrefersVisualThenEditorThenVi(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		env  map[string]string
		want string
	}{
		"visual wins":        {env: map[string]string{"VISUAL": "nvim", editorVariable: nano}, want: "nvim"},
		"then editor":        {env: map[string]string{editorVariable: nano}, want: nano},
		"blank is unset":     {env: map[string]string{"VISUAL": "  ", editorVariable: nano}, want: nano},
		"vi is always there": {env: map[string]string{}, want: "vi"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := editor.Invocation(environment(tt.env), "/work", "notes.md", 0)

			// Assert
			if got.Name != tt.want || got.Dir != "/work" {
				t.Errorf("Invocation = %+v, want %s in /work", got, tt.want)
			}
		})
	}
}

func TestInvocationKeepsTheEditorsOwnArguments(t *testing.T) {
	t.Parallel()

	// Arrange
	// No shell runs this, so the words are split here — and a GUI editor needs
	// its --wait, or it returns before anything was written.
	code := environment(map[string]string{editorVariable: "code --wait --new-window"})

	// Act
	got := editor.Invocation(code, "/work", "notes.md", 0)

	// Assert
	if got.Name != "code" || !slices.Equal(got.Args, []string{"--wait", "--new-window", "notes.md"}) {
		t.Errorf("Invocation = %+v, want code with its flags and then the file", got)
	}
}

func TestInvocationOpensAtALineWhereTheEditorCan(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		editor string
		line   int
		want   []string
	}{
		"vim":            {editor: vim, line: 12, want: []string{"+12", sourceFile}},
		"neovim on path": {editor: "/usr/local/bin/nvim", line: 12, want: []string{"+12", sourceFile}},
		"windows neovim": {editor: `C:\tools\nvim.exe`, line: 12, want: []string{"+12", sourceFile}},
		nano:             {editor: nano, line: 12, want: []string{"+12", sourceFile}},
		"emacs client":   {editor: "emacsclient -t", line: 12, want: []string{"-t", "+12", sourceFile}},
		"vs code":        {editor: "code --wait", line: 12, want: []string{"--wait", "--goto", sourceAtLine}},
		"cursor":         {editor: "cursor", line: 12, want: []string{"--goto", sourceAtLine}},
		"helix":          {editor: "hx", line: 12, want: []string{sourceAtLine}},
		"sublime":        {editor: "subl -w", line: 12, want: []string{"-w", sourceAtLine}},
		"something else": {editor: "ed", line: 12, want: []string{sourceFile}},
		// Without a line, even an editor that can open at one just gets the file.
		"no line": {editor: vim, line: 0, want: []string{sourceFile}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			env := environment(map[string]string{editorVariable: tt.editor})

			// Act
			got := editor.Invocation(env, "/work", sourceFile, tt.line)

			// Assert
			if !slices.Equal(got.Args, tt.want) {
				t.Errorf("Invocation(%s) args = %q, want %q", tt.editor, got.Args, tt.want)
			}
		})
	}
}

func TestADraftComesBackWithoutItsHelp(t *testing.T) {
	t.Parallel()

	const original = "## What this changes\n\nTokens are masked."

	// The help sits below a scissors line rather than behind # comments: a pull
	// request body is Markdown, and its headings start with #.
	draft := editor.Draft(original, "Everything below is ignored.\nSave and quit to continue.")

	cases := map[string]struct {
		saved string
		want  string
	}{
		"an edited draft, with trailing space and lines": {
			saved: "## What this changes\n\nTokens are masked, now in the log too.  \n\n\n" + draft[len(original):],
			want:  "## What this changes\n\nTokens are masked, now in the log too.",
		},
		"text with no scissors line": {saved: "no scissors at all\n", want: "no scissors at all"},
		"an untouched, empty draft":  {saved: editor.Draft("", "help"), want: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := editor.Parse(tt.saved)

			// Assert
			if got != tt.want {
				t.Errorf("Parse = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEditWithAnEditorThatIsNotInstalledSaysSoAndLeavesNoDraft(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	missing := environment(map[string]string{editorVariable: missingEditor, tmpdirVariable: dir})

	// Act
	reported, ok := editor.Edit(missing, t.TempDir(), "draft", "help", func(_ string, err error) tea.Msg {
		return failure{err: err}
	})().(failure)

	// Assert
	if !ok || !errors.Is(reported.err, proc.ErrNotFound) {
		t.Errorf("Edit reported %v, want ErrNotFound for an editor not on PATH", reported.err)
	}

	if left := drafts(t, dir); len(left) != 0 {
		t.Errorf("the draft outlived the editor that could not open it: %q", left)
	}
}

func TestOpenWithAnEditorThatIsNotInstalledSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	missing := environment(map[string]string{editorVariable: missingEditor})

	// Act
	reported, ok := editor.Open(missing, repositoryWith(t, sourceFile), sourceFile, 3, func(err error) tea.Msg {
		return failure{err: err}
	})().(failure)

	// Assert
	if !ok || !errors.Is(reported.err, proc.ErrNotFound) {
		t.Errorf("Open reported %v, want ErrNotFound for an editor not on PATH", reported.err)
	}
}

func TestEditWritesTheDraftOutsideTheRepositoryAndHandsOverTheTerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	env := environment(map[string]string{editorVariable: installedEditor, tmpdirVariable: dir})
	finished := false

	// Act
	msg := editor.Edit(env, t.TempDir(), "draft", "help", func(string, error) tea.Msg {
		finished = true

		return nil
	})()

	// Assert
	// The command hands the terminal to the editor: nothing is reported back
	// until the editor has run.
	if msg == nil || finished {
		t.Errorf("Edit returned %v and finished %v, want a handover and nothing reported yet", msg, finished)
	}

	written := drafts(t, dir)
	if len(written) != 1 {
		t.Fatalf("found drafts %q in $TMPDIR, want exactly one", written)
	}

	contents, err := os.ReadFile(written[0])
	if err != nil || string(contents) != editor.Draft("draft", "help") {
		t.Errorf("the draft holds %q, %v; want the text and its help", contents, err)
	}
}

func TestOpenHandsOverTheTerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	env := environment(map[string]string{editorVariable: installedEditor})
	finished := false

	// Act
	msg := editor.Open(env, repositoryWith(t, sourceFile), sourceFile, 3, func(error) tea.Msg {
		finished = true

		return nil
	})()

	// Assert
	if msg == nil || finished {
		t.Errorf("Open returned %v and finished %v, want a handover and nothing reported yet", msg, finished)
	}
}

func TestEditReportsADraftThatCannotBeWritten(t *testing.T) {
	t.Parallel()

	// Arrange
	missingDir := filepath.Join(t.TempDir(), "missing")
	env := environment(map[string]string{editorVariable: installedEditor, tmpdirVariable: missingDir})

	// Act
	reported, ok := editor.Edit(env, t.TempDir(), "draft", "help", func(_ string, err error) tea.Msg {
		return failure{err: err}
	})().(failure)

	// Assert
	if !ok || !errors.Is(reported.err, fs.ErrNotExist) {
		t.Errorf("Edit reported %+v, want the failure to create a draft in a missing $TMPDIR", reported)
	}
}

// requireGone fails the test if a draft outlived being collected.
func requireGone(t *testing.T, path string) {
	t.Helper()

	_, err := os.Stat(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the draft %s is still there: %v", path, err)
	}
}

// collected is what Collect reports for a draft once the editor exits with
// editorErr.
//
//nolint:ireturn // tea.Msg is Bubble Tea's type for any message at all
func collected(path string, editorErr error) tea.Msg {
	return editor.Collect(path, func(text string, err error) tea.Msg {
		if err != nil {
			return failure{err: err}
		}

		return saved{text: text}
	})(editorErr)
}

// writtenDraft is a saved draft of text, in a directory of its own.
func writtenDraft(t *testing.T, text string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "draft.md")

	err := os.WriteFile(path, []byte(editor.Draft(text, "help")), 0o600)
	if err != nil {
		t.Fatalf("writing the draft: %v", err)
	}

	return path
}

func TestCollectReadsTheSavedDraftAndRemovesIt(t *testing.T) {
	t.Parallel()

	// Arrange
	path := writtenDraft(t, "kept text")

	// Act
	got, ok := collected(path, nil).(saved)

	// Assert
	if !ok || got.text != "kept text" {
		t.Errorf("Collect = %+v, want the saved text", got)
	}

	requireGone(t, path)
}

func TestCollectAfterTheEditorFailedKeepsNothingAndStillCleansUp(t *testing.T) {
	t.Parallel()

	// Arrange
	path := writtenDraft(t, "kept text")

	// Act
	got, ok := collected(path, errEditorCrashed).(failure)

	// Assert
	if !ok || !errors.Is(got.err, errEditorCrashed) {
		t.Errorf("Collect after a crash = %+v, want the editor's error", got)
	}

	requireGone(t, path)
}

func TestCollectOfAVanishedDraftReportsTheReadError(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), "draft.md")

	// Act
	got, ok := collected(path, nil).(failure)

	// Assert
	if !ok || !errors.Is(got.err, fs.ErrNotExist) {
		t.Errorf("Collect of a vanished draft = %+v, want the read's not-exist error", got)
	}
}
