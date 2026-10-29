#!/usr/bin/env bash
# Binary-level redirect check, mock only: the icu binary's intervals client,
# its Zepp data client (WithHTTPClient) and zepp login must refuse a redirect
# to another host and send it nothing, while a same-host redirect still works
# (the control that proves the refusal comes from the host change).
#
# Usage: scripts/redirect-check.sh ICU_BIN ICU_MOCK_BIN FIXTURES_DIR
# Uses a throwaway HOME/ICU_CONFIG_DIR and synthetic credentials; it starts its
# own loopback mock with -redirect-sink and never contacts a real API.
set -euo pipefail
ICU="$1" MOCK="$2" FIX="$3"
T="$(mktemp -d)"
export HOME="$T/home" ICU_CONFIG_DIR="$T/config"
mkdir -p "$HOME" "$ICU_CONFIG_DIR"
export INTERVALS_ICU_API_KEY=mock INTERVALS_ICU_ATHLETE_ID=0
export ZEPP_LOGIN_TOKEN=mock-login ZEPP_APP_TOKEN=mock-app ZEPP_USER_ID=mock-user
unset ICU_BASE_URL ZEPP_BASE_URL ZEPP_EVENTS_URL
"$MOCK" -host 127.0.0.1 -port 0 -fixtures "$FIX" -redirect-sink >"$T/mock.log" 2>&1 &
PID=$!
trap 'kill $PID 2>/dev/null || true; rm -rf "$T"' EXIT
U=""
for _ in $(seq 50); do U="$(sed -n 's/^icu-mock listening on //p' "$T/mock.log")"; [ -n "$U" ] && break; sleep 0.1; done
[ -n "$U" ] || { echo "FAIL mock did not start"; cat "$T/mock.log"; exit 1; }
fail() { echo "FAIL $*"; exit 1; }
R=(--oldest 2026-08-07 --newest 2026-08-10 --timezone America/New_York)

for m in cross307 cross308; do
  "$ICU" --base-url "$U/_redirect/$m" athlete show >/dev/null 2>"$T/e" && fail "intervals $m followed"
  grep -q "refusing redirect" "$T/e" || fail "intervals $m: $(cat "$T/e")"
  "$ICU" wellness sync "${R[@]}" --base-url "$U" --zepp-base-url "$U/_redirect/$m" --allow-insecure-zepp-host >/dev/null 2>"$T/e" && fail "zepp data $m followed"
  grep -q "refusing redirect" "$T/e" || fail "zepp data $m: $(cat "$T/e")"
  "$ICU" zepp login --email rc@example.com --password synthetic --zepp-base-url "$U/_redirect/$m" >/dev/null 2>"$T/e" && fail "zepp login $m followed"
  grep -q "refusing redirect" "$T/e" || fail "zepp login $m: $(cat "$T/e")"
done
echo "OK cross-host 307/308 refused by intervals client, Zepp data client and zepp login"

"$ICU" --base-url "$U/_redirect/same307" athlete show >/dev/null 2>"$T/e" || fail "intervals same307: $(cat "$T/e")"
"$ICU" wellness sync "${R[@]}" --base-url "$U" --zepp-base-url "$U/_redirect/same307" --allow-insecure-zepp-host >"$T/o" 2>"$T/e" || fail "zepp data same307: $(cat "$T/e")"
grep -q "would write" "$T/o" || fail "zepp data same307 output"
"$ICU" zepp login --email rc@example.com --password synthetic --zepp-base-url "$U/_redirect/same307" >/dev/null 2>"$T/e" || fail "zepp login same307: $(cat "$T/e")"
echo "OK control: same-host 307 followed by all three"

L="$(curl -sf "$U/_mock/leaks")"
python3 -c 'import json,sys; d=json.loads(sys.argv[1]); assert d["count"]==0 and not (d["apiKeyReceived"] or d["appTokenReceived"] or d["codeReceived"]), d' "$L" || fail "sink received: $L"
echo "OK sink received nothing: $L"
