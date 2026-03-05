BEGIN;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone ON public.users(phone_number) WHERE phone_number IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_partial ON public.users(email) WHERE email IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_tier ON public.users(tier);

CREATE INDEX IF NOT EXISTS idx_messages_user_created ON public.messages(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_session ON public.messages(session_id);
CREATE INDEX IF NOT EXISTS idx_messages_channel_id ON public.messages(channel_message_id);
CREATE INDEX IF NOT EXISTS idx_messages_intent_partial ON public.messages(intent) WHERE intent IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_sessions_user_last_activity ON public.sessions(user_id, last_activity_at DESC);

CREATE INDEX IF NOT EXISTS idx_skills_registry_category ON skills.registry(category);
CREATE INDEX IF NOT EXISTS idx_skills_registry_plane ON skills.registry(plane);
CREATE INDEX IF NOT EXISTS idx_skills_registry_enabled ON skills.registry(enabled);

CREATE INDEX IF NOT EXISTS idx_execution_log_user_created ON skills.execution_log(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_execution_log_skill_created ON skills.execution_log(skill_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_execution_log_status ON skills.execution_log(status);

CREATE INDEX IF NOT EXISTS idx_oauth_tokens_user_service ON auth.oauth_tokens(user_id, service);
CREATE INDEX IF NOT EXISTS idx_oauth_tokens_expires_at ON auth.oauth_tokens(expires_at);

CREATE INDEX IF NOT EXISTS idx_usage_daily_date ON billing.usage_daily(date);
CREATE INDEX IF NOT EXISTS idx_cost_allocations_user_created ON billing.cost_allocations(user_id, created_at DESC);

COMMIT;
