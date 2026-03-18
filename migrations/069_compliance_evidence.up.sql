-- Extend compliance_evidence with structured evidence fields.
-- The table may already exist from 056_dsr_evidence; add new columns safely.
ALTER TABLE compliance_evidence ADD COLUMN IF NOT EXISTS control_id TEXT;
ALTER TABLE compliance_evidence ADD COLUMN IF NOT EXISTS framework TEXT;
ALTER TABLE compliance_evidence ADD COLUMN IF NOT EXISTS evidence_type TEXT;
ALTER TABLE compliance_evidence ADD COLUMN IF NOT EXISTS pass BOOLEAN;
ALTER TABLE compliance_evidence ADD COLUMN IF NOT EXISTS details JSONB;

-- Backfill control_id for existing rows that lack it.
UPDATE compliance_evidence SET control_id = 'legacy' WHERE control_id IS NULL;
UPDATE compliance_evidence SET framework = 'soc2' WHERE framework IS NULL;
UPDATE compliance_evidence SET evidence_type = 'dsr' WHERE evidence_type IS NULL;
UPDATE compliance_evidence SET pass = true WHERE pass IS NULL;

-- Now make columns NOT NULL with defaults for future inserts.
ALTER TABLE compliance_evidence ALTER COLUMN control_id SET NOT NULL;
ALTER TABLE compliance_evidence ALTER COLUMN control_id SET DEFAULT '';
ALTER TABLE compliance_evidence ALTER COLUMN framework SET NOT NULL;
ALTER TABLE compliance_evidence ALTER COLUMN framework SET DEFAULT 'soc2';
ALTER TABLE compliance_evidence ALTER COLUMN evidence_type SET NOT NULL;
ALTER TABLE compliance_evidence ALTER COLUMN evidence_type SET DEFAULT '';
ALTER TABLE compliance_evidence ALTER COLUMN pass SET NOT NULL;
ALTER TABLE compliance_evidence ALTER COLUMN pass SET DEFAULT true;

CREATE INDEX IF NOT EXISTS idx_compliance_evidence_control_id ON compliance_evidence(control_id);
CREATE INDEX IF NOT EXISTS idx_compliance_evidence_collected_at ON compliance_evidence(collected_at);
CREATE INDEX IF NOT EXISTS idx_compliance_evidence_framework ON compliance_evidence(framework);
