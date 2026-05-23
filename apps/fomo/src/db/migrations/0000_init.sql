CREATE TABLE "alert_state_transitions" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"alert_id" text NOT NULL,
	"user_id" text NOT NULL,
	"from_state" text NOT NULL,
	"to_state" text NOT NULL,
	"at" timestamp with time zone DEFAULT now() NOT NULL,
	"reason" text NOT NULL
);
--> statement-breakpoint
CREATE TABLE "audit_log" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"occurred_at" timestamp with time zone DEFAULT now() NOT NULL,
	"actor_user_id" text,
	"actor_ip" text,
	"actor_user_agent" text,
	"action" text NOT NULL,
	"target" text,
	"result" text NOT NULL,
	"detail" jsonb
);
--> statement-breakpoint
CREATE TABLE "consent" (
	"user_id" text NOT NULL,
	"tool_id" text NOT NULL,
	"granted_at" timestamp with time zone DEFAULT now() NOT NULL,
	"revoked_at" timestamp with time zone,
	CONSTRAINT "consent_user_id_tool_id_pk" PRIMARY KEY("user_id","tool_id")
);
--> statement-breakpoint
CREATE TABLE "cost_records" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"occurred_at" timestamp with time zone DEFAULT now() NOT NULL,
	"user_id" text NOT NULL,
	"capability" text NOT NULL,
	"model_name" text NOT NULL,
	"prompt_version" text NOT NULL,
	"latency_ms" integer NOT NULL,
	"input_tokens" integer NOT NULL,
	"output_tokens" integer NOT NULL,
	"estimated_cost_usd" double precision NOT NULL,
	"schema_valid" boolean NOT NULL
);
--> statement-breakpoint
CREATE TABLE "feedback_events" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"occurred_at" timestamp with time zone DEFAULT now() NOT NULL,
	"user_id" text NOT NULL,
	"alert_id" text,
	"sender_email" text,
	"kind" text NOT NULL,
	"detail" jsonb
);
--> statement-breakpoint
CREATE TABLE "memory_signals" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	"user_id" text NOT NULL,
	"kind" text NOT NULL,
	"scope_key" text DEFAULT '' NOT NULL,
	"detail" jsonb NOT NULL,
	"confidence" double precision NOT NULL,
	"source" text NOT NULL
);
--> statement-breakpoint
CREATE TABLE "oauth_tokens" (
	"user_id" text NOT NULL,
	"provider" text NOT NULL,
	"scopes" jsonb NOT NULL,
	"access_token_ciphertext" text NOT NULL,
	"refresh_token_ciphertext" text,
	"expires_at" timestamp with time zone,
	"obtained_at" timestamp with time zone DEFAULT now() NOT NULL,
	"last_refreshed_at" timestamp with time zone,
	"needs_reauth" boolean DEFAULT false NOT NULL,
	"key_version" integer DEFAULT 1 NOT NULL,
	CONSTRAINT "oauth_tokens_user_id_provider_pk" PRIMARY KEY("user_id","provider")
);
--> statement-breakpoint
CREATE TABLE "tool_invocations" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"occurred_at" timestamp with time zone DEFAULT now() NOT NULL,
	"user_id" text NOT NULL,
	"tool_id" text NOT NULL,
	"invocation_id" text NOT NULL,
	"policy_decision" text NOT NULL,
	"status" text NOT NULL,
	"latency_ms" integer,
	"error_code" text,
	"error_reason" text,
	"metadata" jsonb
);
--> statement-breakpoint
CREATE TABLE "users" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"email" text NOT NULL,
	"timezone" text,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE INDEX "alert_state_transitions_alert_idx" ON "alert_state_transitions" USING btree ("alert_id");--> statement-breakpoint
CREATE INDEX "alert_state_transitions_user_idx" ON "alert_state_transitions" USING btree ("user_id","at");--> statement-breakpoint
CREATE INDEX "audit_log_actor_user_id_idx" ON "audit_log" USING btree ("actor_user_id");--> statement-breakpoint
CREATE INDEX "audit_log_occurred_at_idx" ON "audit_log" USING btree ("occurred_at");--> statement-breakpoint
CREATE INDEX "cost_records_user_idx" ON "cost_records" USING btree ("user_id","occurred_at");--> statement-breakpoint
CREATE INDEX "cost_records_model_idx" ON "cost_records" USING btree ("user_id","model_name");--> statement-breakpoint
CREATE INDEX "feedback_events_user_id_idx" ON "feedback_events" USING btree ("user_id");--> statement-breakpoint
CREATE INDEX "feedback_events_kind_idx" ON "feedback_events" USING btree ("user_id","kind");--> statement-breakpoint
CREATE INDEX "feedback_events_sender_idx" ON "feedback_events" USING btree ("user_id","sender_email");--> statement-breakpoint
CREATE UNIQUE INDEX "memory_signals_identity_uq" ON "memory_signals" USING btree ("user_id","kind","scope_key");--> statement-breakpoint
CREATE UNIQUE INDEX "tool_invocations_invocation_id_uq" ON "tool_invocations" USING btree ("invocation_id");--> statement-breakpoint
CREATE INDEX "tool_invocations_user_idx" ON "tool_invocations" USING btree ("user_id","occurred_at");--> statement-breakpoint
CREATE INDEX "tool_invocations_tool_idx" ON "tool_invocations" USING btree ("user_id","tool_id");--> statement-breakpoint
CREATE INDEX "tool_invocations_status_idx" ON "tool_invocations" USING btree ("user_id","status");--> statement-breakpoint
CREATE UNIQUE INDEX "users_email_uq" ON "users" USING btree ("email");