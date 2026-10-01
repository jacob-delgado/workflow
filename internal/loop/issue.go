// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// IssueOrigin is where a branch's issue was found.
type IssueOrigin int

const (
	// OriginLink is an issue the branch was linked to by hand.
	OriginLink IssueOrigin = iota + 1
	// OriginBranch is an issue the branch's name names.
	OriginBranch
	// OriginPull is an issue the branch's pull request names in its title or
	// description.
	OriginPull
)

// IssueSource is everything that can say which issue a branch is for: the
// branch's name, the issue it was linked to by hand, and its pull request,
// when there is one. Project is the Jira project a key must be in, when one is
// configured.
type IssueSource struct {
	Branch  string
	Link    string
	Pull    *forge.PullRequest
	Project string
}

// BranchIssue is the issue a branch is for, and where that was found: the
// issue it was linked to by hand, which says so on purpose; otherwise the one
// its name names, as workflow names its own branches; otherwise the one its
// pull request names, for work begun outside workflow. A link that names no
// issue is passed over rather than trusted, since it was read back from disk.
func BranchIssue(src IssueSource) (convention.IssueRef, IssueOrigin, bool) {
	if ref, known := convention.RefOf(src.Link); known {
		return ref, OriginLink, true
	}

	if ref, named := convention.IssueKey(src.Branch, src.Project); named {
		return ref, OriginBranch, true
	}

	if src.Pull == nil {
		return convention.IssueRef{}, 0, false
	}

	if ref, named := convention.IssueInText(src.Pull.Title+"\n"+src.Pull.Body, src.Project); named {
		return ref, OriginPull, true
	}

	return convention.IssueRef{}, 0, false
}

// IssueOf is the issue branch is for by its link or its name, where no pull
// request is at hand to ask.
func IssueOf(branch gitrepo.Branch, project string) (convention.IssueRef, bool) {
	ref, _, found := BranchIssue(IssueSource{Branch: branch.Name, Link: branch.IssueLink, Project: project})

	return ref, found
}

// NamedIssue is the issue a branch known only by its name is for: the one it
// was linked to, in links, or else the one its name names.
func NamedIssue(name string, links map[string]string, project string) (convention.IssueRef, bool) {
	ref, _, found := BranchIssue(IssueSource{Branch: name, Link: links[name], Project: project})

	return ref, found
}
