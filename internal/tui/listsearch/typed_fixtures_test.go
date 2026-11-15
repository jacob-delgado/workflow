// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package listsearch_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui/listsearch"
)

// The names of the searches a table starts from, in each stage.
const (
	fromOff    = "off"
	fromTyping = "being typed"
	fromKept   = "kept"
)

// view is what a Typed shows of itself, to compare whole.
type view struct {
	stage listsearch.Stage
	text  string
}

// viewOf is the search's stage and text.
func viewOf(search listsearch.Typed) view {
	return view{stage: search.Stage(), text: search.Text()}
}

// off is a search never begun.
func off(*testing.T) listsearch.Typed {
	return listsearch.Typed{}
}

// typing is a search being typed, with "ab" typed so far.
func typing(t *testing.T) listsearch.Typed {
	t.Helper()

	search := off(t).Begun().Typed("ab")
	if got := viewOf(search); got != (view{stage: listsearch.StageTyping, text: "ab"}) {
		t.Fatalf("a search begun and typed into is %+v; want typing ab", got)
	}

	return search
}

// typingNothing is a search begun, with nothing typed yet.
func typingNothing(t *testing.T) listsearch.Typed {
	t.Helper()

	return off(t).Begun()
}

// kept is a search kept with "ab".
func kept(t *testing.T) listsearch.Typed {
	t.Helper()

	search := typing(t).Kept()
	if got := viewOf(search); got != (view{stage: listsearch.StageKept, text: "ab"}) {
		t.Fatalf("a search kept is %+v; want kept ab", got)
	}

	return search
}
