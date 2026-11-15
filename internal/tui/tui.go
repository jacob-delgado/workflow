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
	"golang.org/x/term"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/lifetime"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
	"github.com/jacob-delgado/workflow/internal/tui/layout"
	"github.com/jacob-delgado/workflow/internal/tui/notice"
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
	// given is deps as New was given them, a dry run's writes held back, for
	// WithLifetime to ask again with its writes ending at its context.
	given Deps
	// session is the context every system outside the interface is asked
	// under: Run's, which ends when the program does.
	session context.Context //nolint:containedctx // tea.Cmd takes no context, so each command captures the session's
	keys    keyMap
	styles  styles
	marks   glyphs

	width, height int
	focus         pane
	mouse         bool
	dryRun        bool
	// accessibleForms asks a form's questions as plain prompts instead of
	// drawing it.
	accessibleForms bool
	// reviewsBegun counts the reviews begun: the key of CI's poll chain, so a
	// poll scheduled in one since replaced ends its chain rather than polling
	// beside the new one's. CI's round would not do: a read begun while a poll
	// waits, a refresh's, would drop the poll before it let the chain go on.
	// It lives here, not in reviewState, where each new review would reset it.
	reviewsBegun int
	// refreshed is when each pane last began loading, so switching to one
	// loaded moments ago does not spend the tracker's or the forge's requests
	// loading it again.
	refreshed [paneCount]time.Time
	// rounds counts the rounds drawn for an overlay as it opens and for a
	// command run as it starts: each answer carries the round of the one
	// that asked, so it lands only there, never in a later one of the same
	// kind, and the output of one run is never shown in another.
	rounds int
	// commitDrafts are the commit messages composed and not yet committed,
	// one per branch, each kept for the branch it was written on.
	commitDrafts commitDrafts
	// prDrafts are the pull requests composed and not yet opened, one per
	// branch, each kept for the branch it was written on, so a failed push
	// or an esc does not lose it.
	prDrafts prDrafts
	// vocab is what a change is called on this forge, pull request or merge
	// request, fixed for the session by the forge the remote points at.
	vocab reviewVocab

	// overlay takes the keyboard while it is open; nil when none is.
	overlay overlay
	// followUp is the offer waiting for the overlay open to close.
	followUp waitingOffer
	// notice is the footer's one-line report of something that just happened,
	// which the next key press clears, and the notices no pane repeats that
	// wait for it, each taking it in turn as keys clear it.
	notice notice.Footer

	// views are the Issues pane's lists, and viewIndex the one it shows now.
	views     []jira.View
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
	// paneWrites are the writes sent from a pane or a preview that keeps the
	// keyboard, and not yet answered, which a switch waits for.
	paneWrites paneWrites
	// next is where the interface was asked to go when it ends.
	next Next
	// commentDrafts are the comments written and not posted, one per issue,
	// kept for the session.
	commentDrafts commentDrafts
	hookgen       hookgenState

	// motion is what moves on the screen, and how far it has moved.
	motion motionState

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
		cfg: cfg, loadErr: loadErr, deps: outlasting(lifetime.Unending, lifetime.Unending, deps, vocab.noun), given: deps,
		keys:   newKeyMap(marks, vocab.noun, cfg.Messaging.Service(), cfg.UI.Keys),
		styles: newStyles(true), marks: marks, vocab: vocab, session: context.Background(),
		width: defaultWidth, height: defaultHeight,
		focus: paneIssues, mouse: cfg.UI.Mouse,
		views: jira.Views(cfg.Jira.Views), viewIndex: 0,
		motion: newMotion(marks.turn, marks.inFlight, deps.now(), cfg.UI.ReducedMotion()),
	}
	// Init always searches, so a list seeded from the cache is in flight.
	model = model.searchingFromCache()

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

// Run starts the interface, reading keys from in — or from the terminal the
// process runs on, when in is a file that is not one — and drawing on out, and
// blocks until the user quits, or switches directory, which Next says. The
// context cancels the program, so a caller can shut the interface down, and
// the program's session ends when it does, stopping the reads it left in
// flight. A write it sent outlasts the program, but not the context: canceling
// it stops every write still under way too, even once Run has returned, so a
// caller cancels it only to end the program's lifetime, never to tidy up after
// a quit. The program takes no signal itself, so Bubble Tea's handler cannot
// block handing it a quit the context already gave: a caller ends it on SIGINT
// or SIGTERM through the context, and ctrl+c reaches it as a key. The
// alternate screen and mouse mode are set declaratively in View, as v2 asks.
func Run(ctx context.Context, model Model, in io.Reader, out io.Writer) (Next, error) {
	session, end := context.WithCancel(ctx)
	defer end()

	options := append(keysFrom(in), tea.WithOutput(out), tea.WithContext(ctx), tea.WithoutSignalHandler())
	options = append(options, model.programOptions...)

	ended, err := tea.NewProgram(model.WithLifetime(ctx).WithContext(session), options...).Run()
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
	// Trade-off TRADE-18: only a child process with no terminal reaches a file
	// that is none.
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
		m.searchIssues(), listIssueBranches(m.session, m.cfg, m.deps, 0), loadBranch(m.session, m.deps, 0),
		loadChanges(m.session, m.deps, 0), findHooks(m.session, m.deps, 0),
		loadReviewQueue(m.session, m.deps, 0), loadAnnounces(m.session, m.deps, 0),
		m.tasks.load(m.session, m.deps, 0),
	}

	// Opened on the Repositories pane, after a switch made from it, that pane
	// is looked at from the start, its read begun, so it is read with the
	// rest.
	if m.repositories.read.Stage() == latest.StageReading {
		loads = append(loads, m.loadRepositories(m.repositories.read.Round()))
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

		return m.resized()
	case tea.KeyPressMsg:
		m.motion = m.motion.looked()
		handled, cmd := m.handleKey(msg)

		return handled.settled(cmd)
	case tea.PasteMsg:
		m.motion = m.motion.looked()
		handled, cmd := m.pasted(msg)

		return handled.settled(cmd)
	case tea.MouseMsg:
		m.motion = m.motion.looked()
		handled, cmd := m.handleMouse(msg)

		// A click can lift the search over the notice row, by focusing
		// another pane, so a notice waiting for the row takes it then.
		return handled.nextNoticeWaiting().nextFrame(cmd)
	case applier:
		applied, cmd := msg.apply(m)

		return applied.settled(cmd)
	default:
		return m.relayed(msg)
	}
}

// relayed hands a message no case above answers to the open overlay's form,
// whose own messages carry it from one field to the next; with no form open,
// it is nothing to this interface.
func (m Model) relayed(msg tea.Msg) (Model, tea.Cmd) {
	form, relays := m.overlay.(relaying)
	if !relays {
		return m, nil
	}

	relayed, cmd := form.relay(m, msg)

	return relayed.settled(cmd)
}

// resized fits the open overlay's form, when it holds one, to the screen's
// new size; otherwise a resize leaves nothing to do but draw at it.
func (m Model) resized() (Model, tea.Cmd) {
	form, relays := m.overlay.(relaying)
	if !relays {
		return m, nil
	}

	return form.resized(m)
}

// settled opens the follow-up once no overlay is open, and asks for the next
// frame while something moves. Every route through Update that can start or
// end a read ends here, so an offer made while another overlay had the
// keyboard opens the moment that overlay closes, and never on top of it.
func (m Model) settled(cmd tea.Cmd) (Model, tea.Cmd) {
	if m.overlay != nil || m.followUp == nil {
		return m.nextFrame(cmd)
	}

	open := m.followUp
	m.followUp = nil
	opened, more := open(m)

	return opened.nextFrame(tea.Batch(cmd, more))
}

// handleKey answers a key press, which clears the notice unless it only moves
// the view, and then gives the notice to one waiting for it.
func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	// A notice is cleared when the next action starts, not by moving around, so
	// looking about after a result does not erase the record of it.
	if !m.navigates(msg) {
		m.notice = m.notice.Cleared()
	}

	answered, cmd := m.answerKey(msg)

	return answered.nextNoticeWaiting(), cmd
}

// answerKey hands a key press to whatever has the keyboard, by its input
// route; ctrl+c always quits.
func (m Model) answerKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch route := m.inputRoute(); {
	case key.Matches(msg, m.keys.interrupt):
		return m.quit()
	case route == routeOverlay:
		return m.overlay.handleKey(m, msg)
	case route == routeIssueFilter:
		return m.handleIssueFilterKey(msg)
	case route == routeTaskFilter:
		return m.handleTaskFilterKey(msg)
	default:
		return m.handleGlobalKey(msg)
	}
}

// inputRoute is what takes a key or a paste: an open overlay, the help among
// them, which takes every key, q included, since a text field needs to type
// it; else an Issues filter or a Tasks search being typed, which takes every
// key, q and the digits included, until it closes; else the panes.
type inputRoute int

const (
	routePanes inputRoute = iota
	routeOverlay
	routeIssueFilter
	routeTaskFilter
)

// inputRoute is where the next key or paste goes.
func (m Model) inputRoute() inputRoute {
	if m.overlay != nil {
		return routeOverlay
	}

	return m.paneRoute()
}

// paneRoute is where a key goes among the panes, the route an open overlay
// covers: the focused pane's filter or search while it is typed, else the
// keys every pane takes.
func (m Model) paneRoute() inputRoute {
	switch {
	case m.focus == paneIssues && m.issues.filtering:
		return routeIssueFilter
	case m.focus == paneTasks && m.tasks.listing.filtering:
		return routeTaskFilter
	default:
		return routePanes
	}
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
	return m.notice.Stage() != notice.StageNone || m.noticeCovered()
}

// noticeCovered reports the notice row given over to a search or filter
// narrowing the list, which hides whatever notice there is.
func (m Model) noticeCovered() bool {
	return m.showsFilter() || m.paneRoute() == routeTaskFilter
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
