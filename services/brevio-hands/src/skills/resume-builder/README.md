# resume-builder

Resume builder adapter for generation, tailoring, and scoring workflows.

## Auth
- No external auth required for deterministic resume composition logic.

## Input
- `action`: `generate`, `tailor`, `score`
- `role` required for `generate` and `tailor`
- `job_description` required for `tailor`
- `resume_markdown` required for `score`

## Output
- `provider`: `resume-builder`
- action echo with optional resume markdown, score, and recommendations
