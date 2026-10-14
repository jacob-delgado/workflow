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
// knows only stderr.
type Stream struct {
	mu  sync.Mutex
	out io.Writer
	// above is the program whose live line is drawn, or nil while none is.
	above *tea.Program
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
// above the line. A blank write is printed as a space, since the program
// prints nothing for an empty line.
func (s *Stream) Write(written []byte) (int, error) {
	s.mu.Lock()

	program := s.above
	if program == nil {
		defer s.mu.Unlock()

		return s.out.Write(written) //nolint:wrapcheck // the stream's own error, passed through as it gave it
	}

	s.mu.Unlock()

	text := strings.TrimSuffix(string(written), "\n")
	if text == "" {
		text = " "
	}

	program.Send(tea.Println(text)())

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

// undrawn writes straight through again, the live line gone.
func (s *Stream) undrawn() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.above = nil
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

// Write writes the program's frame straight through, and has the stream print
// what is written to it above the line from now on.
func (o liveOut) Write(frame []byte) (int, error) {
	o.stream.mu.Lock()
	defer o.stream.mu.Unlock()

	o.stream.above = *o.program

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
