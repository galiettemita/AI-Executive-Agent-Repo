DROP INDEX IF EXISTS idx_dsr_erasure_log_workspace;
DROP TABLE IF EXISTS dsr_erasure_log;

DROP INDEX IF EXISTS idx_purpose_audit_log_accessed_at;
DROP INDEX IF EXISTS idx_purpose_audit_log_consent_id;
DROP TABLE IF EXISTS purpose_audit_log;

DROP POLICY IF EXISTS consent_workspace_isolation ON consent_records;
DROP INDEX IF EXISTS idx_consent_records_revoked;
DROP INDEX IF EXISTS idx_consent_records_workspace_user_purpose;
DROP TABLE IF EXISTS consent_records;
