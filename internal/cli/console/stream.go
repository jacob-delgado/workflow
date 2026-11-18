// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"io"
	"os"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// Stream is stderr as a command says everything on it: each write passed
// straight through, until a wait's live line is drawn there, when each is
// printed above the line instead, so nothing said lands on it — a note, a
// request's debug line from another goroutine, or a line written by code that
// knows only stderr. A line written as the wait ends, once nothing is printed
// above the line any more, prints once the line is erased.
type Stream struct {
	mu    sync.Mutex
	out   io.Writer
	state streamState
	// sending counts the writes on their way above the line, which the wait's
	// end waits out while the program still takes them.
	sending sync.WaitGroup
}

// streamStage is where a stream is in a wait's live line.
type streamStage int

const (
	// streamThrough is no live line drawn: a write goes straight through.
	streamThrough streamStage = iota
	// streamDrawn is a live line drawn: a write is printed above it.
	streamDrawn
	// streamClosing is the wait over, its line not yet erased: a write is
	// held until it is.
	streamClosing
)

// streamState is a stream's stage, the program drawing while a line is
// drawn, and the lines written while it closes, in order.
type streamState struct {
	stage streamStage
	above *tea.Program
	held  []string
}

// firstFramed is the state once program has drawn a frame: drawn, from
// through; a line drawn or closing stays as it is, so the frame that erases
// a closing line does not draw it again.
func (s streamState) firstFramed(program *tea.Program) streamState {
	if s.stage == streamThrough {
		return streamState{stage: streamDrawn, above: program}
	}

	return s
}

// closing is the state once the wait is over, from any stage: a wait over
// before its first frame closes too, so that frame cannot draw a line over
// a program about to stop.
func (s streamState) closing() streamState {
	return streamState{stage: streamClosing, held: s.held}
}

// holding is the closing state with line held after the lines before it.
func (s streamState) holding(line string) streamState {
	return streamState{stage: streamClosing, held: append(s.held, line)}
}

// released is the state once the line is erased, writing straight through,
// and the lines held until then.
func (s streamState) released() (streamState, []string) {
	return streamState{stage: streamThrough}, s.held
}

var _ io.Writer = (*Stream)(nil)

// NewStream is stderr, written straight through until a wait's line is drawn.
func NewStream(stderr io.Writer) *Stream {
	return &Stream{out: stderr}
}

// Underlying is what stream writes straight through to: stream itself,
// unless it is a Stream.
func Underlying(stream io.Writer) io.Writer {
	if kept, isStream := stream.(*Stream); isStream {
		return kept.out
	}

	return stream
}

// Write writes written through, or, once a live line is drawn, prints it
// above the line, or, once the wait is over, holds it until the line is
// erased.
func (s *Stream) Write(written []byte) (int, error) {
	s.mu.Lock()

	return map[streamStage]func([]byte) (int, error){
		streamThrough: s.writeThrough,
		streamDrawn:   s.sendAbove,
		streamClosing: s.hold,
	}[s.state.stage](written)
}

// writeThrough writes written straight through, with the stream locked, and
// unlocks it.
func (s *Stream) writeThrough(written []byte) (int, error) {
	defer s.mu.Unlock()

	return s.out.Write(written) //nolint:wrapcheck // the stream's own error, passed through as it gave it
}

// sendAbove unlocks the stream, locked with a line drawn, and prints written
// above the line, counted among the writes on their way until the program
// takes it. A blank write is printed as a space, since the program prints
// nothing for an empty line.
func (s *Stream) sendAbove(written []byte) (int, error) {
	program := s.state.above

	s.sending.Add(1)
	s.mu.Unlock()

	defer s.sending.Done()

	text := strings.TrimSuffix(string(written), "\n")
	if text == "" {
		text = " "
	}

	program.Send(tea.Println(text)())

	return len(written), nil
}

// hold keeps written, with the stream locked as its line closes, for once
// the line is erased, and unlocks the stream.
func (s *Stream) hold(written []byte) (int, error) {
	defer s.mu.Unlock()

	s.state = s.state.holding(string(written))

	return len(written), nil
}

// drawnBy is the stream program draws its live line on: what it writes goes
// straight through, and from its first write, its first frame, whatever is
// written to s is printed above the line it drew. Before that frame there is
// no line to print above, and the program cannot yet say where one would be.
// Over a file, it is the file to the program too, so the program can tell
// the terminal's size.
func (s *Stream) drawnBy(program **tea.Program) io.Writer {
	drawn := liveOut{stream: s, program: program}

	file, isFile := s.out.(*os.File)
	if !isFile {
		return drawn
	}

	return liveFile{liveOut: drawn, file: file}
}

// closeAbove ends printing above the live line, the wait over: a write from
// now on is held until the line is erased, and closeAbove returns once every
// write already on its way above the line is taken, while the program still
// reads them.
func (s *Stream) closeAbove() {
	s.mu.Lock()
	s.state = s.state.closing()
	s.mu.Unlock()

	s.sending.Wait()
}

// undrawn writes straight through again, the live line gone, starting with
// the lines held as it closed, in order. A failure to print them goes
// unheard, as every write to stderr's does: there is no channel left to say
// it on, and the wait's work is done.
func (s *Stream) undrawn() {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, held := s.state.released()
	s.state = state

	_, _ = io.WriteString(s.out, strings.Join(held, ""))
}

// streamOver is stderr as a Stream: itself, or a new one over it.
func streamOver(stderr io.Writer) *Stream {
	if kept, isStream := stderr.(*Stream); isStream {
		return kept
	}

	return NewStream(stderr)
}

// liveOut is a stream's terminal as the live line's program writes it.
type liveOut struct {
	stream  *Stream
	program **tea.Program
}

var _ io.Writer = liveOut{}

// Write writes the program's frame straight through, and, from the first
// frame, has the stream print what is written to it above the line.
func (o liveOut) Write(frame []byte) (int, error) {
	o.stream.mu.Lock()
	defer o.stream.mu.Unlock()

	o.stream.state = o.stream.state.firstFramed(*o.program)

	return o.stream.out.Write(frame) //nolint:wrapcheck // the stream's own error, passed through as it gave it
}

// liveFile is liveOut over a file, the rest of it the file's, so the program
// reads it as the terminal the file may be.
type liveFile struct {
	liveOut

	file *os.File
}

var _ interface {
	io.ReadWriteCloser
	Fd() uintptr
} = liveFile{}

// Read reads the file.
func (f liveFile) Read(p []byte) (int, error) {
	return f.file.Read(p) //nolint:wrapcheck // the file's own error, passed through as the file gave it
}

// Close closes the file.
func (f liveFile) Close() error {
	return f.file.Close() //nolint:wrapcheck // the file's own error, passed through as the file gave it
}

// Fd is the file's descriptor, what tells a terminal and its size.
func (f liveFile) Fd() uintptr {
	return f.file.Fd()
}
