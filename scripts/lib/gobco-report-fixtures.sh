# shellcheck shell=bash
# shellcheck disable=SC2154 # here, workdir and report are the sourcing test's
#
# The fixtures gobco-report.sh's two tests run it with: a stand-in gobco, go
# and id, the run itself with them, and a file of Go source whose lines the
# statistics a case writes point at, in a repository of its own whose base
# branch holds it as it is, so that no line is changed until a case changes
# one. gobco-report_test.sh drives the measuring, the floors and the arms no
# test reached; gobco-report_changes_test.sh the conditions seen one way on a
# line the change touched, and the base that change is judged against.
#
# Sourced, never run, after lib/testing.sh, with ${report} naming the report:
#
#   # shellcheck source=lib/gobco-report-fixtures.sh
#   source "${here}/lib/gobco-report-fixtures.sh"
#
# The stand-in gobco says which package or file it measured, writes
# GOBCO_STUB_STATS to the file after -stats, or nothing when that is empty, or
# no condition at all for the package GOBCO_STUB_EMPTY names, ignores its other
# flags, and fails as gobco does on a package it cannot read when its package
# or file is GOBCO_STUB_UNREADABLE. A go ahead of the real one answers `go
# version -m`, which cannot read a script, with the Go this project builds with
# or GO_STUB_BUILT_BY, and adds GO_STUB_UNTESTED to the packages listed without
# tests; everything else reaches the real go, so the untested packages checked
# are the module's own. An id ahead of the real one answers `id -u` with
# ID_STUB_UID, or a user's by default. OUT_DIR is always a temporary directory,
# because the report empties the directory it writes to. GOBCO_ALLOWLIST and
# GOBCO_FLOORS name an empty file unless a case writes its own, so the
# repository's own lines play no part. GOBCO_CHANGES_ROOT names the fixture's
# own repository, and GATE_BASE its base branch, unless a case names others,
# or GATE_BASE_UNSET asks for none at all, so the changes judged are never
# the repository's own.

# A gate run hands its base on, so a case names its own or none.
unset GATE_BASE GOBCO_CHANGES_ROOT

real_go="$(command -v go)"
readonly real_go
module="$(cd "${here}/.." && "${real_go}" list -m)"
readonly module
# shellcheck disable=SC2034 # read by the tests that source this
readonly buildinfo="${module}/internal/buildinfo"

mkdir -p "${workdir}/bin"
cat >"${workdir}/bin/gobco" <<'STUB'
#!/usr/bin/env bash
package="${*: -1}"
if [[ -n "${GOBCO_STUB_UNREADABLE:-}" && "${package}" == "${GOBCO_STUB_UNREADABLE}" ]]; then
  echo "gobco: ${package}: cannot load the package" >&2
  exit 1
fi
echo "gobco: measured ${package}"
stats="${GOBCO_STUB_STATS:-}"
if [[ "${package}" == "${GOBCO_STUB_EMPTY:-}" ]]; then
  stats='[]'
fi
while (($# > 0)); do
  if [[ "$1" == "-stats" && -n "${stats}" ]]; then
    printf '%s\n' "${stats}" >"$2"
  fi
  shift
done
STUB
cat >"${workdir}/bin/go" <<'STUB'
#!/usr/bin/env bash
if [[ "$1" == "version" && "$2" == "-m" ]]; then
  printf '%s: %s\n' "$3" "${GO_STUB_BUILT_BY:-$("${REAL_GO}" env GOVERSION)}"
  exit 0
fi
"${REAL_GO}" "$@" || exit
if [[ "$1" == "list" && "$3" == "{{if not"* && -n "${GO_STUB_UNTESTED:-}" ]]; then
  printf '%s\n' "${GO_STUB_UNTESTED}"
fi
STUB
cat >"${workdir}/bin/id" <<'STUB'
#!/usr/bin/env bash
if [[ "$*" == "-u" ]]; then
  echo "${ID_STUB_UID:-1000}"
  exit 0
fi
exec /usr/bin/id "$@"
STUB
chmod +x "${workdir}/bin/gobco" "${workdir}/bin/go" "${workdir}/bin/id"

readonly no_lines="${workdir}/no-lines.txt"
: >"${no_lines}"

# expect runs the report with the stand-ins, and compares its exit and output.
#   expect <pass|fail> <name> <stats> <untested> <want in output> [arg...]
expect() {
  local -a base=("GATE_BASE=${GATE_BASE-base}")
  if [[ -n "${GATE_BASE_UNSET:-}" ]]; then base=(); fi
  expect_output "$1" "$2" "$5" env "${base[@]}" PATH="${workdir}/bin:${PATH}" REAL_GO="${real_go}" \
    OUT_DIR="${workdir}/out" GOBCO_STUB_STATS="$3" GO_STUB_UNTESTED="$4" \
    GOBCO_ALLOWLIST="${GOBCO_ALLOWLIST:-${no_lines}}" GOBCO_FLOORS="${GOBCO_FLOORS:-${no_lines}}" \
    GOBCO_CHANGES_ROOT="${GOBCO_CHANGES_ROOT:-${fixture_repo}}" "${report}" "${@:6}"
}

# A fixture of Go source gives the conditions a file to stand in, so the
# report can name the function each lies in; its lines are the ones the
# statistics the cases write point at.
readonly fixture_repo="${workdir}/fixture"
readonly fixture="${fixture_repo}/reader.go"
mkdir -p "${fixture_repo}"
cat >"${fixture}" <<'GO'
package fixture

func (r *reader[T]) read() error {
	err := r.open()
	if err != nil {
		return err
	}

	return r.close()
}

func ready(open bool, err error) bool {
	return open && err == nil
}

func (w *watcher) wait(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if context.Background().Err() != nil {
		return nil
	}

	if _, errParse := strconv.Atoi(w.text); errParse != nil {
		return errParse
	}

	return w.handle(w.errorHandler != nil)
}

func (c *cache[K, V]) get(key K) error {
	if err := c.read(key); err != nil {
		return err
	}

	return nil
}

func remove(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if err := os.Remove(path); err != nil &&
		errors.Is(err, fs.ErrPermission) {
		return nil
	}

	if ok := exists(path); ok && errors.Is(lastErr, fs.ErrExist) {
		return nil
	}

	return nil
}

func (f *file) close() error {
	if closeErr := f.handle.Close(); closeErr != nil {
		return closeErr
	}

	if err2 := f.sync(); err2 != nil {
		return err2
	}

	return nil
}
GO

# commit commits everything in the fixture's repository.
#   commit <message>
commit() {
  git -C "${fixture_repo}" add -A
  git -C "${fixture_repo}" -c user.email=t@example.com -c user.name=t commit --quiet -m "$1"
}
git init --quiet --initial-branch=base "${fixture_repo}"
commit "the fixture"

# condition_in writes the statistics of one condition at a line and column of
# a file: its code, and how often it was seen true and false. condition does
# so for the fixture.
#   condition_in <file> <line:col> <code> <true count> <false count>
#   condition <line:col> <code> <true count> <false count>
condition_in() {
  printf '{"Start":"%s:%s","Code":"%s","TrueCount":%s,"FalseCount":%s}' "$1" "$2" "$3" "$4" "$5"
}
condition() {
  condition_in "${fixture}" "$@"
}

# allowlist writes an allowlist holding the given lines, and names it.
#   allowlist <name> [line...]
allowlist() {
  local file="${workdir}/allowlist-$1.txt"
  shift
  printf '%s\n' "# kept arms" "$@" >"${file}"
  printf '%s' "${file}"
}
