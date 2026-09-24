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

# The default is a pin, not a drift: scripts/check-go-version.sh reads it here
# and fails the lint gate unless it agrees with go.mod, mise.toml and the README.
# scripts/tool-versions.sh passes the mise.toml value in at build time, and the
# two are held equal by that check. Do not drop the default — the gate reads it.
ARG GO_VERSION=1.27.1
# Pinned by digest as well as by tag, so what gets pulled cannot change under a
# tag that was re-pushed. The digest is the multi-architecture index, so amd64
# and arm64 both resolve from it. Dependabot's docker ecosystem updates this
# line; GO_VERSION and the digest move together, and if they ever disagree the
# digest wins and GOTOOLCHAIN=local fails loudly rather than building quietly
# against the wrong Go.
FROM golang:${GO_VERSION}-trixie@sha256:9baa6b4187bbb98d240372a8a235ac0bb6b5ddd52bba1431dc2f7c0705862728

ARG TASK_VERSION
ARG GOLANGCI_LINT_VERSION
ARG LEFTHOOK_VERSION
ARG SHELLCHECK_VERSION
ARG NODE_VERSION
ARG JQ_VERSION
ARG SHFMT_VERSION
ARG YAMLLINT_VERSION
ARG TYPOS_VERSION
ARG ACTIONLINT_VERSION
ARG HADOLINT_VERSION
ARG GOVULNCHECK_VERSION
ARG GITLEAKS_VERSION
ARG GOBCO_VERSION
ARG TAPLO_VERSION
ARG ZIZMOR_VERSION
ARG MARKDOWNLINT_VERSION

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# System packages. Deliberately unpinned: the base image's Debian release pins
# them, and pinning again here breaks the build on every security update. Only
# the base tools live here — git, the cert bundle, pip for yamllint, and xz to
# unpack the .tar.xz release archives below. shellcheck, node and jq are NOT
# taken from apt: they are gates whose versions the mise.toml pins decide, so
# they are installed from those pins further down.
# hadolint ignore=DL3008
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
    git ca-certificates python3-pip xz-utils; \
    rm -rf /var/lib/apt/lists/*

# node — the runtime the npm tools run under (and the source of npm), from the
# mise.toml pin rather than the base image's apt, so the container runs the same
# node the host and CI do. Installed before the npm step below.
RUN set -eux; \
    : "${NODE_VERSION:?}"; \
    arch="$(dpkg --print-architecture)"; \
    case "${arch}" in \
    amd64) target="x64" ;; \
    arm64) target="arm64" ;; \
    *) echo "unsupported architecture: ${arch}" >&2; exit 1 ;; \
    esac; \
    url="https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${target}.tar.xz"; \
    curl -fsSL "${url}" -o /tmp/node.tar.xz; \
    tar -xJf /tmp/node.tar.xz -C /usr/local --strip-components=1; \
    rm -f /tmp/node.tar.xz; \
    node --version; \
    npm --version

# shellcheck, jq and lefthook — the remaining gates apt used to supply, now from
# their mise.toml pins. lefthook is here so the container runs the wiring tests
# that exercise it rather than skipping them for want of the binary.
RUN set -eux; \
    : "${SHELLCHECK_VERSION:?}" "${JQ_VERSION:?}" "${LEFTHOOK_VERSION:?}"; \
    arch="$(dpkg --print-architecture)"; \
    case "${arch}" in \
    amd64) shellcheck_target="x86_64"; lefthook_target="x86_64" ;; \
    arm64) shellcheck_target="aarch64"; lefthook_target="arm64" ;; \
    *) echo "unsupported architecture: ${arch}" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/koalaman/shellcheck/releases/download/v${SHELLCHECK_VERSION}/shellcheck-v${SHELLCHECK_VERSION}.linux.${shellcheck_target}.tar.xz" \
    -o /tmp/shellcheck.tar.xz; \
    tar -xJf /tmp/shellcheck.tar.xz -C /usr/local/bin --strip-components=1 "shellcheck-v${SHELLCHECK_VERSION}/shellcheck"; \
    rm -f /tmp/shellcheck.tar.xz; \
    shellcheck --version; \
    curl -fsSL "https://github.com/jqlang/jq/releases/download/jq-${JQ_VERSION}/jq-linux-${arch}" \
    -o /usr/local/bin/jq; \
    chmod +x /usr/local/bin/jq; \
    jq --version; \
    curl -fsSL "https://github.com/evilmartians/lefthook/releases/download/v${LEFTHOOK_VERSION}/lefthook_${LEFTHOOK_VERSION}_Linux_${lefthook_target}.gz" \
    | gunzip > /usr/local/bin/lefthook; \
    chmod +x /usr/local/bin/lefthook; \
    lefthook version

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
    go clean -cache -modcache

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

# taplo and zizmor ship prebuilt binaries per architecture, like typos above.
RUN set -eux; \
    : "${TAPLO_VERSION:?}" "${ZIZMOR_VERSION:?}"; \
    arch="$(dpkg --print-architecture)"; \
    case "${arch}" in \
    amd64) taplo_target="x86_64"; zizmor_target="x86_64-unknown-linux-gnu" ;; \
    arm64) taplo_target="aarch64"; zizmor_target="aarch64-unknown-linux-gnu" ;; \
    *) echo "unsupported architecture: ${arch}" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/tamasfe/taplo/releases/download/${TAPLO_VERSION}/taplo-linux-${taplo_target}.gz" \
    | gunzip > /usr/local/bin/taplo; \
    chmod +x /usr/local/bin/taplo; \
    taplo --version; \
    curl -fsSL "https://github.com/zizmorcore/zizmor/releases/download/v${ZIZMOR_VERSION}/zizmor-${zizmor_target}.tar.gz" \
    -o /tmp/zizmor.tar.gz; \
    tar -xzf /tmp/zizmor.tar.gz -C /usr/local/bin zizmor; \
    rm -f /tmp/zizmor.tar.gz; \
    zizmor --version

# markdownlint-cli2 is a Node package; npm comes from the node install above.
RUN set -eux; \
    : "${MARKDOWNLINT_VERSION:?}"; \
    npm install -g --no-fund --no-audit "markdownlint-cli2@${MARKDOWNLINT_VERSION}"; \
    npm cache clean --force; \
    markdownlint-cli2 --version

# The container:* tasks run the gate as the invoking user, who has no passwd
# entry here, so HOME names a directory any user can write: Go's build cache,
# golangci-lint's, corepack's yarn and yarn's own cache all live under it. The
# module cache moves under it too, because its default, /go/pkg/mod, sits in a
# /go/pkg that root made in the go install step above. node_modules is made
# here, open to any user, because the anonymous volume the tasks mount over
# web/node_modules copies this directory's mode. Everything under HOME stays in
# the container's writable layer and goes with --rm. Docker Desktop reports the
# bind-mounted /src itself as root's, and git refuses a checkout another uid
# owns, so the image trusts that one path.
RUN set -eux; \
    mkdir -p /home/gate /src/web/node_modules; \
    chmod 1777 /home/gate /src/web/node_modules; \
    git config --system --add safe.directory /src

ENV GOTOOLCHAIN=local \
    HOME=/home/gate \
    GOMODCACHE=/home/gate/go/pkg/mod \
    PATH="/go/bin:${PATH}"

WORKDIR /src

# The repo is bind-mounted at /src by the container:* tasks, so the image holds
# only the toolchain — no source, nothing to go stale between runs.
CMD ["task", "check"]
