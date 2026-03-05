BEGIN;

CREATE SCHEMA IF NOT EXISTS billing;

CREATE TABLE IF NOT EXISTS billing.usage_daily (
  user_id UUID NOT NULL REFERENCES public.users(id),
  date DATE NOT NULL,
  total_messages INTEGER NOT NULL DEFAULT 0,
  total_skill_executions INTEGER NOT NULL DEFAULT 0,
  total_llm_tokens INTEGER NOT NULL DEFAULT 0,
  total_cost_cents NUMERIC(10,4) NOT NULL DEFAULT 0,
  breakdown_by_skill JSONB NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY (user_id, date)
);

CREATE TABLE IF NOT EXISTS billing.monthly_budgets (
  user_id UUID NOT NULL REFERENCES public.users(id),
  yyyymm CHAR(6) NOT NULL,
  llm_budget_cents INTEGER NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, yyyymm)
);

CREATE TABLE IF NOT EXISTS billing.cost_allocations (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id),
  source VARCHAR(50) NOT NULL,
  source_id VARCHAR(120),
  amount_cents NUMERIC(10,4) NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMIT;
