BEGIN;

ALTER TABLE skills.registry
  ADD COLUMN IF NOT EXISTS deployment_mode VARCHAR(20) NOT NULL DEFAULT 'cloud';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.check_constraints
    WHERE constraint_name = 'skills_registry_deployment_mode_check'
  ) THEN
    ALTER TABLE skills.registry
      ADD CONSTRAINT skills_registry_deployment_mode_check
      CHECK (deployment_mode IN ('cloud', 'local_mac', 'mcp'));
  END IF;
END
$$;

COMMIT;
