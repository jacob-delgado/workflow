// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package lifetime_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/lifetime"
)

// errWrite is what the writes below fail with, so a test can tell the
// wrapper handed it back.
var errWrite = errors.New("the write failed")

// answered is what the writes below that answer a value answer.
const answered = "answered"

// outlivedKey is where a recording's context carries the name it was made
// for, so a write can tell whose context it was asked under.
type outlivedKey struct{}

// recording is an Outliving that keeps the name of each write it made a
// context for and counts the releases, safe for a release that comes from a
// goroutine of its own. Each context it makes keeps its asker's values and
// not its end, as an Outliving's must, so a wrapper that watched the context
// it made in place of its asker's would never see its asker leave.
type recording struct {
	lock     sync.Mutex
	names    []string
	released int
	// releasedOnce is closed at the first release.
	releasedOnce chan struct{}
}

// newRecording is a recording that has made no context yet.
func newRecording() *recording {
	return &recording{releasedOnce: make(chan struct{})}
}

// outlive is the recording as an Outliving.
func (r *recording) outlive(ctx context.Context, name string) (context.Context, context.CancelFunc) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.names = append(r.names, name)

	return context.WithValue(context.WithoutCancel(ctx), outlivedKey{}, name), func() {
		r.lock.Lock()
		defer r.lock.Unlock()

		r.released++
		if r.released == 1 {
			close(r.releasedOnce)
		}
	}
}

// releases is how many times a context the recording made was released.
func (r *recording) releases() int {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.released
}

// made is the names the recording made a context for.
func (r *recording) made() []string {
	r.lock.Lock()
	defer r.lock.Unlock()

	return slices.Clone(r.names)
}

// asking is what a write was asked: whose context it was asked under, how
// many releases had come by then, and its arguments.
type asking struct {
	under    any
	released int
	args     []string
}

// write is a write of any shape seen through one signature: its arguments
// after the context, and its answer beside its error.
type write func(ctx context.Context, args ...string) (string, error)

// wrapped is a wrapper's write, called with the arguments its case passes.
type wrapped func(ctx context.Context) (string, error)

// outlastingCases are the wrappers of each shape, each over a write in the
// one signature above: the arguments its case passes it, and the answer it
// hands back.
func outlastingCases() map[string]struct {
	wrap       func(lifetime.Outliving, write) wrapped
	wantArgs   []string
	wantAnswer string
} {
	type outlastingCase = struct {
		wrap       func(lifetime.Outliving, write) wrapped
		wantArgs   []string
		wantAnswer string
	}

	return map[string]outlastingCase{
		"Outlasting0": {wrap: func(outlive lifetime.Outliving, inner write) wrapped {
			outer := lifetime.Outlasting0(outlive, pushing, func(ctx context.Context) error {
				_, err := inner(ctx)

				return err
			})

			return func(ctx context.Context) (string, error) { return "", outer(ctx) }
		}},
		"Outlasting1": {wantArgs: []string{"a"}, wrap: func(outlive lifetime.Outliving, inner write) wrapped {
			outer := lifetime.Outlasting1(outlive, pushing, func(ctx context.Context, first string) error {
				_, err := inner(ctx, first)

				return err
			})

			return func(ctx context.Context) (string, error) { return "", outer(ctx, "a") }
		}},
		"Outlasting2": {wantArgs: []string{"a", "b"}, wrap: func(outlive lifetime.Outliving, inner write) wrapped {
			outer := lifetime.Outlasting2(outlive, pushing, func(ctx context.Context, first, second string) error {
				_, err := inner(ctx, first, second)

				return err
			})

			return func(ctx context.Context) (string, error) { return "", outer(ctx, "a", "b") }
		}},
		"Outlasting3": {wantArgs: []string{"a", "b", "c"}, wrap: func(outlive lifetime.Outliving, inner write) wrapped {
			outer := lifetime.Outlasting3(outlive, pushing, func(ctx context.Context, first, second, third string) error {
				_, err := inner(ctx, first, second, third)

				return err
			})

			return func(ctx context.Context) (string, error) { return "", outer(ctx, "a", "b", "c") }
		}},
		"OutlastingAnswer0": {wantAnswer: answered, wrap: func(outlive lifetime.Outliving, inner write) wrapped {
			outer := lifetime.OutlastingAnswer0(outlive, pushing, func(ctx context.Context) (string, error) {
				return inner(ctx)
			})

			return func(ctx context.Context) (string, error) { return outer(ctx) }
		}},
		"OutlastingAnswer1": {
			wantArgs: []string{"a"}, wantAnswer: answered,
			wrap: func(outlive lifetime.Outliving, inner write) wrapped {
				outer := lifetime.OutlastingAnswer1(outlive, pushing, func(ctx context.Context, first string) (string, error) {
					return inner(ctx, first)
				})

				return func(ctx context.Context) (string, error) { return outer(ctx, "a") }
			},
		},
		"OutlastingAnswer2": {
			wantArgs: []string{"a", "b"}, wantAnswer: answered,
			wrap: func(outlive lifetime.Outliving, inner write) wrapped {
				outer := lifetime.OutlastingAnswer2(outlive, pushing,
					func(ctx context.Context, first, second string) (string, error) { return inner(ctx, first, second) })

				return func(ctx context.Context) (string, error) { return outer(ctx, "a", "b") }
			},
		},
		"OutlastingAnswer3": {
			wantArgs: []string{"a", "b", "c"}, wantAnswer: answered,
			wrap: func(outlive lifetime.Outliving, inner write) wrapped {
				outer := lifetime.OutlastingAnswer3(outlive, pushing,
					func(ctx context.Context, first, second, third string) (string, error) {
						return inner(ctx, first, second, third)
					})

				return func(ctx context.Context) (string, error) { return outer(ctx, "a", "b", "c") }
			},
		},
	}
}

func TestOutlastingAsksAWriteUnderTheContextItOutlivesItsCallerWithAndReleasesItAfter(t *testing.T) {
	t.Parallel()

	for name, tt := range outlastingCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			record := newRecording()

			var asked asking

			call := tt.wrap(record.outlive, func(ctx context.Context, args ...string) (string, error) {
				asked = asking{under: ctx.Value(outlivedKey{}), released: record.releases(), args: args}

				return answered, errWrite
			})

			// Act
			answer, err := call(t.Context())

			// Assert
			if asked.under != pushing || asked.released != 0 || !slices.Equal(asked.args, tt.wantArgs) {
				t.Errorf("the write was asked under %v, after %d releases, with %q; want under %q's context, "+
					"before its release, with %q", asked.under, asked.released, asked.args, pushing, tt.wantArgs)
			}

			if answer != tt.wantAnswer || !errors.Is(err, errWrite) {
				t.Errorf("the wrapper answered %q, %v; want the write's %q, %v", answer, err, tt.wantAnswer, errWrite)
			}

			if made, releases := record.made(), record.releases(); !slices.Equal(made, []string{pushing}) || releases != 1 {
				t.Errorf("made contexts for %q and released %d; want one for %q, released once", made, releases, pushing)
			}
		})
	}
}

func TestOutlastingANilWriteIsNil(t *testing.T) {
	t.Parallel()

	cases := map[string]func() bool{
		"Outlasting0": func() bool { return lifetime.Outlasting0(lifetime.Unending, pushing, nil) == nil },
		"Outlasting1": func() bool { return lifetime.Outlasting1[string](lifetime.Unending, pushing, nil) == nil },
		"Outlasting2": func() bool { return lifetime.Outlasting2[string, int](lifetime.Unending, pushing, nil) == nil },
		"Outlasting3": func() bool {
			return lifetime.Outlasting3[string, int, bool](lifetime.Unending, pushing, nil) == nil
		},
		"OutlastingAnswer0": func() bool {
			return lifetime.OutlastingAnswer0[string](lifetime.Unending, pushing, nil) == nil
		},
		"OutlastingAnswer1": func() bool {
			return lifetime.OutlastingAnswer1[string, int](lifetime.Unending, pushing, nil) == nil
		},
		"OutlastingAnswer2": func() bool {
			return lifetime.OutlastingAnswer2[string, int, bool](lifetime.Unending, pushing, nil) == nil
		},
		"OutlastingAnswer3": func() bool {
			return lifetime.OutlastingAnswer3[string, int, bool, byte](lifetime.Unending, pushing, nil) == nil
		},
		"OutlastingRun0": func() bool { return lifetime.OutlastingRun0(lifetime.Unending, pushing, nil) == nil },
		"OutlastingRun1": func() bool { return lifetime.OutlastingRun1[string](lifetime.Unending, pushing, nil) == nil },
	}

	for name, wrapsNil := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			stayedNil := wrapsNil()

			// Assert
			if !stayedNil {
				t.Errorf("%s over a nil write = a write, want nil, so a seam unavailable here stays unavailable", name)
			}
		})
	}
}
