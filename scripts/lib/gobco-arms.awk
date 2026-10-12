# The check of the arms no test reached, which gobco-report.sh runs once it
# has measured every package and read every condition:
#
#   LC_ALL=C awk -F'\t' -v allowlist=<file> -v whole=<1 or 0> -v goos=<GOOS> \
#     -f scripts/lib/gobco-arms.awk <<<"<conditions>"
#
# An arm no test reached fails: a condition never evaluated, and an error
# check whose error arm was never seen. An error check compares with nil a
# name ending in err or Err, perhaps with a capitalized or numbered rest
# (err2, errParse, closeErr), or a call of a method so named (ctx.Err()),
# either reached through fields and calls; its error arm is the one the
# error takes. So is an errors.Is or !errors.Is of such a name that follows
# "<name> != nil &&", on its line or the one before, since only an error
# reaches it: its error arm is the one the check's body takes. gobco reports
# the call without its !, so that arm is the call's true one, or its false
# one where a ! comes before it. Columns count bytes, as Go's do, so the awk
# reads in the C locale.
#
# Each arm is named by its file, line and function, and by its number among
# the conditions its function holds that read the same, counted from the
# top: the first "err != nil" in a function is 1, the next 2. A line of the
# allowlist keeps one arm for the trade-off it names, on the systems it names
# ("any", or GOOS values joined by commas), by that number rather than its
# line, which drifts:
#
#   <file> <function> <TRADE-n> <systems> <number> <condition>
#
# So a kept arm a test comes to reach is not traded for a new one no test
# reaches that reads the same, added below it in the same function: the new
# one has a number of its own. A condition that reads the same added above a
# kept arm moves the arm's number on, and the gate fails until the line
# follows it, with one exception it cannot see: when a test comes to reach the
# kept arm in the same change, the new arm takes the kept one's number, the
# line keeps it, and the gate passes. Trade-off TRADE-19: without source lines
# the two cannot be told apart. A line whose arm is no longer unseen fails
# too, so the list only shrinks as tests arrive. A run over named packages
# misses only lines for files it did not measure.
#
# It reads the conditions a line each, "<file>:<line>:<column>", the code, and
# how often each was seen true and false, apart by tabs and sorted by file,
# line and column, and fails naming each arm no test reached that no line of
# the allowlist keeps, and each line of the allowlist that keeps none. whole is
# 1 for a run over the module's roots, which answers for every line of the
# allowlist, and 0 for one over named packages.

# enclosing is the function file holds at line, read on from where the call
# before left it, since the conditions come in order. It leaves that line
# and the one before it in line_text and previous_text.
function enclosing(file, line,    text) {
  if (file != open_file) {
    if (open_file != "") close(open_file)
    open_file = file
    read_to = 0
    current = "(no function)"
    line_text = previous_text = ""
  }
  while (read_to < line && (getline text < file) > 0) {
    read_to++
    previous_text = line_text
    line_text = text
    if (text ~ /^func /) current = function_name(text)
    else if (text ~ /^(var|const|type|import)[ (]/) current = "(no function)"
  }
  return current
}
function function_name(text,    receiver, at) {
  sub(/^func /, "", text)
  if (substr(text, 1, 1) != "(") return name_of(text)
  at = index(text, ")")
  receiver = substr(text, 2, at - 2)
  text = substr(text, at + 1)
  sub(/^ +/, "", text)
  sub(/\[.*/, "", receiver)
  sub(/.* /, "", receiver)
  gsub(/[*]/, "", receiver)
  return receiver "." name_of(text)
}
function name_of(text) {
  match(text, /^[A-Za-z_][A-Za-z0-9_]*/)
  return substr(text, 1, RLENGTH)
}
# errors_is_arm is the error's arm of code, at column of the line enclosing
# read last, when it is an errors.Is of an error check that follows "<it>
# != nil &&": "true", or "false" where a ! before it turns it round; and ""
# for any other condition.
function errors_is_arm(column, code,    name, before, arm, after) {
  if (!match(code, /^errors[.]Is[(][^,]*,/)) return ""
  name = substr(code, 11, RLENGTH - 11)
  if (name !~ (error_check "$")) return ""
  before = substr(line_text, 1, column - 1)
  arm = "true"
  if (before ~ /![ \t]*$/) {
    arm = "false"
    sub(/![ \t]*$/, "", before)
  }
  if (before ~ /^[ \t]*$/) before = previous_text " "
  sub(/[ \t]+$/, " ", before)
  after = name " != nil && "
  if (length(before) < length(after) || substr(before, length(before) - length(after) + 1) != after) return ""
  return arm
}
function applies(systems,    each, count, i) {
  if (systems == "any") return 1
  count = split(systems, each, ",")
  for (i = 1; i <= count; i++) if (each[i] == goos) return 1
  return 0
}
BEGIN {
  error_check = "^([A-Za-z_][A-Za-z0-9_]*([(][)])?[.])*[A-Za-z0-9_]*[Ee]rr([A-Z0-9][A-Za-z0-9_]*)?([(][)])?"
  while ((getline line < allowlist) > 0) {
    number++
    if (line ~ /^[ \t]*(#|$)/) continue
    fields = split(line, field, /[ \t]+/)
    if (field[1] == "") { for (i = 1; i < fields; i++) field[i] = field[i + 1]; fields-- }
    if (fields < 6 || field[3] !~ /^TRADE-[1-9][0-9]*$/ || field[4] !~ /^(any|[a-z0-9]+(,[a-z0-9]+)*)$/ ||
      field[5] !~ /^[1-9][0-9]*$/) {
      printf "%s:%d: want <file> <function> <TRADE-n> <systems> <number> <condition>\n", allowlist, number > "/dev/stderr"
      bad = 1
      continue
    }
    if (!applies(field[4])) continue
    condition = field[6]
    for (i = 7; i <= fields; i++) condition = condition " " field[i]
    arm = field[1] SUBSEP field[2] SUBSEP field[5] SUBSEP condition
    if (arm in allowed) {
      printf "%s:%d: keeps the arm line %d keeps already\n", allowlist, number, allowed[arm] > "/dev/stderr"
      bad = 1
      continue
    }
    allowed[arm] = number
    file[arm] = field[1]
    text[arm] = field[1] " " field[2] " " field[3] " " field[4] " " field[5] " " condition
  }
  allowlist_lines = number
}
{
  path = $1
  sub(/:[0-9]+:[0-9]+$/, "", path)
  measured[path] = 1
  split($1, position, ":")
  function_at = enclosing(path, position[2] + 0)
  nth = ++written[path SUBSEP function_at SUBSEP $2]
  if ($3 == 0 && $4 == 0) why = "was never evaluated"
  else if ($2 ~ (error_check " != nil$") && $3 == 0) why = "was never seen true, the error's arm"
  else if ($2 ~ (error_check " == nil$") && $4 == 0) why = "was never seen false, the error's arm"
  else if (($3 == 0 || $4 == 0) && (errors_arm = errors_is_arm(position[3] + 0, $2)) != "" &&
    (errors_arm == "true" ? $3 : $4) == 0) why = "was never seen " errors_arm ", the error's arm"
  else next
  arm = path SUBSEP function_at SUBSEP nth SUBSEP $2
  unseen[arm] = 1
  if (arm in allowed) next
  missed[++misses] = sprintf("  %s in %s: \"%s\", number %d there, %s", $1, function_at, $2, nth, why)
}
END {
  for (arm in allowed) {
    if (arm in unseen || (!whole && !(file[arm] in measured))) continue
    stale[allowed[arm]] = text[arm]
  }
  for (i = 1; i <= allowlist_lines; i++) {
    if (!(i in stale)) continue
    printf "%s:%d: no unseen arm is left for \"%s\"; remove the line\n", allowlist, i, stale[i] > "/dev/stderr"
    bad = 1
  }
  if (misses > 0) {
    print "Condition arms no test reached, and no trade-off keeps:" > "/dev/stderr"
    for (i = 1; i <= misses; i++) print missed[i] > "/dev/stderr"
    print "Each is a test to write. One a kept trade-off in TECH_DEBT.md keeps goes in" > "/dev/stderr"
    print "the allowlist instead, a line an arm, naming that trade-off." > "/dev/stderr"
    bad = 1
  }
  exit bad
}
