# Skill Adapters

Each skill adapter should follow:

services/brevio-hands/src/skills/{skill-id}/
- index.ts
- schema.ts
- client.ts
- types.ts
- __tests__/unit.test.ts
- __tests__/integration.test.ts
- __tests__/fixtures/
- README.md

## Scaffold Generation

Skill scaffolds are generated from `migrations/006_seed_skills.up.sql`:

```bash
./scripts/skills/generate_hands_skill_scaffolds.sh
```

This generates one adapter directory per seeded skill ID and refreshes
`services/brevio-hands/src/skills/index.ts`.
