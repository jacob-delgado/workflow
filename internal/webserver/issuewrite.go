// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// Why the checked-out branch's pull request cannot be linked on an issue.
var (
	// errNotTheBranchIssue refuses a link on an issue the checked-out branch
	// does not name: the pull request would be recorded against the wrong one.
	errNotTheBranchIssue = errors.New("the checked-out branch does not name that issue")
	// errNoPullToLink refuses a link when the forge has no pull request for the
	// checked-out branch.
	errNoPullToLink = errors.New("the checked-out branch has no pull request to link")
)

// LinkPullRequest records the checked-out branch's pull request as a link on
// the issue the branch names — the first offer the terminal and `workflow pr`
// make once a pull request is open. The pull request is found here, from the
// branch; the request names only the issue, so a caller can never have a URL of
// its choosing recorded on the tracker.
func (s *server) LinkPullRequest(
	_ context.Context, request api.LinkPullRequestRequestObject,
) (api.LinkPullRequestResponseObject, error) {
	if s.deps.LinkPullRequest == nil || s.deps.Branch == nil || s.deps.FindPull == nil {
		return api.LinkPullRequest422ApplicationProblemPlusJSONResponse(
			problem(api.ProblemCodeUnprocessable, "linking a "+s.noun()+" on an issue is not available")), nil
	}

	issueKey := jira.Key(request.Key)

	pull, err := s.branchPull(issueKey)
	if err != nil {
		return s.linkRefusal(err, issueKey), nil
	}

	err = s.deps.LinkPullRequest(issueKey, pull.URL, pull.Title)
	if err != nil {
		body, code := s.fault(err)

		return api.LinkPullRequestdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.LinkPullRequest200JSONResponse(pullDTO(pull)), nil
}

// branchPull is the pull request the forge has for the checked-out branch, when
// that branch names issueKey.
func (s *server) branchPull(issueKey jira.Key) (forge.PullRequest, error) {
	branch, err := s.deps.Branch()
	if err != nil {
		return forge.PullRequest{}, fmt.Errorf("reading the branch: %w", err)
	}

	named, ok := loop.JiraIssue(branch, s.config().Jira.Project)
	if !ok || named != issueKey {
		return forge.PullRequest{}, errNotTheBranchIssue
	}

	pull, found, err := s.deps.FindPull(branch.Name)
	if err != nil {
		return forge.PullRequest{}, fmt.Errorf("finding the branch's pull request: %w", err)
	}

	if !found {
		return forge.PullRequest{}, errNoPullToLink
	}

	return pull, nil
}

// linkRefusal answers a link that was not made: a 409 when the branch does not
// name the issue or has nothing to link, and the curated fault otherwise.
func (s *server) linkRefusal(err error, issueKey jira.Key) api.LinkPullRequestResponseObject {
	switch {
	case errors.Is(err, errNotTheBranchIssue):
		return api.LinkPullRequest409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict,
			"the checked-out branch does not name "+string(issueKey)+"; switch to its branch to link it"))
	case errors.Is(err, errNoPullToLink):
		return api.LinkPullRequest409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict,
			"the checked-out branch has no "+s.noun()+" to link on "+string(issueKey)))
	default:
		body, code := s.fault(err)

		return api.LinkPullRequestdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
	}
}

// TransitionIssue moves an issue to the configured review status — the second
// offer after opening — and nowhere else, without fields. It is not a general
// transition: a move Jira wants fields for is ChangeStatus's, whose form on
// the page asks for them.
func (s *server) TransitionIssue(
	_ context.Context, request api.TransitionIssueRequestObject,
) (api.TransitionIssueResponseObject, error) {
	if s.deps.Transitions == nil || s.deps.Transition == nil {
		return api.TransitionIssue422ApplicationProblemPlusJSONResponse(
			problem(api.ProblemCodeUnprocessable, "moving an issue is not available")), nil
	}

	issueKey := jira.Key(request.Key)
	status := s.config().Jira.ReviewStatus

	move, err := loop.FindReviewTransition(s.deps.Transitions, issueKey, status)
	if err != nil {
		return s.transitionRefusal(err, issueKey, status), nil
	}

	err = s.deps.Transition(issueKey, move, nil)
	if err != nil {
		body, code := s.fault(err)

		return api.TransitionIssuedefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.TransitionIssue200JSONResponse{Key: request.Key, Status: move.ToStatus}, nil
}

// transitionRefusal answers a move to the review status that was not made, in
// words that say what to do; a tracker that could not be read is the curated
// fault, which never carries the tracker's address.
func (s *server) transitionRefusal(err error, issueKey jira.Key, status string) api.TransitionIssueResponseObject {
	switch {
	case errors.Is(err, loop.ErrNoReviewStatus):
		return api.TransitionIssue422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable,
			"no review status is configured; set jira.review_status to move an issue there"))
	case errors.Is(err, loop.ErrReviewNeedsFields):
		return api.TransitionIssue409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict,
			"Jira wants fields filled to move "+string(issueKey)+" to "+status+
				"; change its status from the issue, whose form asks for them"))
	case errors.Is(err, loop.ErrNoReviewTransition):
		return api.TransitionIssue409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict,
			"Jira offers no move of "+string(issueKey)+" to "+status+" from where it stands"))
	default:
		body, code := s.fault(err)

		return api.TransitionIssuedefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
	}
}

// AddComment posts a comment on an issue, in Jira or on the forge, as the
// terminal's c does, and answers it as its tracker stored it. Blank text is
// refused before the tracker is asked.
func (s *server) AddComment(
	_ context.Context, request api.AddCommentRequestObject,
) (api.AddCommentResponseObject, error) {
	issueKey := jira.Key(request.Key)

	switch {
	case s.deps.Comment == nil:
		return commentRefusal("commenting on an issue is not available; configure Jira or a forge to comment"), nil
	case strings.TrimSpace(request.Body.Text) == "":
		return commentRefusal("a comment needs text"), nil
	}

	posted, err := s.deps.Comment(issueKey, loop.CommentMarkupOf(s.config().Jira, issueKey).Stored(request.Body.Text))
	if err != nil {
		return s.commentFailure(err), nil
	}

	return api.AddComment200JSONResponse(commentDTO(posted)), nil
}

// commentFailure answers a comment that was not posted. One the forge turned
// down keeps the forge's reason, such as a body too long, which the caller can
// act on. Every other failure is classified by fault, whose detail names no
// host.
func (s *server) commentFailure(err error) api.AddCommentResponseObject {
	if errors.Is(err, forge.ErrRejected) {
		return commentRefusal(err.Error())
	}

	body, code := s.fault(err)

	return api.AddCommentdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
}

// commentRefusal is a comment that was not posted, and why.
func commentRefusal(detail string) api.AddCommentResponseObject {
	return api.AddComment422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable, detail))
}

// followUps are what the page can offer once a pull request is open, in the
// order the terminal and `workflow pr` offer them: to link it on the issue the
// branch names, then to move that issue to the review status. A branch that
// names no Jira issue is offered neither, and a tracker that cannot be read
// offers no move — the pull request is already open either way. The list is
// never nil, so it goes to the page as [] rather than null.
func (s *server) followUps(branch gitrepo.Branch) []api.FollowUp {
	offers := []api.FollowUp{}

	issueKey, named := loop.JiraIssue(branch, s.config().Jira.Project)
	if !named {
		return offers
	}

	if s.deps.LinkPullRequest != nil {
		offers = append(offers, api.FollowUp{Action: api.FollowUpActionLink, IssueKey: string(issueKey)})
	}

	if s.deps.Transition == nil {
		return offers
	}

	move, ok := loop.ReviewTransition(s.deps.Transitions, issueKey, s.config().Jira.ReviewStatus)
	if ok {
		offers = append(offers, api.FollowUp{
			Action: api.FollowUpActionTransition, IssueKey: string(issueKey), Status: &move.ToStatus,
		})
	}

	return offers
}

// Why a status change's fields cannot be sent as given.
var (
	// errNotAsked is a value given for a field the change does not ask for.
	errNotAsked = errors.New("does not ask for")
	// errChangeGone is a change the tracker no longer offers from where the
	// issue stands.
	errChangeGone = errors.New("that status change is no longer offered")
)

// ListStatusChanges lists the status changes the tracker offers an issue, each
// with the fields it needs, as the terminal's status picker lists them.
func (s *server) ListStatusChanges(
	_ context.Context, request api.ListStatusChangesRequestObject,
) (api.ListStatusChangesResponseObject, error) {
	if s.deps.Transitions == nil {
		return api.ListStatusChanges422ApplicationProblemPlusJSONResponse(
			problem(api.ProblemCodeUnprocessable, "changing an issue's status is not available; configure Jira or a forge")), nil
	}

	moves, err := s.deps.Transitions(jira.Key(request.Key))
	if err != nil {
		body, code := s.fault(err)

		return api.ListStatusChangesdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.ListStatusChanges200JSONResponse(statusChangesDTO(moves)), nil
}

// ChangeStatus makes the status change the request names with the field values
// it gives, once each value passes the checks the terminal's field form makes.
// The changes are read again first, so only one the tracker offers now is made.
func (s *server) ChangeStatus(
	_ context.Context, request api.ChangeStatusRequestObject,
) (api.ChangeStatusResponseObject, error) {
	if s.deps.Transitions == nil || s.deps.Transition == nil {
		return api.ChangeStatus422ApplicationProblemPlusJSONResponse(
			problem(api.ProblemCodeUnprocessable, "changing an issue's status is not available; configure Jira or a forge")), nil
	}

	issueKey := jira.Key(request.Key)

	move, values, err := s.statusChange(issueKey, *request.Body)
	if err != nil {
		return s.statusChangeRefusal(err, issueKey), nil
	}

	err = s.deps.Transition(issueKey, move, values)
	if err != nil {
		body, code := s.fault(err)

		return api.ChangeStatusdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.ChangeStatus200JSONResponse{Key: request.Key, Status: move.ToStatus}, nil
}

// statusChange is the change asked for, as the tracker offers it now, with the
// value given for each field it needs, each checked against its field.
func (s *server) statusChange(
	issueKey jira.Key, asked api.StatusChangeRequest,
) (jira.Transition, []jira.FieldValue, error) {
	moves, err := s.deps.Transitions(issueKey)
	if err != nil {
		return jira.Transition{}, nil, fmt.Errorf("reading the status changes of %s: %w", issueKey, err)
	}

	index := slices.IndexFunc(moves, func(move jira.Transition) bool { return move.ID == asked.TransitionID })
	if index < 0 {
		return jira.Transition{}, nil, errChangeGone
	}

	move := moves[index]

	values, err := fieldValues(move, asked.Fields)

	return move, values, err
}

// fieldValues is the value given for each field move needs, in the order it
// lists them. It refuses a field only Jira can fill, a field left out or given
// a value it does not take, and a value for a field move does not ask for —
// each in words that name the field.
func fieldValues(move jira.Transition, entries []api.FieldEntry) ([]jira.FieldValue, error) {
	if field, blocked := move.Unfillable(); blocked {
		return nil, fmt.Errorf("%s needs %s, %w", move.Name, field.Name, jira.ErrOnlyJira)
	}

	for _, entry := range entries {
		if _, asks := move.Field(entry.ID); !asks {
			return nil, fmt.Errorf("%s %w %s", move.Name, errNotAsked, entry.ID)
		}
	}

	values := make([]jira.FieldValue, 0, len(move.Fields))

	for _, field := range move.Fields {
		value := fieldValue(field, entries)

		err := value.Check()
		if err != nil {
			return nil, fmt.Errorf("%s %w", field.Name, err)
		}

		values = append(values, value)
	}

	return values, nil
}

// fieldValue is the value entries give field, empty when they give none.
func fieldValue(field jira.Field, entries []api.FieldEntry) jira.FieldValue {
	value := jira.FieldValue{Field: field}

	index := slices.IndexFunc(entries, func(entry api.FieldEntry) bool { return entry.ID == field.ID })
	if index < 0 {
		return value
	}

	entry := entries[index]
	value.OptionID, value.Text = orZero(entry.OptionID), strings.TrimSpace(orZero(entry.Text))

	if entry.OptionIds != nil {
		value.OptionIDs = *entry.OptionIds
	}

	return value
}

// statusChangeRefusal answers a status change that was not made: a 409 for one
// no longer offered, a 422 naming the field for a value it cannot take, and
// the curated fault for a tracker that could not be read.
func (s *server) statusChangeRefusal(err error, issueKey jira.Key) api.ChangeStatusResponseObject {
	switch {
	case errors.Is(err, errChangeGone):
		return api.ChangeStatus409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict,
			"that status change is no longer offered for "+string(issueKey)+
				" from where it stands; read its status changes again"))
	case isFieldRefusal(err):
		return api.ChangeStatus422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable, err.Error()))
	default:
		body, code := s.fault(err)

		return api.ChangeStatusdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
	}
}

// isFieldRefusal reports a status change refused for one of its fields, whose
// words name only the change and the field, never the tracker's address.
func isFieldRefusal(err error) bool {
	return slices.ContainsFunc([]error{
		jira.ErrOnlyJira, jira.ErrNeedsValue, jira.ErrNeedsDate, jira.ErrNeedsChoice, jira.ErrNotAnOption, errNotAsked,
	}, func(refusal error) bool { return errors.Is(err, refusal) })
}

// AssignIssue sets an issue's assignee, as the terminal's a does, refusing a
// blank username before the tracker is asked.
func (s *server) AssignIssue(
	_ context.Context, request api.AssignIssueRequestObject,
) (api.AssignIssueResponseObject, error) {
	assignee := strings.TrimSpace(request.Body.Assignee)

	switch {
	case s.deps.Assign == nil:
		return assignRefusal("assigning an issue is not available; configure Jira or a forge"), nil
	case assignee == "":
		return assignRefusal("an assignee needs a username"), nil
	}

	err := s.deps.Assign(jira.Key(request.Key), assignee)
	if err != nil {
		body, code := s.fault(err)

		return api.AssignIssuedefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.AssignIssue200JSONResponse{Key: request.Key, Assignee: assignee}, nil
}

// assignRefusal is an assignment that was not made, and why.
func assignRefusal(detail string) api.AssignIssueResponseObject {
	return api.AssignIssue422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable, detail))
}

// LogWork logs time spent on a Jira issue, as the terminal's w does, with the
// note given. A forge issue keeps no worklog, and a blank duration is refused,
// both before Jira is asked.
func (s *server) LogWork(_ context.Context, request api.LogWorkRequestObject) (api.LogWorkResponseObject, error) {
	issueKey := jira.Key(request.Key)
	spent := strings.TrimSpace(request.Body.TimeSpent)

	switch {
	case s.deps.AddWorklog == nil:
		return worklogRefusal("logging work is not available; configure Jira to log work"), nil
	case trackerOf(issueKey) == api.IssueTrackerForge:
		return worklogRefusal("a forge issue keeps no worklog; log work on a Jira issue"), nil
	case spent == "":
		return worklogRefusal("logging work needs a duration, such as 2h or 30m"), nil
	}

	logged, err := s.deps.AddWorklog(issueKey, spent, strings.TrimSpace(orZero(request.Body.Comment)))
	if err != nil {
		body, code := s.fault(err)

		return api.LogWorkdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.LogWork200JSONResponse{Key: request.Key, TimeSpent: cmp.Or(logged.TimeSpent, spent)}, nil
}

// worklogRefusal is work that was not logged, and why.
func worklogRefusal(detail string) api.LogWorkResponseObject {
	return api.LogWork422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable, detail))
}
