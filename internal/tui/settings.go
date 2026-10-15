// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

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
// per setting under its section, each credential masked. enter opens a Huh
// form for a row — its section's settings together, an entry's value, or a
// new entry — and a save writes every edit together over the read, as the
// web's does, so a file changed since is refused rather than written over.
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
	// edit is the form open over the rows, nil while none is.
	edit    *settingsEdit
	problem error
	// refusals are why settings were refused, each said under its row, and
	// at the foot while a form open over the rows hides them, until that
	// setting is edited or a save is sent. A save's refused write
	// replaces every mark before; a save Settings' own check holds back
	// replaces only the check's earlier marks, and the writes' stay; a
	// refused removal adds its marks to every mark before, its reason in place
	// of one already on a setting it names.
	refusals settingRefusals
	send     sendState
	// said is what the last removal did, until the next key.
	said string
}

var (
	_ failable[settingsForm] = settingsForm{}
	_ pasteable              = settingsForm{}
	_ relaying               = settingsForm{}
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
	head := f.head(width)

	switch {
	case f.reading:
		return settingsTitle, "reading the configuration" + kit.marks.ellipsis
	case f.readErr != nil:
		return settingsTitle, kit.failureBlock(f.readErr, width)
	}

	foot := splitLines(f.footLines(kit, width)...)
	if f.edit != nil {
		return settingsTitle, strings.Join(slices.Concat(head, []string{"", f.edit.form.View()}, foot), "\n")
	}

	list := f.rows(kit, width, rows-len(head)-len(foot))

	return settingsTitle, strings.Join(append(append(head, list...), foot...), "\n")
}

// head is what Settings says above its rows, or a form open over them: where
// it saves, and a blank line.
func (f settingsForm) head(width int) []string {
	return splitLines(wrap("Saves to "+f.shownPath+". "+settingsAbout, width), "")
}

// splitLines is text broken at each line break, so what it is drawn in can be
// counted.
func splitLines(texts ...string) []string {
	return strings.Split(strings.Join(texts, "\n"), "\n")
}

// footLines are the selected setting's hint, which a form open over the
// rows gives its fields instead, saying in its place every refusal the rows
// it hides were marked with, and the outcome of the last key.
func (f settingsForm) footLines(kit renderKit, width int) []string {
	lines := []string{""}

	if hint := f.current().hint; hint != "" && f.edit == nil {
		lines = append(lines, wrap(hint, width))
	}

	if f.edit != nil {
		lines = append(lines, f.standingLines(kit, width)...)
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
// scrolled so the selected one stays in sight, and with it as much as fits of
// why a write or Settings' own check refused it.
func (f settingsForm) rows(kit renderKit, width, space int) []string {
	// top and bottom are the selected setting's first line and its last.
	var (
		lines       []string
		top, bottom int
	)

	for index, field := range f.fields {
		if index == 0 || field.section != f.fields[index-1].section {
			lines = append(lines, kit.styles.strong.Render(field.section))
		}

		drawn := f.rowLines(kit, index, width)
		if index == f.selected {
			top, bottom = len(lines), len(lines)+len(drawn)-1
		}

		lines = append(lines, drawn...)
	}

	// However little room is left, the selected setting's own row is drawn.
	shown := max(1, space)
	first, last := window(min(bottom, top+shown-1), len(lines), shown)

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

// failed marks each setting a save's refused write names on its row, and pins
// what is left of the refusal in the form, so it is read before anything else.
func (f settingsForm) failed(err error) settingsForm {
	marked, unmarked := f.markedByWrite(err)
	marked.send = marked.send.failed(unmarked)

	return marked
}

// pasted types a paste into the field being edited, as typing it would.
func (f settingsForm) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if f.send.sending {
		return m, nil
	}

	return f.relay(m, paste)
}

// resized fits the form open over the rows, while one is, to the pane at the
// screen's new size.
func (f settingsForm) resized(m Model) (Model, tea.Cmd) {
	if f.edit == nil {
		return m, nil
	}

	return m, f.fitEdit(m)
}

// relay hands a message to the form open over the rows, while one is, and
// fits it again to the rows a refusal it now says leaves it.
func (f settingsForm) relay(m Model, msg tea.Msg) (Model, tea.Cmd) {
	if f.edit == nil {
		return m, nil
	}

	return m, tea.Batch(relayToForm(f.edit.form, msg), f.fitEdit(m))
}

// footer offers what Settings can do now: what the open form's field
// answers, keeping what it holds, and backing out of it; otherwise move,
// change the selected setting, save, reload after a refusal, and close.
func (f settingsForm) footer(keys keyMap) []key.Binding {
	switch {
	case f.send.sending:
		return []key.Binding{keys.interrupt}
	case f.edit != nil:
		return append(formKeys(f.edit.form), relabel(keys.saveSettings, settingsKeep), relabel(keys.closeOverlay, escBack))
	case f.reading:
		return []key.Binding{relabel(keys.closeOverlay, escClose)}
	case f.readErr != nil:
		return []key.Binding{relabel(keys.refresh, "try again"), relabel(keys.closeOverlay, escClose)}
	}

	offered := append([]key.Binding{keys.up, keys.down, relabel(keys.confirm, f.current().verb())},
		f.current().quickKeys(keys)...)
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
	case f.edit != nil:
		return f.editKey(m, msg)
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.refresh) && (f.readErr != nil || errors.Is(f.send.err, errSettingsChanged)):
		return m.openSettings()
	case f.reading || f.readErr != nil:
		return m, nil
	}

	f.problem, f.send, f.said = nil, sendState{}, ""

	return f.acted(m, msg)
}

// acted answers a key on the rows: save or remove write, enter opens the
// selected row's form, and any other key moves or changes in place.
func (f settingsForm) acted(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.saveSettings, m.keys.removeEntry):
		return f.written(m, msg)
	case key.Matches(msg, m.keys.confirm):
		return f.startEditing(m)
	}

	m.overlay = f.changeKey(m.keys, msg)

	return m, nil
}

// which names Settings.
func (settingsForm) which() overlayKind { return overlaySettings }

// changeKey moves through the settings, or changes the selected one where
// a key does so in place: a toggle turned, or a choice moved on.
func (f settingsForm) changeKey(keys keyMap, msg tea.KeyPressMsg) settingsForm {
	if key.Matches(msg, keys.cursorKeys()...) {
		f.selected = max(0, min(f.selected+keys.stepOf(msg), len(f.fields)-1))

		return f
	}

	field := f.current()

	if field.kind == settingToggle {
		return f.toggled(keys, msg, field)
	}

	if field.kind == settingChoice {
		return f.chosen(keys, msg, field)
	}

	return f
}

// save checks the edited configuration as the web's save is checked, then
// writes it over the read, or under a dry run says it would have. A check
// that refuses settings marks each on its row, leaving what the writes
// refused; a save sent takes every mark away, for the file's write to judge
// afresh.
func (f settingsForm) save(m Model) (Model, tea.Cmd) {
	cfg, err := f.checked()
	if err != nil {
		marked, unmarked := f.markedByCheck(err)
		marked.problem = unmarked
		m.overlay = marked

		return m, nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would save " + shownDir(m.deps, f.path)), nil
	}

	f.problem, f.refusals, f.send = nil, settingRefusals{}, starting()
	m.overlay = f
	ctx, write, over, path := m.session, m.deps.Settings.Save, f.over, f.path

	return m, func() tea.Msg {
		_, _, err := write(ctx, cfg, nil, over)

		return settingsSaved{path: path, err: err}
	}
}

// checked is the edited configuration, held to the standard a file on disk
// is, and to the keymap the interface starts on.
func (f settingsForm) checked() (config.Config, error) {
	cfg, err := f.edited()
	if err != nil {
		return config.Config{}, err
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
