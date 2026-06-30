#!/bin/bash
set -euo pipefail

ANCHOR="BREVIO-HARNESS-V1-NO-CIRCLING-FAST-SHIPPING"
ROOT="/Users/galiettemita/Projects/Brevio/backend"
PROFILE="/Users/galiettemita/.hermes/profiles/brevio-project-manager"

files=(
  "$ROOT/.hermes/project-management-cycle.prompt.md"
  "$ROOT/.hermes/BREVIO_OPERATING_CONTRACT.md"
  "$PROFILE/BREVIO_OPERATING_CONTRACT.md"
  "$PROFILE/bin/coding-worker"
  "$ROOT/.hermes/CLAUDE_CONTINUATION_AFTER_QUOTA.md"
)

missing=0
for file in "${files[@]}"; do
  if [ ! -r "$file" ]; then
    echo "MISSING_FILE $file"
    missing=1
    continue
  fi
  if grep -Fq "$ANCHOR" "$file"; then
    echo "ANCHOR_OK $file"
  else
    echo "ANCHOR_MISSING $file"
    missing=1
  fi
done

if ! grep -Fq "NO-CIRCLING / FAST-SHIPPING / HUMAN-HARNESS RULES" "$ROOT/.hermes/project-management-cycle.prompt.md"; then
  echo "SECTION_MISSING active project-management cycle prompt"
  missing=1
fi
if ! grep -Fq "NO-CIRCLING / FAST-SHIPPING / HUMAN-HARNESS RULES" "$ROOT/.hermes/BREVIO_OPERATING_CONTRACT.md"; then
  echo "SECTION_MISSING repo-local operating contract"
  missing=1
fi
if ! grep -Fq "NO-CIRCLING / FAST-SHIPPING / HUMAN-HARNESS RULES" "$PROFILE/BREVIO_OPERATING_CONTRACT.md"; then
  echo "SECTION_MISSING profile operating contract"
  missing=1
fi
if ! grep -Fq "Required self-audit before any Brevio report" "$ROOT/.hermes/project-management-cycle.prompt.md"; then
  echo "REPORT_CHECKLIST_MISSING active project-management cycle prompt"
  missing=1
fi
if ! grep -Fq "Required self-audit before any Brevio report" "$PROFILE/BREVIO_OPERATING_CONTRACT.md"; then
  echo "REPORT_CHECKLIST_MISSING profile operating contract"
  missing=1
fi

if ! grep -Fq "Required PR review gate" "$ROOT/.hermes/project-management-cycle.prompt.md"; then
  echo "PR_REVIEW_CHECKLIST_MISSING active project-management cycle prompt"
  missing=1
fi
if ! grep -Fq "$ANCHOR" "$ROOT/.hermes/project-management-cycle.prompt.md"; then
  echo "PR_REVIEW_ANCHOR_MISSING active project-management cycle prompt"
  missing=1
fi

if [ "$missing" -ne 0 ]; then
  echo "BREVIO_HARNESS_VERIFY=FAIL"
  exit 1
fi

echo "BREVIO_HARNESS_VERIFY=PASS"
echo "Harness anchor loaded: $ANCHOR"
