// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// userPath answers "who does this credential belong to?" on both forges. It
// needs no scope on GitHub, which is what makes a 404 here mean a wrong address
// rather than an under-scoped token.
const userPath = "/user"

// userAgent identifies this program. It must never be empty: Go REMOVES the
// header when asked to set it to "", and GitHub answers 403 to a request with
// no User-Agent — a status that would otherwise read as a refused credential.
const userAgent = "workflow (+https://github.com/jacob-delgado/workflow)"

// bodyLimit bounds how much of an answer is read.
const bodyLimit = 16 << 20

// jsonMediaType is what an API answer must be before it is decoded.
const jsonMediaType = "application/json"

// rateLimitRemaining is the header in which GitHub counts the requests left
// before its rate limit; it reads "0" on the 403 that limit answers with.
const rateLimitRemaining = "X-Ratelimit-Remaining"

// A listing is read a page at a time, numbered from 1: perPage is a full page,
// the most either forge returns to one request, and maxPages bounds a listing
// far above any real queue, issue list or commit's check count, so it is read
// to its end without an unbounded loop.
const (
	perPageParam = "per_page"
	perPage      = 100
	maxPages     = 20
)

// uncounted is the count of a listing whose forge does not count it, as GitLab's
// arrays do not: no read reaches it, so only a short page ends the listing.
const uncounted = math.MaxInt

// Errors the client returns. Callers distinguish them with errors.Is.
var (
	// ErrUnauthorized reports a credential the forge did not accept.
	ErrUnauthorized = errors.New("the credential was not accepted")
	// ErrRefused reports a request the forge understood and would not serve
	// for this token, which is a question of what the token may do rather than
	// whether it is valid; the forge's own reason follows it in the message
	// when it gave one. A 403 that asks to wait is ErrRateLimited instead.
	ErrRefused = errors.New("the forge refused the request")
	// ErrNoAPI reports an address with no forge API behind it.
	ErrNoAPI = errors.New("no forge API answered at that address")
	// ErrNotJSON reports an answer that was not JSON, which usually means the
	// base URL points at the web host rather than the API.
	ErrNotJSON = errors.New("the answer was not JSON")
	// ErrRejected reports a request the forge turned down and explained; the
	// explanation follows it in the message.
	ErrRejected = errors.New("the forge rejected the request")
	// ErrNoRepository reports a repository the forge will not show this token:
	// both forges answer 404 rather than 403, so as not to confirm it exists.
	ErrNoRepository = errors.New("the repository was not found, or the token cannot see it")
	// ErrUnexpectedStatus reports any other status.
	ErrUnexpectedStatus = errors.New("unexpected response status")
	// ErrUnreachable reports a request that never got an answer.
	ErrUnreachable = errors.New("could not reach the forge")
	// ErrRedirected reports a redirect this client declined to follow.
	ErrRedirected = httpx.ErrRedirected
	// ErrRateLimited reports a forge asking to wait — a 429, or a 403 whose
	// headers ask for a wait — told apart from a refusal.
	ErrRateLimited = httpx.ErrRateLimited
)

// Doer is the HTTP seam this client accepts; see httpx.Doer.
type Doer = httpx.Doer

// Identity is who a credential belongs to. The two forges name the same thing
// differently, which is the only difference this package has to care about.
type Identity struct {
	Login    string `json:"login"`
	Username string `json:"username"`
}

// Name is the account name, whichever field the forge used.
func (i Identity) Name() string {
	if i.Login != "" {
		return i.Login
	}

	return i.Username
}

// Client talks to one forge's REST API.
type Client struct {
	do    Doer
	base  string
	token Token
}

// New builds a client. Pass the base URL from Repo.APIBase.
func New(do Doer, base string, token Token) Client {
	return Client{do: do, base: strings.TrimRight(base, "/"), token: token}
}

// HTTPClient is the redirect-refusing transport a forge credential travels
// over; see httpx.Client for why refusing matters.
func HTTPClient(timeout time.Duration) *http.Client {
	return httpx.Client(timeout)
}

// Whoami reports which account the credential belongs to.
func (c Client) Whoami(ctx context.Context) (Identity, error) {
	return call[Identity](ctx, c, http.MethodGet, userPath, nil)
}

// call sends one request to the forge and decodes the answer into T.
func call[T any](ctx context.Context, client Client, method, path string, payload any) (T, error) {
	var answer T

	if client.token == "" {
		return answer, ErrNoToken
	}

	request, err := client.newRequest(ctx, method, path, payload)
	if err != nil {
		return answer, err
	}

	body, err := client.exchange(request)
	if err != nil {
		return answer, err
	}

	err = json.Unmarshal(body, &answer)
	if err != nil {
		return answer, fmt.Errorf("reading the answer from %s: %w", client.base, err)
	}

	return answer, nil
}

// send makes one request to the forge and reports only whether it was accepted,
// without reading the answer — for a write whose reply carries nothing the
// caller needs, and which may answer with no body at all.
func send(ctx context.Context, client Client, method, path string, payload any) error {
	if client.token == "" {
		return ErrNoToken
	}

	request, err := client.newRequest(ctx, method, path, payload)
	if err != nil {
		return err
	}

	return client.accepted(request)
}

// accepted performs the request and returns nil once the forge accepted it,
// mapping the status to an error otherwise. Unlike exchange it neither requires
// nor reads a body, so an empty answer is a success rather than a syntax error.
func (c Client) accepted(request *http.Request) error {
	response, err := c.do(request)
	if err != nil {
		return httpx.Unreachable(ErrUnreachable, c.base, err)
	}
	defer func() { _ = response.Body.Close() }()

	return answerError(response)
}

// newRequest builds an authenticated request, with payload encoded as its JSON
// body when there is one.
func (c Client) newRequest(ctx context.Context, method, path string, payload any) (*http.Request, error) {
	var body io.Reader

	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encoding the request: %w", err)
		}

		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		// Unwrapped: the parse error quotes the whole URL.
		return nil, fmt.Errorf("%w: the API address could not be used", ErrUnreachable)
	}

	// GitLab accepts a bearer token for a personal access token, so both forges
	// take the same header. Its own PRIVATE-TOKEN header would be worse: Go
	// forwards a custom header to EVERY redirect target, with none of the
	// same-host check it applies to Authorization.
	request.Header.Set("Authorization", "Bearer "+c.token.Secret())
	request.Header.Set("Accept", jsonMediaType)
	request.Header.Set("User-Agent", userAgent)

	if payload != nil {
		request.Header.Set("Content-Type", jsonMediaType)
	}

	return request, nil
}

// exchange performs the request and returns the answer's body, only once the
// forge accepted the request, answered in JSON, and nothing in the answer can
// drive a terminal.
func (c Client) exchange(request *http.Request) ([]byte, error) {
	response, err := c.do(request)
	if err != nil {
		return nil, httpx.Unreachable(ErrUnreachable, c.base, err)
	}
	defer func() { _ = response.Body.Close() }()

	err = answerError(response)
	if err != nil {
		return nil, err
	}

	err = mustBeJSON(response.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, bodyLimit))
	if err != nil {
		return nil, fmt.Errorf("reading the answer from %s: %w", c.base, err)
	}

	// Every string in the answer is about to be shown, and a forge — or
	// anything answering in its place — chose it.
	return sanitize.JSON(body), nil
}

// mustBeJSON rejects an answer that is not JSON.
//
// A base URL aimed at the web host rather than the API answers 200 with a login
// page, and decoding that produces "invalid character '<'" — which tells nobody
// what went wrong.
func mustBeJSON(contentType string) error {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != jsonMediaType {
		return fmt.Errorf("%w but %q — does the API address point at the web host?",
			ErrNotJSON, contentType)
	}

	return nil
}

// answerError is the error an answer stands for, or nil once the forge
// accepted the request: a wait asked for is told apart from a refusal before
// the status is read with whatever reason the forge gave for it.
func answerError(response *http.Response) error {
	if asksToWait(response) {
		return httpx.RateLimited(response.Header)
	}

	status := statusError(response.StatusCode)
	if status != nil {
		return explained(status, response.Body)
	}

	return nil
}

// asksToWait reports an answer that is a rate limit: a 429 on either forge, or
// GitHub's 403 once its headers say no requests are left or name a wait. Only
// the headers tell that 403 from a token refused permission — its body reads
// like any refusal's — and a request always carries a User-Agent, so a 403
// without them is the permission answer.
func asksToWait(response *http.Response) bool {
	switch response.StatusCode {
	case http.StatusTooManyRequests:
		return true
	case http.StatusForbidden:
		return response.Header.Get(rateLimitRemaining) == "0" || response.Header.Get("Retry-After") != ""
	default:
		return false
	}
}

// statusError translates a response status into something a person can act
// on. A 403 is a refusal of what this token may do: answerError has already
// told a rate limit apart by its headers, so explained keeps the forge's own
// reason after it.
func statusError(status int) error {
	switch status {
	case http.StatusOK, http.StatusCreated:
		return nil
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrRefused
	case http.StatusNotFound:
		return ErrNoAPI
	default:
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, status)
	}
}

// pageQuery is query asking for one full page of a listing.
func pageQuery(query url.Values, page int) string {
	paged := url.Values{perPageParam: {strconv.Itoa(perPage)}, "page": {strconv.Itoa(page)}}
	maps.Copy(paged, query)

	return paged.Encode()
}

// readPages reads a paged listing to its end: a page short of full, or as many
// items as the forge counts in all, whichever comes first within maxPages.
// fetch reads one page and returns its items and the forge's count.
func readPages[T any](fetch func(page int) ([]T, int, error)) ([]T, error) {
	var listed []T

	for page := 1; page <= maxPages; page++ {
		items, total, err := fetch(page)
		if err != nil {
			return nil, err
		}

		listed = append(listed, items...)

		if len(items) < perPage || len(listed) >= total {
			break
		}
	}

	return listed, nil
}
