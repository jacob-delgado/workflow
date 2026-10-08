// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// reasonLimit bounds how much of a refusal is read for its reason.
const reasonLimit = 64 << 10

// wireReason is how either forge explains a refusal. GitHub writes a message
// and a list of errors, each with its own message. GitLab writes a message that
// is a string, a list of strings, or the errors of each field by name; or, for a
// token it turns down, an error with a description and the scope it asks for.
type wireReason struct {
	Message     json.RawMessage `json:"message"`
	Error       string          `json:"error"`
	Description string          `json:"error_description"`
	Scope       string          `json:"scope"`
	Errors      []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// explained adds the forge's own reason to a status error the forge explains: a
// request it understood and turned down, such as a pull request that already
// exists, becomes a rejection. A 404 keeps its own error, and a token turned
// down is a RefusalError, made before this is reached.
func (c Client) explained(statusErr error, body io.Reader) error {
	if !errors.Is(statusErr, ErrUnexpectedStatus) {
		return statusErr
	}

	reason, given := c.reasonIn(body)
	if !given {
		return statusErr
	}

	return fmt.Errorf("%w: %s", ErrRejected, reason)
}

// reasonIn reads the forge's own reason out of a refusal's body, with the
// token masked wherever it quotes it, reporting false when it gave none that
// can be read.
func (c Client) reasonIn(body io.Reader) (string, bool) {
	raw, err := io.ReadAll(io.LimitReader(body, reasonLimit))
	if err != nil {
		return "", false
	}

	var answer wireReason

	err = json.Unmarshal(sanitize.JSON(raw), &answer)
	if err != nil {
		return "", false
	}

	reason := c.masked(answer.text())

	return reason, reason != ""
}

// masked is text with the token put as its mask wherever it appears: a gateway
// in front of a forge can quote the Authorization header back. The token is
// never empty here, since no request is sent without one, which matters:
// replacing "" would splice the mask between every character.
func (c Client) masked(text string) string {
	return strings.ReplaceAll(text, c.token.Secret(), c.token.String())
}

// text joins every part of the reason that was given, and names the scope the
// forge asked for, when it named one.
func (w wireReason) text() string {
	parts := messages(w.Message)

	for _, each := range []string{w.Error, w.Description} {
		if each != "" {
			parts = append(parts, each)
		}
	}

	for _, each := range w.Errors {
		if each.Message != "" {
			parts = append(parts, each.Message)
		}
	}

	text := strings.Join(parts, ": ")
	if w.Scope != "" {
		text += " (needs the " + w.Scope + " scope)"
	}

	return text
}

// messages reads a message that may be one string or a list of them.
func messages(raw json.RawMessage) []string {
	var one string
	if json.Unmarshal(raw, &one) == nil && one != "" {
		return []string{one}
	}

	var many []string
	if json.Unmarshal(raw, &many) == nil {
		return many
	}

	return byField(raw)
}

// byField reads GitLab's message that names each field it refused and why, as
// "field: why" in the order of the fields' names, so the answer is the same
// every time.
func byField(raw json.RawMessage) []string {
	var fields map[string][]string
	if json.Unmarshal(raw, &fields) != nil {
		return nil
	}

	names := slices.Sorted(maps.Keys(fields))
	parts := make([]string, 0, len(names))

	for _, name := range names {
		if reasons := strings.Join(fields[name], ", "); reasons != "" {
			parts = append(parts, name+": "+reasons)
		}
	}

	return parts
}

// RefusalError is a forge turning down the token a request carried: not accepting it
// (401, ErrUnauthorized) or refusing what it asked (403, ErrRefused), with the
// forge's own reason when it gave one, and which forge said so.
type RefusalError struct {
	Kind   Kind
	Status error
	Reason string
}

// Error is the status, with the forge's reason after it.
func (r *RefusalError) Error() string {
	if r.Reason == "" {
		return r.Status.Error()
	}

	return r.Status.Error() + ": " + r.Reason
}

// Unwrap is the status, so a refusal answers to ErrRefused or ErrUnauthorized.
func (r *RefusalError) Unwrap() error {
	return r.Status
}

// Advice is how a token turned down is told — the same words wherever it is
// shown — naming the forge and the scope it asks for, then what the forge said:
// false for any other failure, which keeps its own words.
func Advice(err error) (string, bool) {
	refusal := &RefusalError{Kind: KindUnknown, Status: nil, Reason: ""}
	if !errors.As(err, &refusal) {
		refusal.Status = statusOf(err)
	}

	var advice string

	switch {
	case errors.Is(refusal.Status, ErrUnauthorized):
		advice = refusal.Kind.speaker() + " did not accept the token; it may have expired or been revoked. " +
			"`workflow doctor --online` tests it."
	case errors.Is(refusal.Status, ErrRefused):
		advice = refusal.Kind.speaker() + " refused this: the token may lack " + refusal.Kind.writeScope() +
			", or your role may not allow it."
	default:
		return "", false
	}

	if refusal.Reason != "" {
		advice += " " + refusal.Kind.speaker() + " said: " + withoutAddresses(refusal.Reason)
	}

	return advice, true
}

// withoutAddresses is reason with each word that is an address put as "(an
// address)". A gateway in front of a self-managed forge can answer for it and
// point at its own internal host, and the advice is shown in the browser, where
// an address never goes.
func withoutAddresses(reason string) string {
	words := strings.Fields(reason)
	for index, word := range words {
		if strings.Contains(word, "://") {
			words[index] = "(an address)"
		}
	}

	return strings.Join(words, " ")
}

// statusOf is the token's refusal a bare error stands for, or nil.
func statusOf(err error) error {
	for _, status := range []error{ErrUnauthorized, ErrRefused} {
		if errors.Is(err, status) {
			return status
		}
	}

	return nil
}

// speaker names the forge as the subject of a sentence.
func (k Kind) speaker() string {
	if k == KindUnknown {
		return "The forge"
	}

	return k.String()
}

// writeScope names what a token needs to write on the forge.
func (k Kind) writeScope() string {
	switch k {
	case KindGitHub:
		return "the repo scope or the fine-grained permission this needs"
	case KindGitLab:
		return "the api scope"
	case KindUnknown:
		return "a scope this needs"
	default:
		return "a scope this needs"
	}
}
