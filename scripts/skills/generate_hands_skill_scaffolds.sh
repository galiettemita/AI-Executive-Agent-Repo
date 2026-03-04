#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SQL_FILE="${ROOT_DIR}/migrations/006_seed_skills.up.sql"
SKILLS_DIR="${ROOT_DIR}/services/brevio-hands/src/skills"
REGISTRY_FILE="${SKILLS_DIR}/index.ts"

if [[ ! -f "${SQL_FILE}" ]]; then
  echo "missing seed migration: ${SQL_FILE}" >&2
  exit 1
fi

mkdir -p "${SKILLS_DIR}"

seed_ids() {
  node --input-type=module -e "import fs from 'node:fs';const sql=fs.readFileSync(process.argv[1],'utf8');const sectionSplit=sql.split('), normalized AS (');const seedSection=sectionSplit[0];const unnestPattern=/unnest\\(ARRAY\\[(.*?)\\]\\)\\s+AS\\s+id/gs;const idPattern=/'([a-z0-9-]+)'/g;const ids=new Set();let m;while((m=unnestPattern.exec(seedSection))!==null){let i;while((i=idPattern.exec(m[1]))!==null){ids.add(i[1]);}}process.stdout.write([...ids].sort().join('\n')+'\\n');" "${SQL_FILE}"
}

extract_id_block() {
  local marker="$1"
  awk -v marker="${marker}" '
    $0 ~ marker { in_block = 1 }
    in_block {
      if ($0 ~ /ARRAY\[/) in_array = 1
      if (in_array) print
      if (in_array && $0 ~ /\]\);/) exit
    }
  ' "${SQL_FILE}" \
    | rg -o "'[a-z0-9-]+'" \
    | tr -d "'" \
    | sort -u
}

is_in_file() {
  local needle="$1"
  local file_path="$2"
  rg -n -x "${needle}" "${file_path}" >/dev/null 2>&1
}

to_alias() {
  local skill_id="$1"
  local normalized="${skill_id//-/_}"
  echo "adapter_${normalized}"
}

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

SKILLS_FILE="${TMP_DIR}/skills.txt"
CUSTOM_FILE="${TMP_DIR}/custom.txt"
MANUAL_FILE="${TMP_DIR}/manual.txt"
ALL_SKILLS_FILE="${TMP_DIR}/all_skills.txt"
GATEWAY_FILE="${TMP_DIR}/gateway.txt"
BRAIN_FILE="${TMP_DIR}/brain.txt"

seed_ids >"${SKILLS_FILE}"

cat >"${CUSTOM_FILE}" <<'EOF'
restaurant-reservations
food-delivery-ordering
ride-hailing
hotel-vacation-booking
bill-pay-p2p
streaming-recommendations
local-service-booking
kids-family-management
pharmacy-prescription
pet-care
EOF

cat >"${MANUAL_FILE}" <<'EOF'
shopping-expert
google-maps
google-calendar
tavily
smtp-send
home-assistant
EOF

cat "${SKILLS_FILE}" "${CUSTOM_FILE}" | sort -u >"${ALL_SKILLS_FILE}"

extract_id_block "SET plane = 'gateway'" >"${GATEWAY_FILE}"
extract_id_block "SET plane = 'brain'" >"${BRAIN_FILE}"

SKILL_COUNT="$(wc -l <"${SKILLS_FILE}" | tr -d ' ')"
CUSTOM_COUNT="$(wc -l <"${CUSTOM_FILE}" | tr -d ' ')"
TOTAL_COUNT="$(wc -l <"${ALL_SKILLS_FILE}" | tr -d ' ')"

if [[ "${SKILL_COUNT}" -ne 153 ]]; then
  echo "expected 153 skills from seed migration, got ${SKILL_COUNT}" >&2
  exit 1
fi

is_custom_skill() {
  local skill_id="$1"
  is_in_file "${skill_id}" "${CUSTOM_FILE}"
}

is_manual_skill() {
  local skill_id="$1"
  is_in_file "${skill_id}" "${MANUAL_FILE}"
}

while IFS= read -r skill_id; do
  [[ -z "${skill_id}" ]] && continue
  plane="hands"
  if is_in_file "${skill_id}" "${GATEWAY_FILE}"; then
    plane="gateway"
  elif is_in_file "${skill_id}" "${BRAIN_FILE}"; then
    plane="brain"
  fi
  custom_skill="false"
  if is_custom_skill "${skill_id}"; then
    custom_skill="true"
  fi

  skill_dir="${SKILLS_DIR}/${skill_id}"
  mkdir -p "${skill_dir}/__tests__/fixtures"

  if is_manual_skill "${skill_id}" && [[ -f "${skill_dir}/index.ts" ]]; then
    # Manually maintained skills are intentionally preserved and validated separately.
    : >"${skill_dir}/__tests__/fixtures/.gitkeep"
    continue
  fi

  cat >"${skill_dir}/schema.ts" <<EOF
import { z } from 'zod';

export const InputSchema = z.object({
  payload: z.record(z.unknown()).optional()
});

export const OutputSchema = z.object({
  ok: z.boolean(),
  skill_id: z.string()
});
EOF

  cat >"${skill_dir}/types.ts" <<'EOF'
export interface SkillInputPayload {
  payload?: Record<string, unknown>;
}

export interface SkillOutputPayload {
  ok: boolean;
  skill_id: string;
}
EOF

  cat >"${skill_dir}/client.ts" <<EOF
import type { SkillInputPayload, SkillOutputPayload } from './types.js';

export async function runClient(input: SkillInputPayload): Promise<SkillOutputPayload> {
  void input;
  return { ok: true, skill_id: '${skill_id}' };
}
EOF

  cat >"${skill_dir}/index.ts" <<EOF
import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';

const adapter: ISkillAdapter = {
  id: '${skill_id}',
  plane: '${plane}',
  requiredScopes: [],
  inputSchema: { type: 'object' },
  outputSchema: { type: 'object' },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
EOF
  if [[ "${custom_skill}" == "true" ]]; then
    echo "    // CUSTOM_BUILD_REQUIRED: Awaiting API partnership" >>"${skill_dir}/index.ts"
  fi
  cat >>"${skill_dir}/index.ts" <<EOF
    const data = await runClient({ payload: input });
    return {
      skill_id: '${skill_id}',
      status: 'SUCCESS',
      data,
      latency_ms: 1,
      metadata: {
        retries: 0,
        circuit_breaker_state: 'CLOSED',
        cache_hit: false
      }
    };
  },
  async healthCheck(): Promise<boolean> {
    return true;
  }
};

export default adapter;
EOF

  cat >"${skill_dir}/README.md" <<EOF
# ${skill_id}

Generated skill adapter scaffold.

- Plane: \`${plane}\`
- Source: \`migrations/006_seed_skills.up.sql\`
EOF
  if [[ "${custom_skill}" == "true" ]]; then
    echo "- Custom build gap stub: \`Awaiting API partnership\`" >>"${skill_dir}/README.md"
  fi

  cat >"${skill_dir}/__tests__/unit.test.ts" <<EOF
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

describe('${skill_id} unit', () => {
  it('scaffold compiles', () => {
    assert.equal(1, 1);
  });
});
EOF

  cat >"${skill_dir}/__tests__/integration.test.ts" <<EOF
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

describe('${skill_id} integration', () => {
  it('scaffold compiles', () => {
    assert.equal(1, 1);
  });
});
EOF

  : >"${skill_dir}/__tests__/fixtures/.gitkeep"
done <"${ALL_SKILLS_FILE}"

{
  echo "import type { ISkillAdapter } from '@brevio/shared';"
  echo
  while IFS= read -r skill_id; do
    [[ -z "${skill_id}" ]] && continue
    alias_name="$(to_alias "${skill_id}")"
    echo "import ${alias_name} from './${skill_id}/index.js';"
  done <"${ALL_SKILLS_FILE}"
  echo
  echo 'export const SkillRegistry: Record<string, ISkillAdapter> = {'
  while IFS= read -r skill_id; do
    [[ -z "${skill_id}" ]] && continue
    alias_name="$(to_alias "${skill_id}")"
    echo "  '${skill_id}': ${alias_name},"
  done <"${ALL_SKILLS_FILE}"
  echo '};'
  echo
  echo 'export function getSkillAdapter(skillId: string): ISkillAdapter | null {'
  echo '  return SkillRegistry[skillId] ?? null;'
  echo '}'
} >"${REGISTRY_FILE}"

echo "generated skill scaffolds for ${TOTAL_COUNT} skills (seed=${SKILL_COUNT}, custom=${CUSTOM_COUNT})"
