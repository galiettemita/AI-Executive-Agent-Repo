DROP INDEX IF EXISTS idx_compliance_evidence_framework;
-- Note: not dropping control_id, framework, evidence_type, pass, details columns
-- as they may contain data from ongoing compliance runs.
-- To fully revert: ALTER TABLE compliance_evidence DROP COLUMN IF EXISTS control_id, etc.
