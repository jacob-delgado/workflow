---
title: "Install"
weight: 10
---

# Install

## What it needs

- **git**, always.
- **[lefthook](https://lefthook.dev)**, for running hooks on demand and for
  turning existing `.git/hooks` into a `lefthook.yml`. Without it those two
  actions are not offered; commits still run whatever hooks git has.
- **`gh`**, optionally, as a place to find a GitHub token. See
  [Configuration]({{< relref "/docs/configuration" >}}).

## With Go

The shortest path, if you have Go installed:

```sh
go install github.com/jacob-delgado/workflow/cmd/workflow@latest
```

The binary lands in `$(go env GOPATH)/bin`, which needs to be on your `PATH`:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

For a reproducible install, name the version instead of `@latest`:

```sh
go install github.com/jacob-delgado/workflow/cmd/workflow@v0.1.0
```

`@latest` resolves to the newest release tag. Until the first tag exists, it
resolves to the most recent commit on `main` — so early installs are of
unreleased code, which is worth knowing while the configuration format is still
moving.

## From a release

Release binaries are published for macOS, Linux, and Windows on amd64 and arm64.
Download the one for your platform from the
[releases page](https://github.com/jacob-delgado/workflow/releases), along with
`SHA256SUMS`, then verify and install it:

```sh
sha256sum -c SHA256SUMS --ignore-missing
chmod +x workflow_darwin_arm64
mv workflow_darwin_arm64 /usr/local/bin/workflow
```

Every binary is built by GitHub Actions and carries a provenance attestation, so
you can confirm it came from this repository's CI rather than someone's laptop:

```sh
gh attestation verify workflow_darwin_arm64 --repo jacob-delgado/workflow
```

## From source

```sh
git clone https://github.com/jacob-delgado/workflow.git
cd workflow
mise trust && mise install
task build            # builds bin/workflow
```

This needs [mise](https://mise.jdx.dev), which provisions the pinned toolchain —
Go, the linters, and everything else the build expects. See
[Development]({{< relref "/docs/contributing" >}}) for what else is in there.

## First run

```sh
workflow config init      # writes .workflow.json here, readable only by you
workflow doctor           # says what is still missing
```

Then fill in the two tokens — [Configuration]({{< relref "/docs/configuration" >}})
explains where to get them — and run `workflow` to open the TUI.
[Using workflow]({{< relref "/docs/usage" >}}) walks through it.
