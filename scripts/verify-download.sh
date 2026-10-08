#!/usr/bin/env bash
#
# Fail unless a downloaded file matches the SHA-256 pinned for it.
#
# Usage:
#   scripts/verify-download.sh <checksums-file> <file> <tool> <version> <arch>
#
# build/Dockerfile runs this after every download, against
# build/tool-checksums.txt, whose lines read "<tool> <version> <arch> <sha256>".
# The hash is the one committed there rather than one fetched beside the
# download: a checksum from the same release page proves the file was not
# truncated, not that it was not replaced. A file with no pin, or with two,
# fails as a mismatch does, so a version bump cannot slip through unverified.
set -euo pipefail

if (($# != 5)); then
  sed -n '5,6p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
  exit 2
fi
readonly checksums="$1" file="$2" tool="$3" version="$4" arch="$5"

if [[ ! -f "${checksums}" ]]; then
  echo "verify-download: no checksums file at ${checksums}" >&2
  exit 1
fi

pins="$(awk -v tool="${tool}" -v version="${version}" -v arch="${arch}" \
  '$1 == tool && $2 == version && $3 == arch { print $4 }' "${checksums}")"
count="$(grep -c . <<<"${pins}" || true)"

if ((count == 0)); then
  echo "verify-download: no pinned SHA-256 for ${tool} ${version} ${arch} in ${checksums}" >&2
  exit 1
fi
if ((count > 1)); then
  echo "verify-download: ${tool} ${version} ${arch} is pinned ${count} times in ${checksums}" >&2
  exit 1
fi
if [[ ! "${pins}" =~ ^[0-9a-f]{64}$ ]]; then
  echo "verify-download: the pin for ${tool} ${version} ${arch} is not a SHA-256: ${pins}" >&2
  exit 1
fi

# GNU coreutils has sha256sum; stock macOS has only shasum, and the script
# tests run there too. Both print the hash first.
if command -v sha256sum >/dev/null; then
  digest="$(sha256sum "${file}")"
else
  digest="$(shasum -a 256 "${file}")"
fi
if [[ "${digest%% *}" != "${pins}" ]]; then
  echo "verify-download: ${file} does not match the SHA-256 pinned for ${tool} ${version} ${arch}" >&2
  exit 1
fi
