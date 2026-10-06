// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// The refusals Settings says in its own words.
var (
	errSettingsInvalid = errors.New("the configuration is not valid")
	errNotACount       = errors.New("a count is a whole number, 0 or more; 0 keeps the default")
	errSettingsKeymap  = errors.New("the terminal interface would not start on this keymap")
	errSettingsChanged = errors.New("the configuration file changed after Settings read it — edited on disk, " +
		"or saved from the web — so nothing was saved; reload reads it again in place of your edits here, " +
		"then make your change again")
	errSlackSecretsRefused = errors.New(
		"slack refused the client ID, client secret or refresh token; check them, then save again")
)

// settingsTitle heads the form.
const settingsTitle = "Settings"

// settingsAbout is said above the settings: where they are kept, and when they
// apply.
const settingsAbout = "A save applies when workflow starts again."

// settingsForm is Settings: the configuration the web's Settings edits, a row
// per setting under its section, each credential masked. enter edits a row,
// and a save writes every edit together over the read, as the web's does, so
// a file changed since is refused rather than written over.
type settingsForm struct {
	marks   glyphs
	styles  styles
	fields  []setting
	opened  int
	reading bool
	readErr error
	// read is the configuration as read, masked, in its JSON form, which a
	// save sends back with the edits laid over it; values is the same, read.
	read   []byte
	values map[string]any
	path   string
	// shownPath is path as the screen writes it, from your home.
	shownPath string
	over      config.Revision
	// edits are the values changed since the read, by setting path.
	edits    map[string]any
	selected int
	editing  bool
	input    textinput.Model
	problem  error
	send     sendState
}

var (
	_ failable[settingsForm] = settingsForm{}
	_ pasteable              = settingsForm{}
)

// canEditSettings reports that the configuration files can be read.
func (m Model) canEditSettings() bool {
	return m.deps.Settings.Read != nil
}

// openSettings opens Settings and starts reading the configuration files.
func (m Model) openSettings() (Model, tea.Cmd) {
	m, opened := m.opening()
	m.overlay = settingsForm{
		marks: m.marks, styles: m.styles, fields: settingsFields(m.vocab.noun), opened: opened, reading: true,
	}
	read := m.deps.Settings.Read

	return m, func() tea.Msg {
		cfg, over, err := read()

		return settingsRead{opened: opened, cfg: cfg, over: over, err: err}
	}
}

// settingsRead is the configuration read, masked, or why it was not.
type settingsRead struct {
	opened int
	cfg    config.Config
	over   config.Revision
	err    error
}

var _ applier = settingsRead{}

// apply seeds the form that asked, and nothing once it has closed.
func (msg settingsRead) apply(m Model) (Model, tea.Cmd) {
	form, open := m.overlay.(settingsForm)
	if !open || form.opened != msg.opened {
		return m, nil
	}

	form.reading = false
	// The form masks what it was handed itself, so it never holds a credential
	// whatever the seam gave it, and a credential left alone goes back masked.
	form.read, form.values, form.readErr = seed(msg.cfg.Redacted(), msg.err)
	form.path, form.shownPath, form.over = msg.cfg.Path, m.shownDir(msg.cfg.Path), msg.over
	m.overlay = form

	return m, nil
}

// value is a setting's value: as edited, or as read.
func (f settingsForm) value(path string) any {
	if edited, ok := f.edits[path]; ok {
		return edited
	}

	return valueAt(f.values, path)
}

// view draws the settings under their sections, the selected one's hint, and
// how a save is going.
func (f settingsForm) view(width, rows int) (string, string) {
	head := splitLines(wrap("Saves to "+f.shownPath+". "+settingsAbout, width), "")

	switch {
	case f.reading:
		return settingsTitle, "reading the configuration" + f.marks.ellipsis
	case f.readErr != nil:
		return settingsTitle, failureBlock(f.styles, f.marks, f.readErr, width)
	}

	foot := splitLines(f.footLines(width)...)
	list := f.rows(width, rows-len(head)-len(foot))

	return settingsTitle, strings.Join(append(append(head, list...), foot...), "\n")
}

// splitLines is text broken at each line break, so what it is drawn in can be
// counted.
func splitLines(texts ...string) []string {
	return strings.Split(strings.Join(texts, "\n"), "\n")
}

// footLines are the selected setting's hint, its field while it is edited,
// and the outcome of the last key.
func (f settingsForm) footLines(width int) []string {
	lines := []string{""}

	if field, ok := f.current(); ok && field.hint != "" {
		lines = append(lines, wrap(field.hint, width))
	}

	if f.editing {
		input := f.input
		input.SetWidth(max(1, width-len(input.Prompt)-1))
		lines = append(lines, "", input.View())
	}

	if f.problem != nil {
		lines = append(lines, "", failureLine(f.styles, f.marks, f.problem))
	}

	return append(lines, pinnedOutcome(f.styles, f.marks, f.send, "saving", width)...)
}

// rows draws the settings in as many lines as fit, each section headed,
// scrolled so the selected one stays in sight.
func (f settingsForm) rows(width, space int) []string {
	var (
		lines    []string
		selected int
	)

	for index, field := range f.fields {
		if index == 0 || field.section != f.fields[index-1].section {
			lines = append(lines, f.styles.strong.Render(field.section))
		}

		if index == f.selected {
			selected = len(lines)
		}

		lines = append(lines, ansi.Truncate(f.row(field, index == f.selected), width, f.marks.ellipsis))
	}

	first, last := window(selected, len(lines), space)

	return lines[first:last]
}

// row is one setting: its label and its value, or a checkbox for one that is
// on or off, marked when it was edited.
func (f settingsForm) row(field setting, selected bool) string {
	edited := ""
	if _, ok := f.edits[field.path]; ok {
		edited = "  (edited)"
	}

	if field.kind == settingToggle {
		on, _ := f.value(field.path).(bool)

		return f.marks.marker(selected) + f.marks.checkbox(on) + field.label + edited
	}

	return f.marks.marker(selected) + fmt.Sprintf("%-15s %s", field.label, f.shown(field)) + edited
}

// current is the selected setting, or false before there are any.
func (f settingsForm) current() (setting, bool) {
	if f.selected < 0 || f.selected >= len(f.fields) {
		return setting{}, false
	}

	return f.fields[f.selected], true
}

// failed pins a refused save in the form, so it is read before anything else.
func (f settingsForm) failed(err error) settingsForm {
	f.send = f.send.failed(err)

	return f
}

// pasted types a paste into the field being edited, as typing it would.
func (f settingsForm) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if !f.editing || f.send.sending {
		return m, nil
	}

	f.input, _ = f.input.Update(paste)
	m.overlay = f

	return m, nil
}

// footer offers what the form can do now: keep or back out of a field being
// edited; otherwise move, change the selected setting, save, reload after a
// refusal, and close.
func (f settingsForm) footer(keys keyMap) []key.Binding {
	switch {
	case f.send.sending:
		return []key.Binding{keys.interrupt}
	case f.editing:
		return []key.Binding{relabel(keys.confirm, "keep"), relabel(keys.closeOverlay, escBack)}
	case f.reading:
		return []key.Binding{relabel(keys.closeOverlay, escClose)}
	case f.readErr != nil:
		return []key.Binding{relabel(keys.refresh, "try again"), relabel(keys.closeOverlay, escClose)}
	}

	offered := []key.Binding{keys.up, keys.down}
	if field, ok := f.current(); ok {
		offered = append(offered, relabel(keys.confirm, field.verb()))
	}

	offered = append(offered, keys.saveSettings)
	if errors.Is(f.send.err, errSettingsChanged) {
		offered = append(offered, relabel(keys.refresh, "reload"))
	}

	leave := escClose
	if len(f.edits) > 0 {
		leave = escDiscard
	}

	return append(offered, relabel(keys.closeOverlay, leave))
}

// handleKey answers a key while Settings has the keyboard.
func (f settingsForm) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case f.send.sending:
		return m, nil
	case f.editing:
		return f.editKey(m, msg)
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.refresh) && (f.readErr != nil || errors.Is(f.send.err, errSettingsChanged)):
		return m.openSettings()
	case f.reading || f.readErr != nil:
		return m, nil
	case key.Matches(msg, m.keys.saveSettings):
		return f.save(m)
	}

	f.problem, f.send = nil, sendState{}
	m.overlay = f.changeKey(m.keys, msg)

	return m, nil
}

// changeKey moves through the settings or changes the selected one.
func (f settingsForm) changeKey(keys keyMap, msg tea.KeyPressMsg) settingsForm {
	switch {
	case key.Matches(msg, keys.up):
		f.selected = max(0, f.selected-1)

		return f
	case key.Matches(msg, keys.down):
		f.selected = min(len(f.fields)-1, f.selected+1)

		return f
	}

	field, _ := f.current()

	switch field.kind {
	case settingToggle:
		return f.toggled(keys, msg, field)
	case settingChoice:
		return f.chosen(keys, msg, field)
	case settingText, settingURL, settingSecret, settingCount, settingList:
		return f.startEditing(keys, msg, field)
	}

	return f
}

// toggled turns a setting that is on or off, on enter or space.
func (f settingsForm) toggled(keys keyMap, msg tea.KeyPressMsg, field setting) settingsForm {
	if !key.Matches(msg, keys.confirm, keys.toggleOption) {
		return f
	}

	on, _ := f.value(field.path).(bool)

	return f.with(field.path, !on)
}

// chosen moves a choice on, on enter or →, or back, on ←.
func (f settingsForm) chosen(keys keyMap, msg tea.KeyPressMsg, field setting) settingsForm {
	switch {
	case key.Matches(msg, keys.confirm, keys.cycleRight):
		return f.with(field.path, field.cycled(f.text(field), 1))
	case key.Matches(msg, keys.cycleLeft):
		return f.with(field.path, field.cycled(f.text(field), -1))
	default:
		return f
	}
}

// startEditing opens a setting's field on enter: a credential's empty and
// echoing nothing, so what is typed for it never reaches the screen.
func (f settingsForm) startEditing(keys keyMap, msg tea.KeyPressMsg, field setting) settingsForm {
	if !key.Matches(msg, keys.confirm) {
		return f
	}

	f.editing, f.input = true, newInput(f.editedText(field))
	if field.kind == settingSecret {
		f.input.EchoMode = textinput.EchoNone
	}

	return f
}

// text is a setting's value as text.
func (f settingsForm) text(field setting) string {
	text, _ := f.value(field.path).(string)

	return text
}

// with is the form with a setting's value edited.
func (f settingsForm) with(path string, value any) settingsForm {
	f.edits = maps.Clone(f.edits)
	if f.edits == nil {
		f.edits = map[string]any{}
	}

	f.edits[path] = value

	return f
}

// editKey answers a key while a setting's field is being edited: enter keeps
// what was typed, esc backs out, and every other key types.
func (f settingsForm) editKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		f.editing, f.problem = false, nil
	case key.Matches(msg, m.keys.confirm):
		f = f.kept()
	default:
		f.input, _ = f.input.Update(msg)
		f.problem = nil
	}

	m.overlay = f

	return m, nil
}

// kept is the form with what was typed kept as the setting's value, or with
// why it cannot be.
func (f settingsForm) kept() settingsForm {
	field, _ := f.current()

	value, changes, err := typedValue(field, f.input.Value())
	if err != nil {
		f.problem = err

		return f
	}

	f.editing = false
	if changes {
		f = f.with(field.path, value)
	}

	return f
}

// save checks the edited configuration as the web's save is checked, then
// writes it over the read, or under a dry run says it would have.
func (f settingsForm) save(m Model) (Model, tea.Cmd) {
	cfg, err := f.checked()
	if err != nil {
		f.problem = err
		m.overlay = f

		return m, nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would save " + m.shownDir(f.path)), nil
	}

	f.problem, f.send = nil, starting()
	m.overlay = f
	write, over, path := m.deps.Settings.Save, f.over, f.path

	return m, func() tea.Msg {
		_, _, err := write(cfg, over)

		return settingsSaved{path: path, err: err}
	}
}

// checked is the edited configuration, held to the standard a file on disk
// is, and to the keymap the interface starts on.
func (f settingsForm) checked() (config.Config, error) {
	edited, err := f.edited()
	if err != nil {
		return config.Config{}, err
	}

	cfg, err := config.Parse(bytes.NewReader(edited))
	if err != nil {
		reason := strings.TrimPrefix(err.Error(), config.ErrInvalid.Error()+": ")

		return config.Config{}, fmt.Errorf("%w: %s", errSettingsInvalid, strings.ReplaceAll(reason, "\n", "; "))
	}

	err = CheckKeys(cfg.UI.Keys)
	if err != nil {
		return config.Config{}, fmt.Errorf("%w: %w", errSettingsKeymap, err)
	}

	return cfg, nil
}

// settingsSaved is the configuration saved, or why it was not.
type settingsSaved struct {
	path string
	err  error
}

var _ applier = settingsSaved{}

// apply closes Settings saying where it saved, or keeps it open with the
// refusal: a file changed since the read offers a reload.
func (msg settingsSaved) apply(m Model) (Model, tea.Cmd) {
	switch {
	case errors.Is(msg.err, config.ErrChangedOnDisk):
		return keepOpenWith[settingsForm](m, errSettingsChanged), nil
	case errors.Is(msg.err, messaging.ErrRejected):
		return keepOpenWith[settingsForm](m, errSlackSecretsRefused), nil
	case msg.err != nil:
		return keepOpenWith[settingsForm](m, msg.err), nil
	}

	saved := m.marks.done + " saved " + m.shownDir(msg.path) + "; it applies when workflow starts again"

	return m.closeOverlay().noticed(saved), nil
}
