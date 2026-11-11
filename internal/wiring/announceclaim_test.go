// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// readyAt7 is pull request 7's announcement as ready for review, on the
// fake forge's head.
func readyAt7() loop.Delivery {
	return loop.Delivery{
		Channel: "#dev", Text: "ready", Pull: forge.PullRequest{Number: 7, HeadCommit: "abc1234"}, Head: "abc1234",
		Moment: messaging.MomentReady,
	}
}

// claimsOf is the memory a delivery claims, confirms and releases through,
// over the store's seams.
func claimsOf(store seams.Store) loop.AnnounceMemory {
	return loop.AnnounceMemory{
		Recorded: store.Announced, Claim: store.ClaimAnnounce, Confirm: store.ConfirmAnnounce,
		Release: store.ReleaseAnnounce, Reclaim: store.ReclaimAnnounce, RecordGroups: nil,
	}
}

// posting is a delivery's seams whose CI passed and whose post counts itself
// in posts.
func posting(posts *atomic.Int32) loop.DeliverSeams {
	return loop.DeliverSeams{
		Post: func(context.Context, string, string) error {
			posts.Add(1)

			return nil
		},
		CheckCI: func(context.Context, forge.PullRequest, string) (forge.CI, error) {
			return forge.CI{State: forge.CIPassed}, nil
		},
	}
}

// inAStore is a workspace whose store sits in a home of the test's own.
func inAStore(t *testing.T) wiring.Workspace {
	t.Helper()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", "")

	return onOrigin(t.TempDir(), "https://github.com/org/repo.git")
}

func TestAClaimWhosePostWasNeverConfirmedReadsUnconfirmedAndIsNotPosted(t *testing.T) {
	// Arrange
	// One process claims the announcement and dies before its post is
	// confirmed; another starts on the same store.
	where := inAStore(t)
	made := readyAt7().Made()

	_, err := wired(t, config.Default(), where, nil).Store.ClaimAnnounce(t.Context(), made)
	if err != nil {
		t.Fatalf("claiming: %v", err)
	}

	next := wired(t, config.Default(), where, nil).Store

	var posts atomic.Int32

	// Act
	records, readErr := next.Announced(t.Context())
	deliverErr := loop.Deliver(t.Context(), posting(&posts), claimsOf(next), readyAt7())

	// Assert
	open := forge.PullRequest{Number: 7, State: forge.StateOpen}
	if got := loop.StandingOf(records, open, true); readErr != nil || got != loop.StandingUnconfirmed {
		t.Errorf("the next process reads it %v (%v), want it unconfirmed", got, readErr)
	}

	if !errors.Is(deliverErr, loop.ErrAnnouncedAlready) || posts.Load() != 0 {
		t.Errorf("Deliver = %v after %d posts, want it held and nothing posted", deliverErr, posts.Load())
	}
}

func TestTwoProcessesDeliveringOneAnnouncementPostItOnce(t *testing.T) {
	// Arrange
	// Two processes over one store each deliver the same announcement from
	// four goroutines at once.
	where := inAStore(t)
	first, second := wired(t, config.Default(), where, nil).Store, wired(t, config.Default(), where, nil).Store

	err := first.RecordScope(t.Context(), "api")
	if err != nil {
		t.Fatalf("making the store: %v", err)
	}

	var (
		posts atomic.Int32
		wait  sync.WaitGroup
	)

	deliver := func(store seams.Store) {
		_ = loop.Deliver(context.Background(), posting(&posts), claimsOf(store), readyAt7())
	}

	// Act
	for range 4 {
		wait.Go(func() { deliver(first) })
		wait.Go(func() { deliver(second) })
	}

	wait.Wait()

	// Assert
	if got := posts.Load(); got != 1 {
		t.Errorf("posted %d times from two processes, want once", got)
	}
}

func TestTheStoresReadOfWhatWasAnnouncedSaysWhyItFailed(t *testing.T) {
	// Arrange
	deps := wired(t, config.Default(), brokenStore(t), nil).Store

	// Act
	records, err := deps.Announced(t.Context())

	// Assert
	if err == nil || len(records) != 0 {
		t.Errorf("Announced = %+v, %v; want the store's failure, not none announced", records, err)
	}
}

func TestNoCredentialReachesAnAnnouncementsRow(t *testing.T) {
	// Arrange
	// Every credential the configuration and the remote carry, and an
	// announcement delivered, confirmed, released and claimed again.
	const (
		remoteToken = "ghp_REMOTESECRET123"
		slackToken  = "xoxp-SLACKSECRET123"
		webhook     = "https://hooks.slack.com/services/T0/B0/WEBHOOKSECRET"
		jiraToken   = "JIRASECRET123"
	)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", "")

	cfg := config.Default()
	cfg.Jira.BaseURL, cfg.Jira.Token = jiraAddress, config.Secret(jiraToken)
	cfg.Messaging.AccessToken, cfg.Messaging.WebhookURL = config.Secret(slackToken), config.Secret(webhook)
	where := onOrigin(t.TempDir(), "https://alice:"+remoteToken+"@github.com/org/repo.git")
	deps := wired(t, cfg, where, nil).Store

	var posts atomic.Int32

	merged := readyAt7()
	merged.Moment = messaging.MomentMerged

	// Act
	_ = loop.Deliver(t.Context(), posting(&posts), claimsOf(deps), readyAt7())
	_ = loop.Redeliver(t.Context(), posting(&posts), claimsOf(deps), readyAt7(), 1)
	_, _ = deps.ClaimAnnounce(t.Context(), merged.Made())
	_ = deps.ReleaseAnnounce(t.Context(), merged.Made(), 1)

	// Assert
	dir, err := store.DefaultDir()
	if err != nil {
		t.Fatalf("resolving the store directory: %v", err)
	}

	for _, secret := range []string{remoteToken, slackToken, webhook, "WEBHOOKSECRET", jiraToken} {
		if storeHoldsToken(t, dir, secret) {
			t.Errorf("the store holds the credential %q", secret)
		}
	}
}
