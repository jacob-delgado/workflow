// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/store"
)

// bindKept binds the kept associations into the store's seams: a forge owner's
// Slack identity by this repository's forge host, since an owner is the same
// person in every repository on it, and the Slack groups to tag by the
// repository, each under the Slack workspace it was made in. Management
// writes return their errors, so a surface can say why one was refused. A
// store that keeps nothing binds none of them, and a workspace with no forge
// host binds no owner links, so a surface offers no management that would
// save nothing.
func bindKept(ctx context.Context, kept store.Store, where Workspace, bound seams.Store) seams.Store {
	if !kept.Keeps() {
		return bound
	}

	repo := repoKey(where)

	bound.RepoGroups = func(workspace string) ([]loop.SlackTarget, error) {
		groups, err := kept.RepoGroups(ctx, repo, workspace)

		return fromStoreTargets(groups), err
	}
	bound.SetRepoGroups = func(workspace string, groups []loop.SlackTarget) error {
		return kept.SetRepoGroups(ctx, repo, workspace, toStoreTargets(groups), time.Now())
	}
	bound.LastGroups = func(workspace string) ([]string, bool) {
		ids, chosen, _ := kept.LastGroups(ctx, repo, workspace)

		return ids, chosen
	}
	bound.RecordGroups = func(workspace string, ids []string) error {
		return kept.RecordGroups(ctx, repo, workspace, ids, time.Now())
	}

	return bindOwnerLinks(ctx, kept, forgeHostKey(where), bound)
}

// bindOwnerLinks binds whom each forge owner on host is on Slack, unless there
// is no host to key them by.
func bindOwnerLinks(ctx context.Context, kept store.Store, host string, bound seams.Store) seams.Store {
	if host == "" {
		return bound
	}

	bound.OwnerLinks = func(workspace string) ([]loop.OwnerLink, error) {
		links, err := kept.OwnerLinks(ctx, host, workspace)

		return fromStoreLinks(links), err
	}
	bound.LinkOwner = func(workspace string, decision loop.OwnerLink) error {
		return kept.LinkOwner(ctx, host, workspace, toStoreLink(decision), time.Now())
	}
	bound.ForgetOwner = func(workspace, owner string) error { return kept.ForgetOwner(ctx, host, workspace, owner) }

	return bound
}

// forgeHostKey names the forge host the kept owner links are keyed by: the
// origin remote's host, lowercased since a host's case means nothing, and empty
// where there is no remote that parses, so nothing is kept. It is parsed as
// repoKey parses it, never used raw, because an HTTPS remote can carry a
// credential in its userinfo and the store must never hold a secret.
func forgeHostKey(where Workspace) string {
	repo, parsed := parsedRemote(where)
	if !parsed {
		return ""
	}

	return strings.ToLower(repo.Host)
}

// fromStoreLinks maps the store's owner links to the loop's.
func fromStoreLinks(links []store.OwnerLink) []loop.OwnerLink {
	mapped := make([]loop.OwnerLink, 0, len(links))
	for _, link := range links {
		mapped = append(mapped, loop.OwnerLink{
			Owner: link.Owner, Team: link.Team, OnSlack: link.OnSlack, Slack: loop.SlackTarget(link.Slack),
		})
	}

	return mapped
}

// toStoreLink maps a decision about an owner to the store's.
func toStoreLink(decision loop.OwnerLink) store.OwnerLink {
	return store.OwnerLink{
		Owner: decision.Owner, Team: decision.Team, OnSlack: decision.OnSlack, Slack: store.SlackTarget(decision.Slack),
	}
}

// fromStoreTargets maps the store's Slack targets to the loop's.
func fromStoreTargets(targets []store.SlackTarget) []loop.SlackTarget {
	mapped := make([]loop.SlackTarget, 0, len(targets))
	for _, target := range targets {
		mapped = append(mapped, loop.SlackTarget(target))
	}

	return mapped
}

// toStoreTargets maps Slack targets to the store's.
func toStoreTargets(targets []loop.SlackTarget) []store.SlackTarget {
	mapped := make([]store.SlackTarget, 0, len(targets))
	for _, target := range targets {
		mapped = append(mapped, store.SlackTarget(target))
	}

	return mapped
}
