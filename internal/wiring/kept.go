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
// repository. Management writes return their errors, so a surface can say why
// one was refused.
func bindKept(ctx context.Context, kept store.Store, where Workspace, bound seams.Store) seams.Store {
	host := forgeHostKey(where)
	repo := repoKey(where)

	bound.OwnerLinks = func() ([]loop.OwnerLink, error) {
		links, err := kept.OwnerLinks(ctx, host)

		return fromStoreLinks(links), err
	}
	bound.LinkOwner = func(owner string, target *loop.SlackTarget) error {
		return kept.LinkOwner(ctx, host, owner, toStoreTarget(target), time.Now())
	}
	bound.ForgetOwner = func(owner string) error { return kept.ForgetOwner(ctx, host, owner) }
	bound.RepoGroups = func() ([]loop.SlackTarget, error) {
		groups, err := kept.RepoGroups(ctx, repo)

		return fromStoreTargets(groups), err
	}
	bound.SetRepoGroups = func(groups []loop.SlackTarget) error {
		return kept.SetRepoGroups(ctx, repo, toStoreTargets(groups), time.Now())
	}
	bound.LastGroups = func() ([]string, bool) {
		ids, chosen, _ := kept.LastGroups(ctx, repo)

		return ids, chosen
	}
	bound.RecordGroups = func(ids []string) error { return kept.RecordGroups(ctx, repo, ids, time.Now()) }

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
			Owner: link.Owner, OnSlack: link.OnSlack, Slack: loop.SlackTarget(link.Slack),
		})
	}

	return mapped
}

// toStoreTarget maps a Slack target to the store's, keeping nil as "not on
// Slack".
func toStoreTarget(target *loop.SlackTarget) *store.SlackTarget {
	if target == nil {
		return nil
	}

	mapped := store.SlackTarget(*target)

	return &mapped
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
