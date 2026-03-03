BEGIN;

DROP INDEX IF EXISTS idx_cost_allocations_user_created;
DROP INDEX IF EXISTS idx_usage_daily_date;
DROP INDEX IF EXISTS idx_oauth_tokens_expires_at;
DROP INDEX IF EXISTS idx_oauth_tokens_user_service;
DROP INDEX IF EXISTS idx_execution_log_status;
DROP INDEX IF EXISTS idx_execution_log_skill_created;
DROP INDEX IF EXISTS idx_execution_log_user_created;
DROP INDEX IF EXISTS idx_skills_registry_enabled;
DROP INDEX IF EXISTS idx_skills_registry_plane;
DROP INDEX IF EXISTS idx_skills_registry_category;
DROP INDEX IF EXISTS idx_sessions_user_last_activity;
DROP INDEX IF EXISTS idx_messages_intent_partial;
DROP INDEX IF EXISTS idx_messages_channel_id;
DROP INDEX IF EXISTS idx_messages_session;
DROP INDEX IF EXISTS idx_messages_user_created;
DROP INDEX IF EXISTS idx_users_tier;
DROP INDEX IF EXISTS idx_users_email_partial;
DROP INDEX IF EXISTS idx_users_phone;

COMMIT;
