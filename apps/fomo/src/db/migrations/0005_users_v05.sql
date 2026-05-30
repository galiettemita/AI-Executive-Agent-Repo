-- Phase v0.5.1 — Multi-tenant Substrate migration.
--
-- Adds the columns and tables the friend-beta substrate needs:
--   * users.phone_e164_encrypted  — KEK-wrapped jsonb envelope shape
--                                   matching the existing token-crypto
--                                   layer. Plaintext NEVER on disk.
--   * users.phone_e164_hash       — deterministic HMAC over normalized
--                                   E.164. The lookup + uniqueness
--                                   key. Authenticated via the new
--                                   BREVIO_PHONE_HASH_KEY env var
--                                   (separate from BREVIO_TOKEN_KEK
--                                   per separation-of-duties).
--   * users.is_founder            — true for the founder row; false
--                                   for friend rows. Single-source-
--                                   of-truth for "is alert.user_id
--                                   the founder?" lookups (drives
--                                   the friend-safe Slack card branch
--                                   in the renderer).
--   * UNIQUE INDEX on phone_e164_hash — DB-level duplicate-phone
--                                   rejection. Application surfaces
--                                   the violation as DuplicatePhoneError.
--   * invite_tokens table         — one-time, expiring, bound-to-
--                                   intended-phone invites. token_hash
--                                   only (plaintext NEVER persisted);
--                                   consumed_at NULL until the OAuth
--                                   callback succeeds; consumption
--                                   is atomic (UPDATE WHERE consumed_at
--                                   IS NULL AND expires_at > now()).

ALTER TABLE "users" ADD COLUMN "phone_e164_encrypted" jsonb;
--> statement-breakpoint
ALTER TABLE "users" ADD COLUMN "phone_e164_hash" text;
--> statement-breakpoint
ALTER TABLE "users" ADD COLUMN "is_founder" boolean NOT NULL DEFAULT false;
--> statement-breakpoint
CREATE UNIQUE INDEX "users_phone_e164_hash_uq" ON "users" USING btree ("phone_e164_hash");
--> statement-breakpoint
CREATE TABLE "invite_tokens" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"token_hash" text NOT NULL,
	"intended_phone_hash" text NOT NULL,
	"issued_by_user_id" text NOT NULL,
	"issued_at" timestamp with time zone DEFAULT now() NOT NULL,
	"expires_at" timestamp with time zone NOT NULL,
	"consumed_at" timestamp with time zone,
	"consumed_user_id" text
);
--> statement-breakpoint
CREATE UNIQUE INDEX "invite_tokens_token_hash_uq" ON "invite_tokens" USING btree ("token_hash");
--> statement-breakpoint
CREATE INDEX "invite_tokens_intended_phone_idx" ON "invite_tokens" USING btree ("intended_phone_hash","expires_at");
--> statement-breakpoint
CREATE INDEX "invite_tokens_unconsumed_idx" ON "invite_tokens" USING btree ("consumed_at","expires_at");
