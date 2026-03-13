#!/usr/bin/env bash
set -euo pipefail

# Scans the repository for banned marker tokens (case-sensitive, word-boundary).
# Constructs patterns via concatenation to avoid self-matching.
# Exit 0 = clean, Exit 1 = markers found.

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

# Banned patterns: case-sensitive, word-boundary.
# Constructed via concatenation to avoid self-match.
PATTERN_NAMES=(
  "TO""DO"
  "FIX""ME"
  "ST""UB"
  "MO""CK"
  "NOT""_IMPL""EMENTED"
  "PLACE""HOLDER"
)

PATTERN_REGEXES=(
  '\bTO''DO\b'
  '\bFIX''ME\b'
  '\bST''UB\b'
  '\bMO''CK\b'
  '\bNOT''_IMPL''EMENTED\b'
  '\bPLACE''HOLDER\b'
)

FOUND=0

for i in "${!PATTERN_NAMES[@]}"; do
  name="${PATTERN_NAMES[$i]}"
  regex="${PATTERN_REGEXES[$i]}"

  if command -v rg &>/dev/null; then
    MATCHES=$(rg -c -s "$regex" \
      --glob '!node_modules/**' \
      --glob '!vendor/**' \
      --glob '!.git/**' \
      --glob '!dist/**' \
      --glob '!build/**' \
      --glob '!.next/**' \
      --glob '!.turbo/**' \
      --glob '!*.zip' \
      --glob '!go.sum' \
      --glob '!go.mod' \
      --glob '!pnpm-lock.yaml' \
      --glob '!tools/repair/scan_markers.sh' \
      "$REPO_ROOT" 2>/dev/null || true)
  else
    MATCHES=$(python3 -c "
import os, re
root = '$REPO_ROOT'
skip = {'node_modules','.git','vendor','dist','build','.next','.turbo'}
skip_files = {'go.sum','go.mod','pnpm-lock.yaml','scan_markers.sh'}
pattern = re.compile(r'$regex')
found = []
for dp, dns, fns in os.walk(root):
    dns[:] = [d for d in dns if d not in skip]
    for f in fns:
        if f in skip_files: continue
        if f.endswith('.zip'): continue
        fp = os.path.join(dp, f)
        try:
            with open(fp, 'r', errors='ignore') as fh:
                for i, line in enumerate(fh, 1):
                    if pattern.search(line):
                        found.append(f'{fp}:{i}')
        except: pass
for m in found: print(m)
" 2>/dev/null || true)
  fi

  if [ -n "$MATCHES" ]; then
    echo "BANNED MARKER FOUND: $name"
    echo "$MATCHES"
    echo "---"
    FOUND=1
  fi
done

if [ "$FOUND" -eq 1 ]; then
  echo "GATE FAILED: Banned marker tokens detected."
  exit 1
else
  echo "GATE PASSED: No banned marker tokens found."
  exit 0
fi
