// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package armed

import "errors"

var (
	// ErrNoCI gives up on an armed announcement whose pull request reported no
	// check within NoCIPatience: it is announced by hand.
	ErrNoCI = errors.New("no checks were reported on the pull request")
	// ErrPullChanged gives up on an armed announcement whose branch's pull
	// request is another one, or none.
	ErrPullChanged = errors.New("the branch's pull request is no longer the one announced")
	// ErrMergedFirst gives up on an armed announcement whose pull request
	// merged before its CI passed.
	ErrMergedFirst = errors.New("the pull request merged before its CI passed")
	// ErrNotListedYet is a find that does not see a pull request just opened,
	// which a forge may list only a moment later: a read to make again.
	ErrNotListedYet = errors.New("the forge does not list the pull request yet")
)
