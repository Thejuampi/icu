#!/usr/bin/env bash
# Fails if any tracked file is larger than the limit (default 1 MiB), so a
# built binary can never be committed again (see PR #24 and #28).
# Usage: scripts/check-large-files.sh [limit-bytes]
set -euo pipefail
limit="${1:-1048576}"
# Pre-existing data files above the limit, reviewed and allowed by path.
allow=(
  testdata/wave2_live_corpus.json
)
status=0
while IFS= read -r -d '' file; do
  [ -f "$file" ] || continue
  size="$(wc -c <"$file")"
  [ "$size" -le "$limit" ] && continue
  for ok in "${allow[@]}"; do [ "$file" = "$ok" ] && continue 2; done
  echo "tracked file over $limit bytes: $file ($size bytes)"
  status=1
done < <(git ls-files -z)
exit "$status"
