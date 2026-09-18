// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package editor hands text and files to the user's own editor.
//
// Every multi-line thing workflow asks for — a commit body, a Jira comment, a
// pull request description, a Slack message — is written in $EDITOR rather than
// in a text box drawn inside the interface: people already have an editor they
// are fast in, and a text box would be a worse one.
package editor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// Scissors marks where a draft's text ends and its help begins. Everything from
// this line down is thrown away when the draft comes back. It is git's own
// scissors line, and it is a line rather than a # comment prefix because the
// text is often Markdown, whose headings start with #.
const Scissors = "# ------------------------ >8 ------------------------"

// Getenv reads an environment variable. os.Getenv satisfies it.
type Getenv func(string) string

// lineArgs shapes the arguments that open a file at a line.
type lineArgs func(file, line string) []string

// lineShapes maps an editor's program name to how it is told a line. An editor
// missing from the map is given the file alone.
func lineShapes() map[string]lineArgs {
	plus := func(file, line string) []string { return []string{"+" + line, file} }
	gotoFlag := func(file, line string) []string { return []string{"--goto", file + ":" + line} }
	suffix := func(file, line string) []string { return []string{file + ":" + line} }

	return map[string]lineArgs{
		"vi": plus, "vim": plus, "nvim": plus, "nano": plus, "emacs": plus,
		"emacsclient": plus, "micro": plus, "kak": plus, "mg": plus,
		"code": gotoFlag, "code-insiders": gotoFlag, "codium": gotoFlag, "cursor": gotoFlag,
		"hx": suffix, "helix": suffix, "subl": suffix, "zed": suffix,
	}
}

// Invocation is how to open a file in the user's editor, at a line when the
// editor can be told one and line is positive.
//
// The editor setting is split into words here rather than run through a shell:
// "code --wait" is common and works, while shell syntax in $EDITOR is rare and
// is not something to hand to sh on the user's behalf.
func Invocation(getenv Getenv, dir, file string, line int) proc.Command {
	words := strings.Fields(chosen(getenv))
	name, own := words[0], words[1:]

	return proc.Command{Dir: dir, Name: name, Args: append(own, fileArgs(name, file, line)...), Env: nil}
}

// chosen is the editor setting in effect.
func chosen(getenv Getenv) string {
	for _, variable := range []string{"VISUAL", "EDITOR"} {
		if value := strings.TrimSpace(getenv(variable)); value != "" {
			return value
		}
	}

	return defaultEditor(runtime.GOOS)
}

// defaultEditor is what opens when neither $VISUAL nor $EDITOR is set: vi on
// Unix, and notepad on Windows, where vi is not usually present.
func defaultEditor(goos string) string {
	if goos == "windows" {
		return "notepad"
	}

	return "vi"
}

// fileArgs are the arguments naming the file, and the line if the editor takes
// one.
func fileArgs(name, file string, line int) []string {
	program := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(name, `\`, "/")), ".exe")

	shape, known := lineShapes()[program]
	if !known || line <= 0 {
		return []string{file}
	}

	return shape(file, strconv.Itoa(line))
}

// Draft is what the editor opens: the text, then the scissors line and help that
// will be thrown away.
func Draft(text, help string) string {
	return text + "\n\n" + Scissors + "\n" + help + "\n"
}

// Parse is what the user wrote: everything above the scissors line, with the
// trailing space editors leave behind removed.
func Parse(raw string) string {
	text, _, _ := strings.Cut(raw, Scissors)

	lines := strings.Split(text, "\n")
	for index, line := range lines {
		lines[index] = strings.TrimRight(line, " \t\r")
	}

	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// Edit hands text to the user's editor and reports what they saved, through
// done. The interface releases the terminal while the editor has it. The draft
// is written to $TMPDIR, or the system's temporary directory, never into the
// repository, where it would show up as an untracked file.
func Edit(getenv Getenv, dir, text, help string, done func(string, error) tea.Msg) tea.Cmd {
	path, err := writeDraft(getenv("TMPDIR"), Draft(text, help))
	if err != nil {
		return report(done("", err))
	}

	command, err := proc.Interactive(Invocation(getenv, dir, path, 0))
	if err != nil {
		_ = os.Remove(path)

		return report(done("", err))
	}

	return tea.ExecProcess(command, Collect(path, done))
}

// Collect is what happens once the editor closes: the draft at path is read
// back, parsed, reported through done, and removed. An editor that exited with
// an error keeps nothing, the way git treats an aborted commit message.
func Collect(path string, done func(string, error) tea.Msg) func(error) tea.Msg {
	return func(editorErr error) tea.Msg {
		defer func() { _ = os.Remove(path) }()

		if editorErr != nil {
			return done("", fmt.Errorf("the editor exited with an error, so nothing was kept: %w", editorErr))
		}

		saved, err := os.ReadFile(path) //nolint:gosec // the path is the draft Edit wrote
		if err != nil {
			return done("", fmt.Errorf("reading the draft back: %w", err))
		}

		return done(Parse(string(saved)), nil)
	}
}

// ErrNoSuchFile reports a place to open that names no file.
var ErrNoSuchFile = errors.New("no such file")

// Locate names a file in full: from dir, unless it is a full path already.
//
// A place comes from what a tool printed, so it may name nothing (go test
// prints places relative to a package, not to the repository) and it may begin
// with anything a file name can. An editor handed a file that is not there
// opens an empty buffer, and saving that leaves a stray file; and an editor
// reads an argument by how it begins. Named in full, a file is there, and its
// name begins with the root.
func Locate(dir, file string) (string, error) {
	full := file
	if !filepath.IsAbs(full) {
		full = filepath.Join(dir, file)
	}

	info, err := os.Stat(full)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: %s", ErrNoSuchFile, file)
	}

	return full, nil
}

// Resolve turns a place a tool printed into a file Open can find, or reports
// that none does. A file that is there as printed is kept unchanged; one that is
// not — go test prints a place relative to its package, not the root — is looked
// for below dir and kept only when exactly one file matches, so an ambiguous
// name resolves to nothing rather than to the wrong file.
func Resolve(dir, file string) (string, bool) {
	_, err := Locate(dir, file)
	if err == nil {
		return file, true
	}

	matches := matchesBelow(dir, file)
	if len(matches) == 1 {
		return matches[0], true
	}

	return "", false
}

// matchesBelow lists, relative to dir, every file whose path is or ends in the
// place a tool printed. The place may be a bare name or a partial path; the .git
// directory is skipped so its packed copies of tracked files are not matches.
func matchesBelow(dir, place string) []string {
	suffix := filepath.ToSlash(filepath.Clean(place))

	var matches []string

	_ = filepath.WalkDir(dir, func(name string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			//nolint:nilerr // an unreadable entry is one place the file is not, not a reason to abandon the search.
			return nil
		case entry.IsDir() && entry.Name() == ".git" && name != dir:
			return fs.SkipDir
		case entry.IsDir():
			return nil
		}

		rel, err := filepath.Rel(dir, name)
		if err == nil && placeMatches(filepath.ToSlash(rel), suffix) {
			matches = append(matches, rel)
		}

		return nil
	})

	return matches
}

// placeMatches reports whether a file's slash-separated path is, or ends in, the
// place a tool named.
func placeMatches(relSlash, suffix string) bool {
	return relSlash == suffix || strings.HasSuffix(relSlash, "/"+suffix)
}

// Open opens a file in the user's editor at a line, and reports through done
// once the editor has closed.
func Open(getenv Getenv, dir, file string, line int, done func(error) tea.Msg) tea.Cmd {
	located, err := Locate(dir, file)
	if err != nil {
		return report(done(err))
	}

	command, err := proc.Interactive(Invocation(getenv, dir, located, line))
	if err != nil {
		return report(done(err))
	}

	return tea.ExecProcess(command, func(runErr error) tea.Msg { return done(runErr) })
}

// writeDraft puts a draft in a new private file in dir, or the system's
// temporary directory when dir is empty. The .md suffix gets Markdown
// highlighting in editors that key off it, which suits most drafts.
func writeDraft(dir, text string) (string, error) {
	file, err := os.CreateTemp(dir, "workflow-*.md")
	if err != nil {
		return "", fmt.Errorf("creating a draft: %w", err)
	}

	_, writeErr := file.WriteString(text)

	err = errors.Join(writeErr, file.Close())
	if err != nil {
		_ = os.Remove(file.Name())

		return "", fmt.Errorf("writing the draft: %w", err)
	}

	return file.Name(), nil
}

// report is a command that delivers an already-known message.
func report(msg tea.Msg) tea.Cmd {
	return func() tea.Msg { return msg }
}
