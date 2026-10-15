// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package lifetime carries, in a context, the end of what a piece of work is
// done for: the web server serving it, or the command running it. Work that
// goes on when its asker leaves (a read others share, a refresh that must not
// stop halfway) detaches from the asker through Detached, and still ends with
// that lifetime, so a server that stops, or a command a signal ends, is never
// held open by it. A context can carry an InFlight too, the work begun under
// it that has not yet ended, which a process waits for before it exits. A
// write that must outlast its asker, as the terminal's writes outlast a quit,
// is wrapped by Outlasting0 and its kin to run under the context an Outliving
// makes for it: Unending, EndingWith or UncountedEndingWith.
package lifetime

import "context"

// key is where a context carries its lifetime.
type key struct{}

// Within is ctx carrying end as the lifetime work detached under it ends
// with, in place of any it carried before.
func Within(ctx, end context.Context) context.Context {
	return context.WithValue(ctx, key{}, end)
}

// Detached is Until(ctx, the lifetime ctx carries), or, where it carries
// none, a context with ctx's values that never ends but on release.
func Detached(ctx context.Context) (context.Context, context.CancelFunc) {
	end, carried := ctx.Value(key{}).(context.Context)
	if !carried {
		return context.WithCancel(context.WithoutCancel(ctx))
	}

	return Until(ctx, end)
}

// Until is a context with ctx's values that goes on when ctx ends and ends
// when end does, with end's cause, and the function that releases it once the
// work it was made for is done.
func Until(ctx, end context.Context) (context.Context, context.CancelFunc) {
	detached, cancel := context.WithCancelCause(context.WithoutCancel(ctx))
	stop := context.AfterFunc(end, func() { cancel(context.Cause(end)) })

	return detached, func() {
		stop()
		cancel(nil)
	}
}
