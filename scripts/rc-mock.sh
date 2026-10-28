#!/usr/bin/env bash
# Start the local intervals.icu mock in the background for RC testing.
#
# Usage: scripts/rc-mock.sh [PORT]      (PORT defaults to 0 = pick a free port)
#
# Prints the port, base URL, PID and log file, plus the export/stop commands.
# RCs are tested ONLY against this mock, never against the real API.
set -euo pipefail

port="${1:-0}"
repo="$(cd "$(dirname "$0")/.." && pwd)"
workdir="$(mktemp -d "${TMPDIR:-/tmp}/icu-mock.XXXXXX")"
bin="$workdir/icu-mock"
log="$workdir/icu-mock.log"

# Remove the temp dir on any failure; on success the "Stop" line removes it.
cleanup() { [ -n "${pid:-}" ] && kill "$pid" 2>/dev/null; rm -rf "$workdir"; }
trap cleanup ERR INT TERM

(cd "$repo" && go build -o "$bin" ./tools/mockserver)

cd "$repo"
nohup "$bin" -host 127.0.0.1 -port "$port" -fixtures "$repo/testdata/fixtures" >"$log" 2>&1 &
pid=$!

url=""
for _ in $(seq 1 100); do
  url="$(sed -n 's/^icu-mock listening on //p' "$log" | head -n1)"
  [ -n "$url" ] && break
  if ! kill -0 "$pid" 2>/dev/null; then
    echo "icu-mock failed to start:" >&2
    cat "$log" >&2
    cleanup
    exit 1
  fi
  sleep 0.1
done

if [ -z "$url" ]; then
  echo "icu-mock did not report its address:" >&2
  cat "$log" >&2
  cleanup
  exit 1
fi

echo "PORT=${url##*:}"
echo "BASE_URL=$url"
echo "PID=$pid"
echo "LOG=$log"
echo
echo "# Point the CLI at the mock (flag wins over env):"
echo "export ICU_BASE_URL=$url INTERVALS_ICU_API_KEY=mock INTERVALS_ICU_ATHLETE_ID=0"
echo "# Inspect / reset writes:  curl -s $url/_mock/writes   |   curl -s -X POST $url/_mock/reset"
trap - ERR INT TERM
echo "# Stop (also removes $workdir):  kill $pid; rm -rf $workdir"
