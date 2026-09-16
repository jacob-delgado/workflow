# Build container: the pinned toolchain, without installing anything on the
# host. It runs the same `task check` gate the host runs and cross-compiles the
# release binaries.
#
# Never build this by hand — `task container:build` passes the versions in from
# mise.toml (scripts/tool-versions.sh), which is what keeps this image from
# drifting away from the host toolchain. Every ARG below is required; a bare
# `docker build` fails loudly rather than silently installing something else.
#
# Usage:
#   task container:check     # lint + test + scan inside the container
#   task container:release   # cross-compiled binaries into dist/

ARG GO_VERSION=1.27.1
FROM golang:${GO_VERSION}-trixie

ARG TASK_VERSION
ARG GOLANGCI_LINT_VERSION
ARG SHFMT_VERSION
ARG YAMLLINT_VERSION
ARG TYPOS_VERSION
ARG ACTIONLINT_VERSION
ARG HADOLINT_VERSION
ARG GOVULNCHECK_VERSION
ARG GITLEAKS_VERSION
ARG GOBCO_VERSION

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# System packages. Deliberately unpinned: the base image's Debian release pins
# them, and pinning again here breaks the build on every security update.
# hadolint ignore=DL3008
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
    git ca-certificates shellcheck python3-pip jq; \
    rm -rf /var/lib/apt/lists/*

# Tools that ship as Go modules, built with the Go above so none of them can
# reject a project targeting this toolchain.
RUN set -eux; \
    : "${TASK_VERSION:?}" "${GOLANGCI_LINT_VERSION:?}" "${SHFMT_VERSION:?}"; \
    : "${GOVULNCHECK_VERSION:?}" "${GITLEAKS_VERSION:?}" "${ACTIONLINT_VERSION:?}"; \
    : "${GOBCO_VERSION:?}"; \
    go install "github.com/go-task/task/v3/cmd/task@v${TASK_VERSION}"; \
    go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v${GOLANGCI_LINT_VERSION}"; \
    go install "mvdan.cc/sh/v3/cmd/shfmt@v${SHFMT_VERSION}"; \
    go install "github.com/rhysd/actionlint/cmd/actionlint@v${ACTIONLINT_VERSION}"; \
    go install "golang.org/x/vuln/cmd/govulncheck@v${GOVULNCHECK_VERSION}"; \
    go install "github.com/zricethezav/gitleaks/v8@v${GITLEAKS_VERSION}"; \
    go install "github.com/rillig/gobco@v${GOBCO_VERSION}"; \
    rm -rf /root/.cache/go-build

# yamllint is Python; the image is not a Python project, so installing into the
# system environment is what --break-system-packages is for here.
RUN set -eux; \
    : "${YAMLLINT_VERSION:?}"; \
    pip3 install --no-cache-dir --break-system-packages "yamllint==${YAMLLINT_VERSION}"

# typos ships prebuilt binaries per architecture.
RUN set -eux; \
    : "${TYPOS_VERSION:?}"; \
    arch="$(dpkg --print-architecture)"; \
    case "${arch}" in \
    amd64) target="x86_64-unknown-linux-musl" ;; \
    arm64) target="aarch64-unknown-linux-musl" ;; \
    *) echo "unsupported architecture: ${arch}" >&2; exit 1 ;; \
    esac; \
    url="https://github.com/crate-ci/typos/releases/download/v${TYPOS_VERSION}/typos-v${TYPOS_VERSION}-${target}.tar.gz"; \
    curl -fsSL "${url}" -o /tmp/typos.tar.gz; \
    tar -xzf /tmp/typos.tar.gz -C /usr/local/bin --strip-components=1 ./typos; \
    rm -f /tmp/typos.tar.gz; \
    typos --version

# hadolint (Haskell) ships one static binary per architecture.
RUN set -eux; \
    : "${HADOLINT_VERSION:?}"; \
    arch="$(dpkg --print-architecture)"; \
    case "${arch}" in \
    amd64) target="x86_64" ;; \
    arm64) target="arm64" ;; \
    *) echo "unsupported architecture: ${arch}" >&2; exit 1 ;; \
    esac; \
    url="https://github.com/hadolint/hadolint/releases/download/v${HADOLINT_VERSION}/hadolint-Linux-${target}"; \
    curl -fsSL "${url}" -o /usr/local/bin/hadolint; \
    chmod +x /usr/local/bin/hadolint; \
    hadolint --version

ENV GOTOOLCHAIN=local \
    PATH="/go/bin:${PATH}"

WORKDIR /src

# The repo is bind-mounted at /src by the container:* tasks, so the image holds
# only the toolchain — no source, nothing to go stale between runs.
CMD ["task", "check"]
