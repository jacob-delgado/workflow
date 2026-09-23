---
title: "Web API errors"
weight: 25
---

# Web API errors

The `workflow --web` API answers a failed request with an
[RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) problem details object, sent
as `application/problem+json`. (The loopback, same-origin and dry-run **guards**
refuse a request as plain text *before* it reaches the API, so those few
responses are not problem details.)

```json
{
  "type": "https://jacob-delgado.github.io/workflow/docs/errors/#not-found",
  "title": "Not found",
  "status": 404,
  "detail": "issue PROJ-412 was not found",
  "code": "not_found"
}
```

- **`type`** — a stable URI naming the problem; it points at the matching section
  below.
- **`title`** — a short, fixed summary of the problem type.
- **`status`** — the HTTP status code.
- **`detail`** — what went wrong this time, safe to show. It is curated: it never
  carries a secret (tokens are redacted before an error is formed) or an internal
  host. A write that is refused may include the git or forge's own reason so you
  can act on it; a read failure and an unreachable upstream stay generic.
- **`code`** — a stable, machine-readable reason, for a client to switch on rather
  than parsing prose.

The codes below are the whole set.

## Bad request

Status 400. The request could not be understood — a malformed body, or a query
parameter that did not fit the contract.

## Not found

Status 404. The addressed resource does not exist: most often an issue that is
not in the tracker, or a `view` the configuration does not name — the issue list
and the event stream both refuse a view they do not know rather than answering
with the default one.

## Conflict

Status 409. The request cannot be applied to the current state — a working tree
with uncommitted changes, a branch that already exists, nothing staged to commit,
or no pull request to announce.

## Unprocessable

Status 422. The request was understood but cannot be carried out as asked — an
invalid configuration body, or a request for an issue when no tracker is
configured.

## Unreachable

Status 502. An upstream service — Jira or the Git forge — could not be reached.
The request was well formed; try again once the service is back.

## Internal

Status 500. An unexpected failure the caller cannot act on. The detail stays
generic on purpose; the cause is in the server's own output, not the response.
