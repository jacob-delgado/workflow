// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// Why no move to a status is made, for a surface that answers a request for
// one rather than offering it.
var (
	// ErrNoReviewStatus is a configuration with no jira.review_status to move to.
	ErrNoReviewStatus = errors.New("no review status is configured (jira.review_status)")
	// ErrNoTransition is an issue the tracker offers no move to the status
	// for, from where it stands — or no issue, or no tracker to ask.
	ErrNoTransition = errors.New("no move to that status is offered")
	// ErrNeedsFields is a move that asks for fields, which a move made without
	// a form cannot fill.
	ErrNeedsFields = errors.New("the move asks for fields")
)

// NoTransitionError is a move to Status the tracker does not offer Key from
// where it stands, naming the statuses it does offer.
type NoTransitionError struct {
	Key     jira.Key
	Status  string
	Offered []string
}

var _ error = NoTransitionError{}

func (e NoTransitionError) Error() string {
	refused := fmt.Sprintf("no move of %s to %q is offered from where it stands", e.Key, e.Status)
	if len(e.Offered) == 0 {
		return refused
	}

	return refused + "; it can move to " + strings.Join(e.Offered, ", ")
}

// Unwrap is ErrNoTransition, the kind of refusal this is.
func (e NoTransitionError) Unwrap() error { return ErrNoTransition }

// NeedsFieldsError is a move of Key that asks for fields, naming them.
type NeedsFieldsError struct {
	Key  jira.Key
	Move jira.Transition
}

var _ error = NeedsFieldsError{}

func (e NeedsFieldsError) Error() string {
	names := make([]string, 0, len(e.Move.Fields))
	for _, field := range e.Move.Fields {
		names = append(names, cmp.Or(field.Name, field.ID))
	}

	return fmt.Sprintf("moving %s to %s asks for %s", e.Key, e.Move.ToStatus, strings.Join(names, ", "))
}

// Unwrap is ErrNeedsFields, the kind of refusal this is.
func (e NeedsFieldsError) Unwrap() error { return ErrNeedsFields }

// TransitionTo is the move of key to the status of that name, matched without
// regard to case, as the tracker offers it from where the issue stands under
// ctx. A status not offered is a NoTransitionError naming those that are; a
// move that asks for fields is a NeedsFieldsError naming them; a tracker that
// cannot be read is its own error, saying what was read.
func TransitionTo(
	ctx context.Context, transitions func(context.Context, jira.Key) ([]jira.Transition, error), key jira.Key,
	status string,
) (jira.Transition, error) {
	moves, err := transitions(ctx, key)
	if err != nil {
		return jira.Transition{}, fmt.Errorf("reading the transitions of %s: %w", key, err)
	}

	index, found := jira.FindTransition(moves, status)
	if !found {
		return jira.Transition{}, NoTransitionError{Key: key, Status: status, Offered: offeredStatuses(moves)}
	}

	if len(moves[index].Fields) > 0 {
		return jira.Transition{}, NeedsFieldsError{Key: key, Move: moves[index]}
	}

	return moves[index], nil
}

// offeredStatuses are the statuses moves lead to, in the order offered.
func offeredStatuses(moves []jira.Transition) []string {
	statuses := make([]string, 0, len(moves))
	for _, move := range moves {
		statuses = append(statuses, move.ToStatus)
	}

	return statuses
}

// ReviewTransition is the move to the configured review status for an issue
// whose pull request is now open, and whether there is one worth offering: the
// status is configured, the branch names an issue, Jira offers a move to that
// status, and the move needs no fields. A tracker that cannot be read offers
// nothing — the pull request is already open. The tracker is asked under ctx.
func ReviewTransition(
	ctx context.Context, transitions func(context.Context, jira.Key) ([]jira.Transition, error), key jira.Key,
	status string,
) (jira.Transition, bool) {
	move, err := FindReviewTransition(ctx, transitions, key, status)

	return move, err == nil
}

// FindReviewTransition is ReviewTransition saying why there is no move: no
// review status configured, no issue or tracker to ask, or TransitionTo's
// refusal. With no status configured the tracker is not asked at all;
// otherwise it is asked under ctx.
func FindReviewTransition(
	ctx context.Context, transitions func(context.Context, jira.Key) ([]jira.Transition, error), key jira.Key,
	status string,
) (jira.Transition, error) {
	if status == "" {
		return jira.Transition{}, ErrNoReviewStatus
	}

	if key == "" || transitions == nil {
		return jira.Transition{}, ErrNoTransition
	}

	return TransitionTo(ctx, transitions, key, status)
}
