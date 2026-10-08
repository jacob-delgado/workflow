// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package tui renders workflow's terminal interface: a rail of panes down the
// left, a detail pane beside it, the loop's stages across the top and the keys
// along the bottom.
package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"golang.org/x/term"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui/layout"
)

// defaultWidth and defaultHeight stand in until the terminal reports its real
// size, which Bubble Tea sends straight after start.
const (
	defaultWidth  = 100
	defaultHeight = 30
)

// Model satisfies tea.Model with value receivers, so the assertion uses a value
// rather than a pointer — and every method on it stays a value receiver, which
// recvcheck enforces.
var _ tea.Model = Model{}

// Model is the interface's state.
type Model struct {
	cfg     config.Config
	loadErr error
	deps    Deps
	keys    keyMap
	styles  styles
	marks   glyphs

	width, height int
	focus         pane
	mouse         bool
	dryRun        bool
	// runs counts the programs started, so the output of one run is never
	// shown in another.
	runs int
	// reviewsBegun counts the reviews begun, so a CI poll scheduled in one since
	// replaced ends its chain rather than polling beside the new one's. It lives
	// here, not in reviewState, where each new review's literal would reset it.
	reviewsBegun int
	// refreshed is when each pane last began loading, so switching to one
	// loaded moments ago does not spend the tracker's or the forge's requests
	// loading it again.
	refreshed [paneCount]time.Time
	// detailReads counts the issue reads started, so the answer to one a later
	// read superseded is dropped rather than shown over the later one's.
	detailReads int
	// overlaysOpened counts the overlays opened whose reads answer later — the
	// announcement preview, People and groups, the pull request composer — so
	// an answer lands only in the one it was started for, never in a later
	// one of the same kind.
	overlaysOpened int
	// draft is the commit message last composed and not yet committed.
	draft commitDraft
	// prDraft is the pull request last composed and not yet opened, kept per
	// branch so a failed push or an esc does not lose it.
	prDraft prDraft
	// vocab is what a change is called on this forge, pull request or merge
	// request, fixed for the session by the forge the remote points at.
	vocab reviewVocab

	// overlay takes the keyboard while it is open; nil when none is.
	overlay overlay
	// followUp is an overlay to open once the one that is open closes: an offer
	// made at a moment the interface is already asking something else — the
	// status picker after a branch, the issue linker after a pull request. Nil
	// when nothing waits.
	followUp func(Model) (Model, tea.Cmd)
	// notice is the footer's one-line report of something that just happened.
	// The next key press clears it.
	notice notice

	// views are the named issue lists the Issues pane moves between, and
	// viewIndex is the one it shows now.
	views     []issueView
	viewIndex int

	issues      issueList
	detail      issueDetail
	branch      branchState
	changes     changeList
	diff        diffState
	review      reviewState
	messaging   messagingState
	reviewQueue reviewQueueState
	tasks       tasksState
	summary     summaryState
	// repositories is the Repositories pane's favorites and cursor.
	repositories repositoriesState
	// next is where the interface was asked to go when it ends.
	next Next
	// commentDrafts are the comments written and not posted, one per issue,
	// kept for the session.
	commentDrafts commentDrafts
	hookgen       hookgenState

	// programOptions are what Run adds to the program it starts, so a choice
	// made on the model, such as dropping color, reaches the terminal too.
	programOptions []tea.ProgramOption
}

// New builds the interface for a configuration, the error if any from loading
// it, and what it may ask of the world outside.
func New(cfg config.Config, loadErr error, deps Deps) Model {
	marks := unicodeGlyphs()
	if cfg.UI.ASCII {
		marks = asciiGlyphs()
	}

	vocab := forgeVocab(deps.Forge.Kind)

	model := Model{
		cfg: cfg, loadErr: loadErr, deps: deps,
		keys:   newKeyMap(marks, vocab.noun, cfg.Messaging.Service(), cfg.UI.Keys),
		styles: newStyles(true), marks: marks, vocab: vocab,
		width: defaultWidth, height: defaultHeight,
		focus: paneIssues, mouse: cfg.UI.Mouse,
		views: issueViews(cfg.Jira.Views), viewIndex: 0,
	}
	// Init always searches, so a list seeded from the cache is in flight.
	model.issues = model.seededIssues()
	model = model.searching()

	// Init loads every pane but the Summary and Repositories, so each starts
	// fresh; those two are read the first time they are looked at, which
	// costs a startup that never looks at them nothing.
	for index := range model.refreshed {
		if pane(index) != paneSummary && pane(index) != paneRepositories {
			model.refreshed[index] = deps.now()
		}
	}

	return model
}

// WithDryRun is the interface holding back every write — to Jira, the forge,
// the messaging service, Taskwarrior, git and files — and saying instead what
// it would have done. Reads stay live, so what it says is about the real state
// of things. Of the store it keeps only the kept reads — whom a post would tag,
// and the favorites — which --dry-run binds to kept.db, read read-only, and it
// reads nothing from the cache.
func (m Model) WithDryRun() Model {
	m.deps = heldBack(m.deps)
	m.dryRun = true

	return m
}

// WithoutColor draws no hue while keeping the bold, faint and reverse-video
// cursor that carry meaning without it: its styles hold no hue, and Run forces
// the ANSI profile, which still draws those attributes.
func (m Model) WithoutColor() Model {
	m.styles = newStyles(false)
	m.programOptions = []tea.ProgramOption{tea.WithColorProfile(colorprofile.ANSI)}

	return m
}

// Run starts the interface, reading keys from in — or from the terminal the
// process runs on, when in is a file that is not one — and drawing on out, and
// blocks until the user quits, or switches directory, which Next says. The
// context cancels the program, so a caller can shut the interface down. The
// alternate screen and mouse mode are set declaratively in View, as v2 asks.
func Run(ctx context.Context, model Model, in io.Reader, out io.Writer) (Next, error) {
	options := append(keysFrom(in), tea.WithOutput(out), tea.WithContext(ctx))
	options = append(options, model.programOptions...)

	ended, err := tea.NewProgram(model, options...).Run()
	if err != nil {
		return Next{}, fmt.Errorf("running the interface: %w", err)
	}

	final, _ := ended.(Model)

	return final.next, nil
}

// keysFrom is where the program reads keys: from input, unless input is a
// file that is not a terminal — the pipe or /dev/null standard input may be —
// which holds no keys. Then it names no input, and Bubble Tea opens the
// terminal the process runs on, as it does by default: `workflow < /dev/null`
// still takes keys from the person at it, and an editor the interface hands
// off to gets that terminal rather than the pipe.
func keysFrom(input io.Reader) []tea.ProgramOption {
	file, isFile := input.(*os.File)
	if isFile && !term.IsTerminal(int(file.Fd())) {
		return nil
	}

	return []tea.ProgramOption{tea.WithInput(input)}
}

// Init implements tea.Model: it starts every load the panes need. Each runs
// inside a command, which Bubble Tea executes off the update loop, so a slow
// service never freezes the screen — and each pane fills in, or fails, on its
// own.
func (m Model) Init() tea.Cmd {
	loads := []tea.Cmd{
		m.searchIssues(), listIssueBranches(m.cfg, m.deps), loadBranch(m.deps), loadChanges(m.deps), findHooks(m.deps),
		loadReviewQueue(m.deps), loadAnnounces(m.deps), m.tasks.load(m.deps),
	}

	// Opened on the Repositories pane, after a switch made from it, that pane
	// is looked at from the start, so it is read with the rest.
	if m.focus == paneRepositories {
		loads = append(loads, m.loadRepositories())
	}

	return tea.Batch(loads...)
}

// Update implements tea.Model. Every load and result is an applier, which knows
// what it changes, so this only routes. A key or a result may close an overlay,
// so both settle, opening whatever follow-up waited on it.
//
//nolint:ireturn // tea.Model is the return type bubbletea's interface requires
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

		return m, nil
	case tea.KeyPressMsg:
		handled, cmd := m.handleKey(msg)

		return handled.settled(cmd)
	case tea.PasteMsg:
		handled, cmd := m.pasted(msg)

		return handled.settled(cmd)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case applier:
		applied, cmd := msg.apply(m)

		return applied.settled(cmd)
	default:
		return m, nil
	}
}

// settled opens the follow-up once no overlay is open. Every route through
// Update ends here, so an offer made while another overlay had the keyboard
// opens the moment that overlay closes, and never on top of it.
func (m Model) settled(cmd tea.Cmd) (Model, tea.Cmd) {
	if m.overlay != nil || m.followUp == nil {
		return m, cmd
	}

	open := m.followUp
	m.followUp = nil
	opened, more := open(m)

	return opened, tea.Batch(cmd, more)
}

// handleKey answers a key press. ctrl+c always quits. Otherwise an open overlay
// — the help among them — has the keyboard, including q, which a text field
// needs to type; then the Issues filter while it is open; then the keys that
// work everywhere, which fall through to the focused pane's own.
func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	// A notice is cleared when the next action starts, not by moving around, so
	// looking about after a result does not erase the record of it.
	if !m.navigates(msg) {
		m.notice = notice{}
	}

	switch {
	case key.Matches(msg, m.keys.interrupt):
		return m, tea.Quit
	case m.overlay != nil:
		return m.overlay.handleKey(m, msg)
	case m.filteringIssues():
		return m.handleIssueFilterKey(msg)
	case m.filteringTasks():
		return m.handleTaskFilterKey(msg)
	default:
		return m.handleGlobalKey(msg)
	}
}

// filteringIssues reports that the Issues pane is capturing keystrokes into its
// filter, which takes every key — q and the digits included — until it closes.
func (m Model) filteringIssues() bool {
	return m.focus == paneIssues && m.issues.filtering
}

// quitOrGuard quits, unless a post is waiting for CI, in which case it asks
// first: quitting would lose the post without a word.
func (m Model) quitOrGuard() (Model, tea.Cmd) {
	if m.messaging.pending.waiting() {
		m.overlay = quitGuard{}

		return m, nil
	}

	return m, tea.Quit
}

// handleGlobalKey answers the keys that work in every pane, and hands the rest
// to the focused one.
func (m Model) handleGlobalKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.quit):
		return m.quitOrGuard()
	case key.Matches(msg, m.keys.toggleHelp):
		return m.openHelp()
	case key.Matches(msg, m.keys.next):
		return m.switchTo(around(m.focus, paneCount).next())
	case key.Matches(msg, m.keys.previous):
		return m.switchTo(around(m.focus, paneCount).prev())
	case key.Matches(msg, m.keys.jump):
		// CheckKeys refuses to move jump-to-pane, so the binding only matches the
		// pane digits, 1 through paneCount, and the digit is always a valid pane.
		return m.switchTo(pane(msg.String()[0] - '1'))
	case key.Matches(msg, m.keys.toggleMouse):
		return m.toggleMouse()
	case key.Matches(msg, m.keys.first, m.keys.last, m.keys.scrollDown, m.keys.scrollUp):
		return m.handleViewKey(msg)
	default:
		return behaviorOf(m.focus).handle(m, msg)
	}
}

// handleViewKey answers the keys that move the focused pane's view on every
// pane: the scroll keys page its detail, and home and end move its list to the
// first or last row or, on a pane with no list, its detail to the top or the
// bottom.
func (m Model) handleViewKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.scrollDown):
		return m.scrollDetail(m.halfPage()), nil
	case key.Matches(msg, m.keys.scrollUp):
		return m.scrollDetail(-m.halfPage()), nil
	}

	if move := behaviorOf(m.focus).move; move != nil {
		return move(m, m.keys.stepOf(msg))
	}

	return m.scrollDetail(m.keys.stepOf(msg)), nil
}

// halfPage is how far a scroll key moves the detail.
func (m Model) halfPage() int {
	return max(1, m.detailRows()/2) //nolint:mnd // half, as in half a page
}

// scrollDetail moves the focused pane's detail by delta lines from where it is
// drawn, and clamps the result to the content. A stored offset can sit past the
// end — a taller terminal, or a body that shrank while its pane was away, leaves
// it there — so it is clamped before it moves as well as after: otherwise one
// scroll-up spends itself undoing offsets the content no longer has, and the
// view does not move.
func (m Model) scrollDetail(delta int) Model {
	lines, rows := m.detailLines(), m.detailRows()
	offset := behaviorOf(m.focus).scroll(&m)
	from := firstShown(lines, *offset, rows)
	*offset = firstShown(lines, from+delta, rows)

	return m
}

// focusOn moves focus to a pane, which comes back scrolled where it was left.
// Leaving the Issues pane cancels any filter, so it never narrows a list you
// cannot see.
func (m Model) focusOn(target pane) Model {
	m.focus = target
	m.issues = m.issues.clearFilter()

	return m.withoutTaskFilter()
}

// toggleMouse gives the terminal its own click-drag selection back, or takes it
// again. Capture is on by default and breaks copying text out of the screen,
// which is why this exists.
func (m Model) toggleMouse() (Model, tea.Cmd) {
	m.mouse = !m.mouse
	// v2 reads the mouse mode from View every render, so flipping the flag is the
	// whole change: the next render turns capture on or off.
	if m.mouse {
		return m.noticed("mouse on: clicks focus panes and pick rows"), nil
	}

	return m.noticed("mouse off: your terminal selects text again"), nil
}

// navigates reports a key that only moves the view, which keeps a notice rather
// than clearing it.
func (m Model) navigates(msg tea.KeyPressMsg) bool {
	return key.Matches(msg, m.keys.up, m.keys.down, m.keys.first, m.keys.last, m.keys.scrollUp, m.keys.scrollDown,
		m.keys.next, m.keys.previous, m.keys.jump, m.keys.toggleHelp)
}

// minNoticeHeight is the shortest terminal that gives a notice its own row: a
// spine, a body row, the notice and the footer.
const minNoticeHeight = 4

// showsNotice reports a notice — or the issue filter — that has room for its own
// row above the hints: a terminal tall enough, where the row does not take the
// focused pane under the rows it needs to be read.
func (m Model) showsNotice() bool {
	return m.hasNotice() && m.height >= minNoticeHeight &&
		layout.NoticeKeepsFocus(m.terminal(), m.railShape())
}

// hasNotice reports something for the notice row to say: a notice, or a search
// or filter narrowing the list.
func (m Model) hasNotice() bool {
	return m.notice.text != "" || m.showsFilter() || m.filteringTasks()
}

// showsFilter reports that the Issues pane's filter should be shown on its own
// row, which is whenever it is being typed or it or the places still narrow
// the list.
func (m Model) showsFilter() bool {
	return m.focus == paneIssues && (m.issues.filtering || m.issues.filter != "" || len(m.issues.places) > 0)
}

// shape is the layout for the terminal as it is now, a row shorter when a notice
// takes one above the footer.
func (m Model) shape() layout.Layout {
	if m.showsNotice() {
		result, _ := layout.ComputeWithNotice(m.terminal(), m.railShape())

		return result
	}

	return layout.Compute(m.terminal(), m.railShape())
}

// terminal is the screen's size, as the terminal last reported it.
func (m Model) terminal() layout.Terminal {
	return layout.Terminal{Width: m.width, Height: m.height}
}

// railShape is every pane, stacked, with the focused one marked.
func (m Model) railShape() layout.Rail {
	return layout.Rail{Panes: paneCount, Focused: int(m.focus)}
}
