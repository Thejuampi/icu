#!/usr/bin/env bash
# Fails if any tracked file is larger than the limit (default 1 MiB), so a
# built binary can never be committed again (see PR #24 and #28).
# Files above the limit must be pinned below by sha256, so any change to them
# fails CI until reviewed and re-pinned; nothing may exceed the hard cap.
# Usage: scripts/check-large-files.sh [limit-bytes]
set -euo pipefail
limit="${1:-1048576}"
hard_cap=2097152 # 2 MiB: no tracked file may exceed this, allowlisted or not
# path=sha256 of reviewed data files above the limit.
allow=(
  "testdata/wave2_live_corpus.json=7ed6dd5fb274edcb471a1daeb5c4efa07939f4deeae14c987e2a26304b28fd86"
)
status=0
while IFS= read -r -d '' file; do
  [ -f "$file" ] || continue
  size="$(wc -c <"$file")"
  if [ "$size" -gt "$hard_cap" ]; then
    echo "tracked file over the $hard_cap-byte hard cap: $file ($size bytes)"
    status=1
    continue
  fi
  [ "$size" -le "$limit" ] && continue
  sum="$(sha256sum "$file" | cut -d' ' -f1)"
  for ok in "${allow[@]}"; do
    if [ "$file" = "${ok%%=*}" ]; then
      [ "$sum" = "${ok#*=}" ] && continue 2
      echo "allowlisted file changed: $file (sha256 $sum, pinned ${ok#*=}); review and re-pin"
      status=1
      continue 2
    fi
  done
  echo "tracked file over $limit bytes: $file ($size bytes)"
  status=1
done < <(git ls-files -z)
exit "$status"
