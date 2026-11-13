// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// errNotAJiraKey is a tracker key with no project part, such as the forge issue
// number 42 a branch can name. Jira would read that number as the id of an
// unrelated issue, so it is never asked.
var errNotAJiraKey = errors.New("not a Jira issue key")

// jiraDeps is what a surface asks of Jira, each seam reaching it through
// jiraClient, which finds the token the first time Jira is asked, under the
// context the seam is asked under, and again when Jira refuses one a command
// printed. It needs no guard for a missing or malformed configuration: the
// client refuses before sending anything, and the pane shows why.
func jiraDeps(settings config.Jira, jiraClient *jiraConnection) seams.Jira {
	// A link reads only the address, so it comes from a client that never sends
	// and so never needs the token.
	addresses := jira.New(nil, settings)

	return seams.Jira{
		Search: func(ctx context.Context, jql string, startAt int) (jira.SearchResult, error) {
			return askJira(ctx, jiraClient, func(client jira.Client) (jira.SearchResult, error) {
				return client.Search(ctx, jql, startAt)
			})
		},
		SearchLenient: func(ctx context.Context, jql string, startAt int) (jira.SearchResult, error) {
			return askJira(ctx, jiraClient, func(client jira.Client) (jira.SearchResult, error) {
				return client.SearchLenient(ctx, jql, startAt)
			})
		},
		Issue: func(ctx context.Context, issueKey jira.Key) (jira.IssueDetail, error) {
			return askJiraIssue(ctx, jiraClient, issueKey, func(client jira.Client) (jira.IssueDetail, error) {
				return client.Issue(ctx, issueKey)
			})
		},
		Transitions: func(ctx context.Context, issueKey jira.Key) ([]jira.Transition, error) {
			return askJiraIssue(ctx, jiraClient, issueKey, func(client jira.Client) ([]jira.Transition, error) {
				return client.Transitions(ctx, issueKey)
			})
		},
		Transition: func(ctx context.Context, issueKey jira.Key, to jira.Transition, values []jira.FieldValue) error {
			return tellJiraIssue(ctx, jiraClient, issueKey, func(client jira.Client) error {
				return client.ApplyTransition(ctx, issueKey, to, values)
			})
		},
		Comment: func(ctx context.Context, issueKey jira.Key, text string) (jira.Comment, error) {
			return changeJiraIssue(ctx, jiraClient, issueKey, func(client jira.Client) (jira.Comment, error) {
				return client.AddComment(ctx, issueKey, text)
			})
		},
		Activity: func(ctx context.Context, start, end time.Time) (jira.Activity, error) {
			return askJira(ctx, jiraClient, func(client jira.Client) (jira.Activity, error) {
				return client.Activity(ctx, start, end)
			})
		},
		Assign: func(ctx context.Context, issueKey jira.Key, assignee string) error {
			return tellJiraIssue(ctx, jiraClient, issueKey, func(client jira.Client) error {
				return client.Assign(ctx, issueKey, assignee)
			})
		},
		AddWorklog: func(ctx context.Context, issueKey jira.Key, timeSpent, comment string) (jira.Worklog, error) {
			return changeJiraIssue(ctx, jiraClient, issueKey, func(client jira.Client) (jira.Worklog, error) {
				return client.AddWorklog(ctx, issueKey, timeSpent, comment)
			})
		},
		LinkPullRequest: func(ctx context.Context, issueKey jira.Key, pullURL, title string) error {
			return tellJiraIssue(ctx, jiraClient, issueKey, func(client jira.Client) error {
				return client.LinkPullRequest(ctx, issueKey, pullURL, title)
			})
		},
		BrowseURL: func(issueKey jira.Key) string { return browseJiraIssue(addresses, issueKey) },
	}
}

// jiraConnected is a Jira client, and whether its token came from
// token_command, which can print another once Jira refuses the one it did.
type jiraConnected struct {
	client      jira.Client
	fromCommand bool
}

// connectJira finds the Jira token, reading the keychain through system or
// running its command when either is set, and builds the client that carries
// it over doer. A token source that gives none is no credential at all, and is
// reported as such before anything is sent.
func (e Environment) connectJira(
	ctx context.Context, settings config.Jira, system Keychain, doer jira.Doer,
) (jiraConnected, error) {
	if settings.AuthMode() == config.AuthNone {
		return jiraConnected{client: jira.New(doer, settings), fromCommand: false}, nil
	}

	token, source, err := e.resolveSetToken(ctx, settings, system)
	if err != nil {
		return jiraConnected{}, fmt.Errorf("%w: %w", jira.ErrNoCredential, err)
	}

	settings.Token = token

	return jiraConnected{client: jira.New(doer, settings), fromCommand: source == sourceCommand}, nil
}

// jiraConnection is the Jira client found the first time Jira is asked and
// held for every call after, as onceConnected holds a connection: a failure
// is not held, so the next call looks again. Each client found is numbered
// from one, zero while none has been, so a token refused to several calls at
// once is found again once, by the first of them, and handed to the rest. One
// call connects at a time, and the others wait for it only as long as their
// own contexts last.
type jiraConnection struct {
	turn       connecting
	connect    func(context.Context) (jiraConnected, error)
	connected  jiraConnected
	generation int
}

// newJiraConnection holds no client yet, and finds one with connect.
func newJiraConnection(connect func(context.Context) (jiraConnected, error)) *jiraConnection {
	return &jiraConnection{turn: newConnecting(), connect: connect, connected: jiraConnected{}, generation: 0}
}

// current is the client held, and its number, finding one first when none is.
func (c *jiraConnection) current(ctx context.Context) (jiraConnected, int, error) {
	return c.holdingTurn(ctx, func() (jiraConnected, int, error) {
		if c.generation > 0 {
			return c.connected, c.generation, nil
		}

		return c.connectAgain(ctx)
	})
}

// renewed is a client whose token was found again, for the one numbered
// refused, whose token Jira turned away. A client found since that one is
// handed out as it is, rather than running the token command once more. One
// not found keeps the refused one held, so the next call is refused and finds
// it again.
func (c *jiraConnection) renewed(ctx context.Context, refused int) (jiraConnected, error) {
	connected, _, err := c.holdingTurn(ctx, func() (jiraConnected, int, error) {
		if c.generation != refused {
			return c.connected, c.generation, nil
		}

		return c.connectAgain(ctx)
	})

	return connected, err
}

// holdingTurn runs use while holding the connecting turn, which it waits for
// only as long as ctx lasts: a call whose context ends while another
// connects gives up with that context's end.
func (c *jiraConnection) holdingTurn(
	ctx context.Context, use func() (jiraConnected, int, error),
) (jiraConnected, int, error) {
	err := c.turn.take(ctx)
	if err != nil {
		return jiraConnected{}, 0, err
	}
	defer c.turn.give()

	return use()
}

// connectAgain finds a client and holds it under the next number. The caller
// holds the turn.
func (c *jiraConnection) connectAgain(ctx context.Context) (jiraConnected, int, error) {
	connected, err := c.connect(ctx)
	if err != nil {
		return jiraConnected{}, 0, err
	}

	c.connected = connected
	c.generation++

	return connected, c.generation, nil
}

// askJira asks Jira what question reads, through the client jiraClient holds.
// A token token_command printed can expire while workflow runs, so when Jira
// refuses one the command runs again, once, and the question is asked again
// with what it prints: a read changes nothing however Jira served it. A token
// from any other source is the same token however often it is read, and its
// refusal stands.
func askJira[T any](ctx context.Context, jiraClient *jiraConnection, question func(jira.Client) (T, error)) (T, error) {
	return renewingJira(ctx, jiraClient, question, func(err error) bool {
		return errors.Is(err, jira.ErrUnauthorized)
	})
}

// changeJira has Jira make the change asks for, renewing a refused
// token_command token as askJira does, but only when Jira did nothing: a
// write it served anonymously and answered as a success is done, and sending
// it again would make it twice.
func changeJira[T any](
	ctx context.Context, jiraClient *jiraConnection, change func(jira.Client) (T, error),
) (T, error) {
	return renewingJira(ctx, jiraClient, change, func(err error) bool {
		return errors.Is(err, jira.ErrUnauthorized) && !errors.Is(err, jira.ErrServedAnonymously)
	})
}

// renewingJira sends request through the client jiraClient holds and, when
// its token came from token_command and askAgain takes Jira's refusal for one
// that changed nothing, sends it once more with the token the command prints
// next.
func renewingJira[T any](
	ctx context.Context, jiraClient *jiraConnection, request func(jira.Client) (T, error), askAgain func(error) bool,
) (T, error) {
	var none T

	connected, generation, err := jiraClient.current(ctx)
	if err != nil {
		return none, err
	}

	answer, err := request(connected.client)
	if !connected.fromCommand || !askAgain(err) {
		return answer, err
	}

	connected, err = jiraClient.renewed(ctx, generation)
	if err != nil {
		return none, err
	}

	return request(connected.client)
}

// askJiraIssue asks Jira about the issue issueKey names. A key with no project
// part is refused as missing without asking, or finding the token to ask with:
// Jira reads a bare number as an issue's id, so a forge issue's 42 would reach
// whichever Jira issue has that id. The forge's issues refuse a key that is
// not a number the same way, so every surface falls back as it does for an
// issue the tracker lacks.
func askJiraIssue[T any](
	ctx context.Context, jiraClient *jiraConnection, issueKey jira.Key, question func(jira.Client) (T, error),
) (T, error) {
	if !isJiraKey(issueKey) {
		var none T

		return none, notAJiraIssue(issueKey)
	}

	return askJira(ctx, jiraClient, question)
}

// changeJiraIssue has Jira change the issue issueKey names, through
// changeJira, refusing a key with no project part as askJiraIssue does.
func changeJiraIssue[T any](
	ctx context.Context, jiraClient *jiraConnection, issueKey jira.Key, change func(jira.Client) (T, error),
) (T, error) {
	if !isJiraKey(issueKey) {
		var none T

		return none, notAJiraIssue(issueKey)
	}

	return changeJira(ctx, jiraClient, change)
}

// tellJiraIssue is changeJiraIssue for a change Jira answers with nothing.
func tellJiraIssue(
	ctx context.Context, jiraClient *jiraConnection, issueKey jira.Key, change func(jira.Client) error,
) error {
	_, err := changeJiraIssue(ctx, jiraClient, issueKey, func(client jira.Client) (struct{}, error) {
		return struct{}{}, change(client)
	})

	return err
}

// notAJiraIssue is the refusal of a key with no project part: missing, as an
// issue Jira lacks is.
func notAJiraIssue(issueKey jira.Key) error {
	return fmt.Errorf("%w: %w: %q", jira.ErrNotFound, errNotAJiraKey, issueKey)
}

// browseJiraIssue links an issue for someone to click, or is empty for a key
// with no project part, which has no page of its own on Jira.
func browseJiraIssue(client jira.Client, issueKey jira.Key) string {
	if !isJiraKey(issueKey) {
		return ""
	}

	return client.BrowseURL(issueKey)
}

// isJiraKey reports a key shaped like Jira's, PROJ-42, rather than the bare
// forge issue number a branch can carry instead, which Jira would refuse or
// read as the id of an unrelated issue.
func isJiraKey(issueKey jira.Key) bool {
	ref, known := convention.RefOf(string(issueKey))

	return known && ref.Tracker == convention.TrackerJira
}
