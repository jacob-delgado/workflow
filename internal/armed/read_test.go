// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package armed_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// scriptedForge answers a hold's reads in turn, the last answer repeating:
// each find, and each read of CI, which it notes the head of.
type scriptedForge struct {
	finds    []forge.PullRequest
	findErr  error
	cis      []forge.CI
	ciErr    error
	branches []string
	heads    []string
}

// nth is the answer at turn of answers, the last repeating.
func nth[T any](answers []T, turn int) T {
	return answers[min(turn, len(answers)-1)]
}

// find answers the next find: the pull request, found unless it is the zero
// one.
func (f *scriptedForge) find(_ context.Context, branch string) (forge.PullRequest, bool, error) {
	f.branches = append(f.branches, branch)
	if f.findErr != nil {
		return forge.PullRequest{}, false, f.findErr
	}

	pull := nth(f.finds, len(f.branches)-1)

	return pull, pull.Number != 0, nil
}

// checkCI answers the next read of CI, noting the head it was read at.
func (f *scriptedForge) checkCI(_ context.Context, _ forge.PullRequest, head string) (forge.CI, error) {
	f.heads = append(f.heads, head)
	if f.ciErr != nil {
		return forge.CI{}, f.ciErr
	}

	return nth(f.cis, len(f.heads)-1), nil
}

// seams are the hold's seams over the scripted forge, posting through post.
func (f *scriptedForge) seams(post func(context.Context, string, string) error) armed.Seams {
	return armed.Seams{
		FindPull: f.find, CheckCI: f.checkCI,
		Deliver: loop.DeliverSeams{Post: post, CheckCI: f.checkCI},
	}
}

// stoppedClock tells armedAt plus a minute.
func stoppedClock() time.Time {
	return after(time.Minute)
}

func TestAReadFindsThePullRequestByTheBranchArmed(t *testing.T) {
	t.Parallel()

	// Arrange
	// C37: whatever the checkout has switched to, the read finds the branch
	// the hold was armed on, and reads CI at the forge's head (P023).
	scripted := &scriptedForge{finds: []forge.PullRequest{heldPull()}, cis: []forge.CI{{State: forge.CIRunning}}}

	// Act
	reading, failure := scripted.seams(nil).Read(t.Context(), running(t), stoppedClock)

	// Assert
	if failure.Err != nil || !reading.Found || reading.CI.State != forge.CIRunning {
		t.Fatalf("Read = %+v, %v; want CI running heard", reading, failure.Err)
	}

	if len(scripted.branches) != 1 || scripted.branches[0] != branchName ||
		len(scripted.heads) != 1 || scripted.heads[0] != heldHead {
		t.Errorf("read %q at %q, want %q at %q", scripted.branches, scripted.heads, branchName, heldHead)
	}

	if reading.Round != running(t).Round() || !reading.At.Equal(stoppedClock()) {
		t.Errorf("reading in round %d at %v, want the hold's round, at the clock's time", reading.Round, reading.At)
	}
}

func TestAReadOfAPullRequestNotToWaitOnReadsNoCI(t *testing.T) {
	t.Parallel()

	cases := map[string]forge.PullRequest{
		"none":    {},
		"closed":  with(heldPull(), forge.StateClosed),
		"merged":  with(heldPull(), forge.StateMerged),
		"another": otherPull(),
	}

	for name, found := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			scripted := &scriptedForge{finds: []forge.PullRequest{found}}

			// Act
			reading, failure := scripted.seams(nil).Read(t.Context(), running(t), stoppedClock)

			// Assert
			if failure.Err != nil || reading.Pull != found || len(scripted.heads) != 0 {
				t.Errorf("Read = %+v, %v, reading CI at %q; want the find heard, no CI read",
					reading, failure.Err, scripted.heads)
			}
		})
	}
}

func TestAReadThatFailsIsAFailure(t *testing.T) {
	t.Parallel()

	noHead := heldPull()
	noHead.HeadCommit = ""

	cases := map[string]struct {
		scripted *scriptedForge
		want     error
	}{
		"the find failed": {scripted: &scriptedForge{findErr: errSeam}, want: errSeam},
		"CI could not be read": {
			scripted: &scriptedForge{finds: []forge.PullRequest{heldPull()}, ciErr: errSeam}, want: errSeam,
		},
		"no head to read CI at (P023)": {
			scripted: &scriptedForge{finds: []forge.PullRequest{noHead}}, want: armed.ErrNoHead,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			hold := running(t)

			// Act
			_, failure := tt.scripted.seams(nil).Read(t.Context(), hold, stoppedClock)

			// Assert
			if !errors.Is(failure.Err, tt.want) || failure.Round != hold.Round() || !failure.At.Equal(stoppedClock()) {
				t.Errorf("Read failed with %+v, want %v in the hold's round, at the clock's time", failure, tt.want)
			}
		})
	}
}

// postsTo is a post that notes each text it posts.
func postsTo(texts *[]string) func(context.Context, string, string) error {
	return func(_ context.Context, _, text string) error {
		*texts = append(*texts, text)

		return nil
	}
}

// unaskedAna tags Ana, asked of no one.
func unaskedAna(context.Context, loop.Delivery) messaging.Mentions {
	mentions, _ := messaging.NewMentions([]string{"U0ANA"}, nil)

	return mentions
}

func TestDeliveringAHoldPostsItsDelivery(t *testing.T) {
	t.Parallel()

	// Arrange
	var texts []string

	scripted := &scriptedForge{cis: []forge.CI{{State: forge.CIPassed}}}
	seams := scripted.seams(postsTo(&texts))
	seams.UnaskedTags = unaskedAna
	hold := due(t)

	// Act
	outcome := seams.DeliverHeld(t.Context(), hold, stoppedClock)

	// Assert
	if outcome.Err != nil || outcome.Round != hold.Round() || !outcome.At.Equal(stoppedClock()) {
		t.Errorf("DeliverHeld = %+v, want posted in the hold's round, at the clock's time", outcome)
	}

	// Asked for, its tags are its own: none proposed as it posts.
	if len(texts) != 1 || texts[0] != hold.Delivery().Text || scripted.heads[0] != heldHead {
		t.Errorf("posted %q, the gate reading %q; want the delivery's text, untagged, the gate at %q",
			texts, scripted.heads, heldHead)
	}
}

func TestDeliveringAHoldArmedOnItsOwnProposesItsTags(t *testing.T) {
	t.Parallel()

	// Arrange
	var texts []string

	scripted := &scriptedForge{cis: []forge.CI{{State: forge.CIPassed}}}
	seams := scripted.seams(postsTo(&texts))
	seams.UnaskedTags = unaskedAna
	hold := heardAs(listing(t), heldPull(), forge.CIPassed, time.Minute)

	// Act
	outcome := seams.DeliverHeld(t.Context(), hold, stoppedClock)

	// Assert
	if outcome.Err != nil || len(texts) != 1 || !strings.HasSuffix(texts[0], "\ncc <@U0ANA>") {
		t.Errorf("DeliverHeld = %v, posted %q; want it posted tagging Ana", outcome.Err, texts)
	}
}

func TestDeliveringAHoldAnswersHowThePostWent(t *testing.T) {
	t.Parallel()

	// Arrange
	scripted := &scriptedForge{cis: []forge.CI{{State: forge.CIFailed}}}

	// Act
	outcome := scripted.seams(postsTo(new([]string))).DeliverHeld(t.Context(), due(t), stoppedClock)

	// Assert
	if !errors.Is(outcome.Err, loop.ErrCIFailed) {
		t.Errorf("DeliverHeld = %v, want the gate's refusal", outcome.Err)
	}
}

func TestDeliveringAHoldArmedOnItsOwnWithNoOneToTagPostsItUntagged(t *testing.T) {
	t.Parallel()

	// Arrange
	// Without a Slack user token there is no directory to propose tags from.
	var texts []string

	scripted := &scriptedForge{cis: []forge.CI{{State: forge.CIPassed}}}
	hold := heardAs(listing(t), heldPull(), forge.CIPassed, time.Minute)

	// Act
	outcome := scripted.seams(postsTo(&texts)).DeliverHeld(t.Context(), hold, stoppedClock)

	// Assert
	if outcome.Err != nil || len(texts) != 1 || texts[0] != hold.Delivery().Text {
		t.Errorf("DeliverHeld = %v, posted %q; want the text alone", outcome.Err, texts)
	}
}
