// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/seams"
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
const settingsAbout = "A save reopens workflow here, so it applies at once."

// settingsForm is Settings: the configuration the web's Settings edits, a row
// per setting under its section, each credential masked. enter edits a row,
// and a save writes every edit together over the read, as the web's does, so
// a file changed since is refused rather than written over.
type settingsForm struct {
	// noun is the forge's word for a pull request, and actions the keys the
	// interface binds, which the fields are laid out with.
	noun    string
	actions []seams.KeyAction
	fields  []setting
	opened  int
	reading bool
	readErr error
	// values is the configuration as read, masked, in its JSON form, which a
	// save sends back with the edits laid over it.
	values map[string]any
	path   string
	// shownPath is path as the screen writes it, from your home.
	shownPath string
	over      config.Revision
	// edits are the values changed since the read, by setting path.
	edits    map[string]any
	selected int
	editing  bool
	// adding is the name of an entry being added, kept while its value is
	// asked; empty while its name is.
	adding  string
	input   textinput.Model
	problem error
	send    sendState
	// said is what the last removal did, until the next key.
	said string
}

var _ failable[settingsForm] = settingsForm{}

var (
	_ failable[settingsForm] = settingsForm{}
	_ pasteable              = settingsForm{}
)

// canEditSettings reports that the configuration files can be read.
func canEditSettings(deps Deps) bool {
	return deps.Settings.Read != nil
}

// openSettings opens Settings and starts reading the configuration files.
func (m Model) openSettings() (Model, tea.Cmd) {
	m, opened := m.opening()
	m.overlay = settingsForm{
		noun: m.vocab.noun, opened: opened, reading: true,
		actions: KeyActions(m.vocab.noun, m.cfg.Messaging.Service(), nil),
	}.laidOut()
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

	form.reading, form.edits = false, map[string]any{}
	// The form masks what it was handed itself, so it never holds a credential
	// whatever the seam gave it, and a credential left alone goes back masked.
	form.values, form.readErr = seed(msg.cfg.Redacted(), msg.err)
	form = form.laidOut()
	form.path, form.shownPath, form.over = msg.cfg.Path, shownDir(m.deps, msg.cfg.Path), msg.over
	m.overlay = form

	return m, nil
}

// laidOut is the form with its rows laid out for what it holds, the cursor
// kept among them.
func (f settingsForm) laidOut() settingsForm {
	f.fields = f.settings()
	f.selected = min(f.selected, len(f.fields)-1)

	return f
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
func (f settingsForm) view(kit renderKit, width, rows int) (string, string) {
	head := splitLines(wrap("Saves to "+f.shownPath+". "+settingsAbout, width), "")

	switch {
	case f.reading:
		return settingsTitle, "reading the configuration" + kit.marks.ellipsis
	case f.readErr != nil:
		return settingsTitle, kit.failureBlock(f.readErr, width)
	}

	foot := splitLines(f.footLines(kit, width)...)
	list := f.rows(kit, width, rows-len(head)-len(foot))

	return settingsTitle, strings.Join(append(append(head, list...), foot...), "\n")
}

// splitLines is text broken at each line break, so what it is drawn in can be
// counted.
func splitLines(texts ...string) []string {
	return strings.Split(strings.Join(texts, "\n"), "\n")
}

// footLines are the selected setting's hint, its field while it is edited,
// and the outcome of the last key.
func (f settingsForm) footLines(kit renderKit, width int) []string {
	lines := []string{""}

	if hint := f.current().hint; hint != "" {
		lines = append(lines, wrap(hint, width))
	}

	if f.editing {
		input := f.input
		input.SetWidth(max(1, width-len(input.Prompt)-1))
		lines = append(lines, "", input.View())
	}

	if f.problem != nil {
		lines = append(lines, "", kit.failureBlock(f.problem, width))
	}

	if f.said != "" {
		lines = append(lines, "", wrap(f.said, width))
	}

	return append(lines, kit.pinnedOutcome(f.send, "saving", width)...)
}

// rows draws the settings in as many lines as fit, each section headed,
// scrolled so the selected one stays in sight.
func (f settingsForm) rows(kit renderKit, width, space int) []string {
	var (
		lines    []string
		selected int
	)

	for index, field := range f.fields {
		if index == 0 || field.section != f.fields[index-1].section {
			lines = append(lines, kit.styles.strong.Render(field.section))
		}

		if index == f.selected {
			selected = len(lines)
		}

		lines = append(lines, ansi.Truncate(f.row(kit, field, index == f.selected), width, kit.marks.ellipsis))
	}

	first, last := window(selected, len(lines), space)

	return lines[first:last]
}

// row is one setting: its label and its value, or a checkbox for one that is
// on or off, marked when it was edited.
func (f settingsForm) row(kit renderKit, field setting, selected bool) string {
	edited := ""
	if f.isEdited(field) {
		edited = "  (edited)"
	}

	switch field.kind {
	case settingToggle:
		on, _ := f.value(field.path).(bool)

		return kit.marks.marker(selected) + checkbox(on) + field.label + edited
	case settingAdd:
		return kit.marks.marker(selected) + field.label
	case settingText, settingURL, settingSecret, settingCount, settingList, settingChoice, settingEntry:
	}

	return kit.marks.marker(selected) + fmt.Sprintf("%-15s %s", field.label, f.shown(field)) + edited
}

// current is the selected setting; the cursor never leaves the settings.
func (f settingsForm) current() setting {
	return f.fields[f.selected]
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

	offered := []key.Binding{keys.up, keys.down, relabel(keys.confirm, f.current().verb())}
	if f.removable(f.current()) {
		offered = append(offered, keys.removeEntry)
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
	case key.Matches(msg, m.keys.saveSettings, m.keys.removeEntry):
		f.problem, f.send, f.said = nil, sendState{}, ""

		return f.written(m, msg)
	}

	f.problem, f.send, f.said = nil, sendState{}, ""
	m.overlay = f.changeKey(m.keys, msg)

	return m, nil
}

// which names Settings.
func (settingsForm) which() overlayKind { return overlaySettings }

// changeKey moves through the settings or changes the selected one.
func (f settingsForm) changeKey(keys keyMap, msg tea.KeyPressMsg) settingsForm {
	if key.Matches(msg, keys.cursorKeys()...) {
		f.selected = max(0, min(f.selected+keys.stepOf(msg), len(f.fields)-1))

		return f
	}

	field := f.current()

	switch field.kind {
	case settingToggle:
		return f.toggled(keys, msg, field)
	case settingChoice:
		return f.chosen(keys, msg, field)
	case settingText, settingURL, settingSecret, settingCount, settingList, settingEntry, settingAdd:
		return f.startEditing(keys, msg, field)
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
		return m.closeOverlay().noticed("dry run: would save " + shownDir(m.deps, f.path)), nil
	}

	f.problem, f.send = nil, starting()
	m.overlay = f
	write, over, path := m.deps.Settings.Save, f.over, f.path

	return m, func() tea.Msg {
		_, _, err := write(cfg, nil, over)

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

// written answers a key that writes: save, or remove, which writes at once
// only a credential the read holds.
func (f settingsForm) written(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if key.Matches(msg, m.keys.saveSettings) {
		return f.save(m)
	}

	return f.remove(m)
}

// savedRefusal is a write's refusal in Settings' own words, where it has them.
func savedRefusal(err error) error {
	switch {
	case errors.Is(err, config.ErrChangedOnDisk):
		return errSettingsChanged
	case errors.Is(err, messaging.ErrRejected):
		return errSlackSecretsRefused
	default:
		return err
	}
}

// settingsSaved is the configuration saved, or why it was not.
type settingsSaved struct {
	path string
	err  error
}

var _ applier = settingsSaved{}

// apply closes Settings and reopens workflow with what it saved, or keeps it
// open with the refusal: a file changed since the read offers a reload.
func (msg settingsSaved) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return keepOpenWith[settingsForm](m, savedRefusal(msg.err)), nil
	}

	return m.reopenWith(reopening{path: msg.path})
}
