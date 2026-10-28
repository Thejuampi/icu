#!/usr/bin/env bash
# Usage: coverage-floor.sh TEST_OUT FLOOR PKG...
# Every listed package must have an "ok" line in TEST_OUT with coverage >= FLOOR.
# A package without one (no tests: Go 1.26+ prints "\tpkg\t\tcoverage: 0.0% ...",
# older Go prints "?   pkg [no test files]") fails.
set -euo pipefail
test_out="$1"
floor="$2"
shift 2
[ "$#" -gt 0 ] || { echo "no packages given"; exit 1; }
status=0
for pkg in "$@"; do
  pct="$(awk -v pkg="$pkg" '$1 == "ok" && $2 == pkg {
      for (i = 1; i <= NF; i++) if ($i == "coverage:") { p = $(i+1) } }
    END { print p }' "$test_out")"
  if [ -z "$pct" ] || [ "$pct" = "[no" ]; then
    echo "FAIL $pkg: no tests / no coverage line"
    status=1
    continue
  fi
  pct="${pct%\%}"
  if awk -v p="$pct" -v f="$floor" 'BEGIN { exit !(p + 0 < f + 0) }'; then
    echo "FAIL $pkg coverage ${pct}% < ${floor}%"
    status=1
  else
    echo "ok   $pkg coverage ${pct}% >= ${floor}%"
  fi
done
exit "$status"
