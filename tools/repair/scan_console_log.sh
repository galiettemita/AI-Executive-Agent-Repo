#!/usr/bin/env bash
set -euo pipefail

# Scans services/ and packages/ runtime sources for console.log usage.
# Exit 0 = clean, Exit 1 = console.log found in production sources.

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

FOUND=0

for DIR in services packages; do
  TARGET="$REPO_ROOT/$DIR"
  [ -d "$TARGET" ] || continue

  if command -v rg &>/dev/null; then
    MATCHES=$(rg -n "console\.log" \
      --glob '!node_modules/**' \
      --glob '!dist/**' \
      --glob '!build/**' \
      --glob '!*.test.*' \
      --glob '!*.spec.*' \
      --glob '!__tests__/**' \
      --type ts --type js \
      "$TARGET" 2>/dev/null || true)
  else
    MATCHES=$(python3 -c "
import os, re
target = '$TARGET'
skip = {'node_modules','dist','build','__tests__'}
for dp, dns, fns in os.walk(target):
    dns[:] = [d for d in dns if d not in skip]
    for f in fns:
        if not (f.endswith('.ts') or f.endswith('.js')): continue
        if '.test.' in f or '.spec.' in f: continue
        fp = os.path.join(dp, f)
        try:
            with open(fp) as fh:
                for i, line in enumerate(fh, 1):
                    if 'console.log' in line:
                        print(f'{fp}:{i}: {line.rstrip()}')
        except: pass
" 2>/dev/null || true)
  fi

  if [ -n "$MATCHES" ]; then
    echo "console.log FOUND in $DIR/:"
    echo "$MATCHES"
    echo "---"
    FOUND=1
  fi
done

if [ "$FOUND" -eq 1 ]; then
  echo "GATE FAILED: console.log detected in production sources."
  exit 1
else
  echo "GATE PASSED: No console.log in production sources."
  exit 0
fi
