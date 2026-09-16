// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package editor_test

import (
	"errors"
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

// errEditorCrashed stands in for an editor that exited non-zero.
var errEditorCrashed = errors.New("exit status 1")

// failure is what a finished edit reports back, captured for a test to read.
type failure struct{ err error }

// saved is a finished edit's text.
type saved struct{ text string }

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

			got := editor.Invocation(environment(tt.env), "/work", "notes.md", 0)
			if got.Name != tt.want || got.Dir != "/work" {
				t.Errorf("Invocation = %+v, want %s in /work", got, tt.want)
			}
		})
	}
}

func TestInvocationKeepsTheEditorsOwnArguments(t *testing.T) {
	t.Parallel()

	// No shell runs this, so the words are split here — and a GUI editor needs
	// its --wait, or it returns before anything was written.
	code := environment(map[string]string{editorVariable: "code --wait --new-window"})
	got := editor.Invocation(code, "/work", "notes.md", 0)

	if got.Name != "code" || !slices.Equal(got.Args, []string{"--wait", "--new-window", "notes.md"}) {
		t.Errorf("Invocation = %+v, want code with its flags and then the file", got)
	}
}

func TestInvocationOpensAtALineWhereTheEditorCan(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		editor string
		want   []string
	}{
		"vim":            {editor: vim, want: []string{"+12", sourceFile}},
		"neovim on path": {editor: "/usr/local/bin/nvim", want: []string{"+12", sourceFile}},
		"windows neovim": {editor: `C:\tools\nvim.exe`, want: []string{"+12", sourceFile}},
		nano:             {editor: nano, want: []string{"+12", sourceFile}},
		"emacs client":   {editor: "emacsclient -t", want: []string{"-t", "+12", sourceFile}},
		"vs code":        {editor: "code --wait", want: []string{"--wait", "--goto", sourceAtLine}},
		"cursor":         {editor: "cursor", want: []string{"--goto", sourceAtLine}},
		"helix":          {editor: "hx", want: []string{sourceAtLine}},
		"sublime":        {editor: "subl -w", want: []string{"-w", sourceAtLine}},
		"something else": {editor: "ed", want: []string{sourceFile}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := editor.Invocation(environment(map[string]string{editorVariable: tt.editor}), "/work", sourceFile, 12)
			if !slices.Equal(got.Args, tt.want) {
				t.Errorf("Invocation(%s) args = %q, want %q", tt.editor, got.Args, tt.want)
			}
		})
	}

	// Without a line, every editor just gets the file.
	got := editor.Invocation(environment(map[string]string{editorVariable: vim}), "/work", sourceFile, 0)
	if !slices.Equal(got.Args, []string{sourceFile}) {
		t.Errorf("Invocation with no line = %q, want just the file", got.Args)
	}
}

func TestADraftComesBackWithoutItsHelp(t *testing.T) {
	t.Parallel()

	const original = "## What this changes\n\nTokens are masked."

	draft := editor.Draft(original, "Everything below is ignored.\nSave and quit to continue.")

	// The help sits below a scissors line rather than behind # comments: a pull
	// request body is Markdown, and its headings start with #.
	edited := "## What this changes\n\nTokens are masked, now in the log too.  \n\n\n" + draft[len(original):]

	if got, want := editor.Parse(edited), "## What this changes\n\nTokens are masked, now in the log too."; got != want {
		t.Errorf("Parse = %q, want %q", got, want)
	}

	if got := editor.Parse("no scissors at all\n"); got != "no scissors at all" {
		t.Errorf("Parse without a scissors line = %q, want the whole text", got)
	}

	if got := editor.Parse(editor.Draft("", "help")); got != "" {
		t.Errorf("Parse of an untouched empty draft = %q, want nothing", got)
	}
}

func TestEditWithAnEditorThatIsNotInstalledSaysSo(t *testing.T) {
	t.Parallel()

	missing := environment(map[string]string{editorVariable: "workflow-editor-that-does-not-exist"})

	cmd := editor.Edit(missing, t.TempDir(), "draft", "help", func(_ string, err error) tea.Msg {
		return failure{err: err}
	})

	reported, ok := cmd().(failure)
	if !ok || !errors.Is(reported.err, proc.ErrNotFound) {
		t.Errorf("Edit reported %v, want ErrNotFound for an editor not on PATH", reported.err)
	}

	cmd = editor.Open(missing, t.TempDir(), sourceFile, 3, func(err error) tea.Msg { return failure{err: err} })

	reported, ok = cmd().(failure)
	if !ok || !errors.Is(reported.err, proc.ErrNotFound) {
		t.Errorf("Open reported %v, want ErrNotFound for an editor not on PATH", reported.err)
	}
}

func TestEditWritesTheDraftOutsideTheRepositoryAndHandsOverTheTerminal(t *testing.T) {
	t.Parallel()

	drafts := t.TempDir()
	env := environment(map[string]string{editorVariable: installedEditor, "TMPDIR": drafts})

	cmd := editor.Edit(env, t.TempDir(), "draft", "help", func(_ string, err error) tea.Msg {
		return failure{err: err}
	})

	// The command hands the terminal to the editor; nothing has been reported
	// back yet, because the editor has not run.
	if _, reported := cmd().(failure); reported {
		t.Error("Edit reported back before the editor ran")
	}

	written, _ := filepath.Glob(filepath.Join(drafts, "workflow-*.md"))
	if len(written) != 1 {
		t.Fatalf("found drafts %q in $TMPDIR, want exactly one", written)
	}

	opened := editor.Open(env, t.TempDir(), sourceFile, 3, func(error) tea.Msg { return failure{} })
	if _, reported := opened().(failure); reported {
		t.Error("Open reported back before the editor ran")
	}
}

func TestEditReportsADraftThatCannotBeWritten(t *testing.T) {
	t.Parallel()

	env := environment(map[string]string{editorVariable: installedEditor, "TMPDIR": filepath.Join(t.TempDir(), "missing")})

	reported, ok := editor.Edit(env, t.TempDir(), "draft", "help", func(_ string, err error) tea.Msg {
		return failure{err: err}
	})().(failure)
	if !ok || reported.err == nil {
		t.Errorf("Edit reported %+v, want the failure to create a draft", reported)
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

func TestCollectReadsTheSavedDraftAndRemovesIt(t *testing.T) {
	t.Parallel()

	collected := func(path string, editorErr error) tea.Msg {
		return editor.Collect(path, func(text string, err error) tea.Msg {
			if err != nil {
				return failure{err: err}
			}

			return saved{text: text}
		})(editorErr)
	}

	path := filepath.Join(t.TempDir(), "draft.md")

	write := func() {
		err := os.WriteFile(path, []byte(editor.Draft("kept text", "help")), 0o600)
		if err != nil {
			t.Fatalf("writing the draft: %v", err)
		}
	}

	write()

	if got, ok := collected(path, nil).(saved); !ok || got.text != "kept text" {
		t.Errorf("Collect = %+v, want the saved text", got)
	}

	requireGone(t, path)

	write()

	// An editor that failed keeps nothing — and still cleans up.
	if got, ok := collected(path, errEditorCrashed).(failure); !ok || !errors.Is(got.err, errEditorCrashed) {
		t.Errorf("Collect after a crash = %+v, want the editor's error", got)
	}

	requireGone(t, path)

	if got, ok := collected(path, nil).(failure); !ok || got.err == nil {
		t.Errorf("Collect of a vanished draft = %+v, want a read error", got)
	}
}
