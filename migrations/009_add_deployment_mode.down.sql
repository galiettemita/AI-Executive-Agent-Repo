BEGIN;

ALTER TABLE skills.registry DROP CONSTRAINT IF EXISTS skills_registry_deployment_mode_check;
ALTER TABLE skills.registry DROP COLUMN IF EXISTS deployment_mode;

COMMIT;
