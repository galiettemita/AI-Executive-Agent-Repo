-- Migration 014: Gateway production hardening
-- Creates gateway_queue, gateway_dedup, gateway_nonces tables for durable ingress.
-- Creates outbox view aliasing outbox_items for outbox service compatibility.
-- Adds gateway-specific idempotency columns to idempotency_keys table.

BEGIN;

-- =============================================================================
-- Gateway deduplication table
-- =============================================================================
CREATE TABLE IF NOT EXISTS gateway_dedup (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  dedup_hash text NOT NULL,
  message_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, dedup_hash)
);
ALTER TABLE gateway_dedup ENABLE ROW LEVEL SECURITY;
CREATE POLICY gateway_dedup_rls ON gateway_dedup
  USING (workspace_id = current_setting('app.workspace_id')::uuid);

-- =============================================================================
-- Gateway nonce replay protection table
-- =============================================================================
CREATE TABLE IF NOT EXISTS gateway_nonces (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  nonce text NOT NULL,
  message_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, nonce)
);
ALTER TABLE gateway_nonces ENABLE ROW LEVEL SECURITY;
CREATE POLICY gateway_nonces_rls ON gateway_nonces
  USING (workspace_id = current_setting('app.workspace_id')::uuid);

-- =============================================================================
-- Gateway durable message queue
-- =============================================================================
CREATE TABLE IF NOT EXISTS gateway_queue (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  ingress_turn_id uuid NOT NULL REFERENCES ingress_turns(id),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  channel text NOT NULL,
  channel_identifier text NOT NULL,
  user_channel_id text NOT NULL,
  group_key text NOT NULL DEFAULT '',
  dedup_key text NOT NULL DEFAULT '',
  payload jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE gateway_queue ENABLE ROW LEVEL SECURITY;
CREATE POLICY gateway_queue_rls ON gateway_queue
  USING (workspace_id = current_setting('app.workspace_id')::uuid);

CREATE INDEX idx_gateway_queue_created_at ON gateway_queue(created_at ASC);
CREATE INDEX idx_gateway_dedup_workspace ON gateway_dedup(workspace_id, dedup_hash);
CREATE INDEX idx_gateway_nonces_workspace ON gateway_nonces(workspace_id, nonce);

-- =============================================================================
-- Gateway idempotency cache (DB-backed HTTP response cache)
-- =============================================================================
CREATE TABLE IF NOT EXISTS gateway_idempotency_cache (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  idempotency_key text NOT NULL UNIQUE,
  status_code int NOT NULL,
  response_body bytea,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_gateway_idempotency_expires ON gateway_idempotency_cache(expires_at);

-- =============================================================================
-- Outbox compatibility: create 'outbox' as a view over 'outbox_items' so that
-- the outbox service (which queries 'outbox') works with the migration 001
-- table definition ('outbox_items').
-- =============================================================================
-- Add missing columns to outbox_items that the outbox service expects.
ALTER TABLE outbox_items ADD COLUMN IF NOT EXISTS aggregate_type text NOT NULL DEFAULT '';
ALTER TABLE outbox_items ADD COLUMN IF NOT EXISTS aggregate_id text NOT NULL DEFAULT '';
ALTER TABLE outbox_items ADD COLUMN IF NOT EXISTS event_type text NOT NULL DEFAULT '';
ALTER TABLE outbox_items ADD COLUMN IF NOT EXISTS target text NOT NULL DEFAULT '';
ALTER TABLE outbox_items ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'pending';
ALTER TABLE outbox_items ADD COLUMN IF NOT EXISTS attempts int NOT NULL DEFAULT 0;
ALTER TABLE outbox_items ADD COLUMN IF NOT EXISTS max_attempts int NOT NULL DEFAULT 5;
ALTER TABLE outbox_items ADD COLUMN IF NOT EXISTS next_retry_at timestamptz;
ALTER TABLE outbox_items ADD COLUMN IF NOT EXISTS fail_reason text NOT NULL DEFAULT '';
-- Rename payload column type if needed (outbox_items has jsonb payload, outbox service expects bytea-compatible)
-- The outbox service uses []byte which pgx maps to bytea, but the column is jsonb.
-- We keep jsonb since pgx can scan jsonb into []byte.

-- Rename 'sent_at' to 'dispatched_at' to match outbox service expectations.
ALTER TABLE outbox_items RENAME COLUMN sent_at TO dispatched_at;

-- Create the 'outbox' view that the outbox service queries.
CREATE OR REPLACE VIEW outbox AS SELECT * FROM outbox_items;

-- Allow INSERT/UPDATE/DELETE on the view via INSTEAD OF rules.
CREATE OR REPLACE RULE outbox_insert AS ON INSERT TO outbox
  DO INSTEAD INSERT INTO outbox_items VALUES (NEW.*);
CREATE OR REPLACE RULE outbox_update AS ON UPDATE TO outbox
  DO INSTEAD UPDATE outbox_items SET
    workspace_id = NEW.workspace_id,
    payload = NEW.payload,
    deliver_after = NEW.deliver_after,
    dispatched_at = NEW.dispatched_at,
    created_at = NEW.created_at,
    aggregate_type = NEW.aggregate_type,
    aggregate_id = NEW.aggregate_id,
    event_type = NEW.event_type,
    target = NEW.target,
    status = NEW.status,
    attempts = NEW.attempts,
    max_attempts = NEW.max_attempts,
    next_retry_at = NEW.next_retry_at,
    fail_reason = NEW.fail_reason
  WHERE outbox_items.id = OLD.id;
CREATE OR REPLACE RULE outbox_delete AS ON DELETE TO outbox
  DO INSTEAD DELETE FROM outbox_items WHERE outbox_items.id = OLD.id;

-- =============================================================================
-- OAuth token persistence: add refresh token columns to user_oauth_tokens
-- =============================================================================
ALTER TABLE user_oauth_tokens ADD COLUMN IF NOT EXISTS provider text NOT NULL DEFAULT '';
ALTER TABLE user_oauth_tokens ADD COLUMN IF NOT EXISTS refresh_ciphertext bytea;
ALTER TABLE user_oauth_tokens ADD COLUMN IF NOT EXISTS refresh_nonce bytea;
ALTER TABLE user_oauth_tokens ADD COLUMN IF NOT EXISTS expires_at timestamptz;
ALTER TABLE user_oauth_tokens ADD COLUMN IF NOT EXISTS last_refreshed_at timestamptz;
ALTER TABLE user_oauth_tokens ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

-- Unique constraint for one token per user per connector per workspace.
-- The existing table has connector_id as uuid referencing connectors(id).
-- We add a unique index to prevent duplicates.
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_oauth_tokens_unique
  ON user_oauth_tokens(workspace_id, user_id, connector_id);

-- =============================================================================
-- Extend ingress_turns with additional columns used by the gateway service
-- =============================================================================
ALTER TABLE ingress_turns ADD COLUMN IF NOT EXISTS parsed_interactive_reply text NOT NULL DEFAULT '';
ALTER TABLE ingress_turns ADD COLUMN IF NOT EXISTS parsed_discovery_answer text NOT NULL DEFAULT '';
ALTER TABLE ingress_turns ADD COLUMN IF NOT EXISTS transcript text NOT NULL DEFAULT '';
ALTER TABLE ingress_turns ADD COLUMN IF NOT EXISTS attachments jsonb NOT NULL DEFAULT '[]'::jsonb;

COMMIT;
