#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SKILLS_DIR="${ROOT_DIR}/services/brevio-hands/src/skills"
MANUAL_OVERRIDE_FILE="${ROOT_DIR}/config/skill-manual-overrides.txt"

bash "${ROOT_DIR}/scripts/skills/generate_hands_skill_scaffolds.sh"

if [[ ! -f "${MANUAL_OVERRIDE_FILE}" ]]; then
  echo "missing manual skill override list: ${MANUAL_OVERRIDE_FILE}" >&2
  exit 1
fi

manual_skills=()
while IFS= read -r skill_id; do
  [[ -z "${skill_id}" ]] && continue
  manual_skills+=("${skill_id}")
done < <(rg -v '^\s*$|^\s*#' "${MANUAL_OVERRIDE_FILE}" | sort -u)

if [[ ${#manual_skills[@]} -eq 0 ]]; then
  echo "manual skill override list is empty: ${MANUAL_OVERRIDE_FILE}" >&2
  exit 1
fi

diff_args=("--" "${SKILLS_DIR}")
for skill_id in "${manual_skills[@]}"; do
  diff_args+=(":(exclude)${SKILLS_DIR}/${skill_id}/**")
done

git diff --exit-code "${diff_args[@]}"
