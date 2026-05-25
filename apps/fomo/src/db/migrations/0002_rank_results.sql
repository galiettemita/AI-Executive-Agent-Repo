CREATE TABLE "rank_results" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"user_id" text NOT NULL,
	"message_id" text NOT NULL,
	"invocation_id" text NOT NULL,
	"model_name" text NOT NULL,
	"prompt_version" text NOT NULL,
	"label" text NOT NULL,
	"score" double precision NOT NULL,
	"reason" text NOT NULL,
	"latency_ms" integer NOT NULL,
	"input_tokens" integer NOT NULL,
	"output_tokens" integer NOT NULL,
	"estimated_cost_usd" double precision NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE UNIQUE INDEX "rank_results_user_message_uq" ON "rank_results" USING btree ("user_id","message_id");--> statement-breakpoint
CREATE INDEX "rank_results_user_created_idx" ON "rank_results" USING btree ("user_id","created_at");--> statement-breakpoint
CREATE INDEX "rank_results_label_idx" ON "rank_results" USING btree ("user_id","label");