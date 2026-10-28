#!/usr/bin/env bash
# Tests for coverage-floor.sh. Run: bash scripts/coverage-floor_test.sh
set -uo pipefail
dir="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
expect() { # name want_exit file pkgs...
  local name="$1" want="$2" file="$3"
  shift 3
  bash "$dir/coverage-floor.sh" "$file" 90.0 "$@" >"$tmp/log" 2>&1
  local got=$?
  if [ "$got" -ne "$want" ]; then
    echo "FAIL $name: exit $got, want $want"
    cat "$tmp/log"
    fails=1
  else
    echo "ok   $name"
  fi
}
printf 'ok  \tex/a\t1.0s\tcoverage: 91.2%% of statements\nok  \tex/b\t1.0s\tcoverage: 90.0%% of statements\n' >"$tmp/pass"
expect "all above floor" 0 "$tmp/pass" ex/a ex/b
printf 'ok  \tex/a\t1.0s\tcoverage: 91.2%% of statements\nok  \tex/b\t1.0s\tcoverage: 89.9%% of statements\n' >"$tmp/low"
expect "one below floor" 1 "$tmp/low" ex/a ex/b
printf 'ok  \tex/a\t1.0s\tcoverage: 91.2%% of statements\n\tex/c\t\tcoverage: 0.0%% of statements\n' >"$tmp/go126"
expect "go1.26 no-tests line" 1 "$tmp/go126" ex/a ex/c
printf 'ok  \tex/a\t1.0s\tcoverage: 91.2%% of statements\n?   \tex/c\t[no test files]\n' >"$tmp/old"
expect "pre-1.26 no test files" 1 "$tmp/old" ex/a ex/c
expect "package missing from output" 1 "$tmp/pass" ex/a ex/b ex/d
expect "no packages" 1 "$tmp/pass"
exit "$fails"
