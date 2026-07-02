#!/bin/bash
set -euo pipefail

ANCHOR="BREVIO-HARNESS-V1-NO-CIRCLING-FAST-SHIPPING"
ROOT="/Users/galiettemita/Projects/Brevio/backend"
PROFILE="/Users/galiettemita/.hermes/profiles/brevio-project-manager"

required_files=(
  "$ROOT/.hermes/BREVIO_CONSTITUTION.md"
  "$ROOT/.hermes/BREVIO_OPERATING_CONTRACT.md"
  "$ROOT/.hermes/ACTIVE_PHASE_CONTRACT.md"
  "$ROOT/.hermes/NEXT_PR_QUEUE.md"
  "$ROOT/.hermes/project-management-cycle.prompt.md"
  "$ROOT/.hermes/BREVIO_REPORT_TEMPLATE.md"
  "$ROOT/.hermes/M1_B_CLOSEOUT.md"
  "$ROOT/.hermes/verify-brevio-harness.sh"
  "$PROFILE/scripts/build-brevio-cycle-context.sh"
  "$PROFILE/scripts/brevio-cycle.sh"
  "$PROFILE/bin/coding-worker"
)

missing=0
for file in "${required_files[@]}"; do
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

executable_files=(
  "$ROOT/.hermes/verify-brevio-harness.sh"
  "$PROFILE/scripts/build-brevio-cycle-context.sh"
  "$PROFILE/scripts/brevio-cycle.sh"
  "$PROFILE/bin/coding-worker"
)
for file in "${executable_files[@]}"; do
  if [ ! -x "$file" ]; then
    echo "NOT_EXECUTABLE $file"
    missing=1
  fi
done

if [ -r "$ROOT/.hermes/NEXT_PR_QUEUE.md" ]; then
  next_count=$(grep -Ec '^## NEXT — ' "$ROOT/.hermes/NEXT_PR_QUEUE.md" || true)
  if [ "$next_count" -ne 1 ]; then
    echo "NEXT_QUEUE_COUNT_INVALID count=$next_count expected=1"
    missing=1
  else
    echo "NEXT_QUEUE_COUNT_OK count=1"
  fi
fi

if ! grep -Fq "BREVIO NO-CIRCLE HARNESS SHIPPING MODE" "$ROOT/.hermes/BREVIO_OPERATING_CONTRACT.md"; then
  echo "NO_CIRCLE_MODE_MISSING operating contract"
  missing=1
fi
if ! grep -Eq "^## Current phase$" "$ROOT/.hermes/ACTIVE_PHASE_CONTRACT.md"; then
  echo "ACTIVE_PHASE_MISSING current phase heading"
  missing=1
fi
if ! grep -Eq "^## Memory V1 exit condition$" "$ROOT/.hermes/ACTIVE_PHASE_CONTRACT.md"; then
  echo "ACTIVE_PHASE_MISSING Memory V1 exit condition"
  missing=1
fi
if ! grep -Fq "Current NEXT queue item" "$ROOT/.hermes/BREVIO_REPORT_TEMPLATE.md"; then
  echo "REPORT_TEMPLATE_MISSING NEXT queue field"
  missing=1
fi
if ! grep -Fq "build-brevio-cycle-context.sh" "$PROFILE/scripts/brevio-cycle.sh"; then
  echo "MECHANICAL_CONTEXT_INJECTION_MISSING brevio-cycle.sh"
  missing=1
fi

if [ "$missing" -ne 0 ]; then
  echo "BREVIO_HARNESS_VERIFY=FAIL"
  exit 1
fi

echo "BREVIO_HARNESS_VERIFY=PASS"
echo "Harness anchor loaded: $ANCHOR"
