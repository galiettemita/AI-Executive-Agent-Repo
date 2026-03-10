BEGIN;

DROP TABLE IF EXISTS executor_audit_log;
DROP TABLE IF EXISTS tool_side_effects;
DROP TABLE IF EXISTS tool_execution_receipts;
DROP TABLE IF EXISTS tool_executions;

COMMIT;
