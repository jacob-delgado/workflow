# shellcheck shell=bash
#
# The pins the cloud toolchain is installed from, as a checkout holds them:
# the version mise.toml pins for each tool scripts/cloud-session-setup.sh
# installs. The setup reads its versions here and stamps the toolchain with
# the digest of them all, and scripts/gate-as-dev.sh reads the same from the
# commit it gates and refuses one whose digest has no stamp. So the two read
# one list in one order, and a change to mise.toml that moves none of these,
# a tool the setup leaves out or a line of [env], leaves the digest as it was.
#
# Sourced, never run; the setup installs it beside its copy of the gate:
#
#   # shellcheck source=lib/toolchain-pins.sh
#   source "${here}/lib/toolchain-pins.sh"

# sha256_hex prints the SHA-256 of what it reads, with GNU's sha256sum or, as
# on stock macOS, shasum.
sha256_hex() {
  local sha256_line
  if command -v sha256sum >/dev/null; then
    sha256_line="$(sha256sum)"
  else
    sha256_line="$(shasum -a 256)"
  fi
  printf '%s' "${sha256_line%% *}"
}

# mise_pin prints the version the mise.toml it reads pins for a [tools] key,
# and nothing when it pins none.
#   mise_pin <key>
mise_pin() {
  local escaped_key="${1//\//\\/}"
  sed -n -E "s/^\"?${escaped_key}\"?[[:space:]]*=[[:space:]]*\"([^\"]+)\".*/\1/p" | head -1
}

# toolchain_pins prints "<name> <pin>" a line for each pin the toolchain is
# installed from, reading the checkout's mise.toml with the command given, the
# file's path after its arguments. A tool mise.toml does not pin prints with
# no pin; the setup stops on it, so no stamp names one.
#   toolchain_pins <read>...
toolchain_pins() {
  # Named apart from the sourcing script's own, which may be readonly.
  local -a pinned_tools=(go node shellcheck jq lefthook typos hadolint taplo
    zizmor rumdl task golangci-lint shfmt actionlint
    go:golang.org/x/vuln/cmd/govulncheck go:github.com/zricethezav/gitleaks/v8
    go:github.com/rillig/gobco go:golang.org/x/tools/cmd/deadcode)
  local mise_text pinned_tool
  mise_text="$("$@" mise.toml)" || mise_text=""
  for pinned_tool in "${pinned_tools[@]}"; do
    printf '%s %s\n' "${pinned_tool}" "$(mise_pin "${pinned_tool}" <<<"${mise_text}")"
  done
}

# toolchain_digest prints the SHA-256 of toolchain_pins, which names the
# toolchain's stamp.
#   toolchain_digest <read>...
toolchain_digest() {
  toolchain_pins "$@" | sha256_hex
}
