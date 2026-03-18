ALTER TABLE dpo_rounds DROP COLUMN IF EXISTS sampling_rate;
ALTER TABLE dpo_rounds DROP COLUMN IF EXISTS sigma;
ALTER TABLE dpo_rounds DROP COLUMN IF EXISTS dp_delta;
ALTER TABLE dpo_rounds DROP COLUMN IF EXISTS dp_epsilon;

DROP INDEX IF EXISTS idx_workspace_dp_budgets_workspace_id;
DROP TABLE IF EXISTS workspace_dp_budgets;
