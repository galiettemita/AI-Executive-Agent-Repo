CREATE TABLE "alerts" (
	"alert_id" text PRIMARY KEY NOT NULL,
	"user_id" text NOT NULL,
	"message_id" text NOT NULL,
	"rank_result_id" bigint NOT NULL,
	"label" text NOT NULL,
	"score" double precision NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE UNIQUE INDEX "alerts_rank_result_uq" ON "alerts" USING btree ("rank_result_id");--> statement-breakpoint
CREATE INDEX "alerts_user_created_idx" ON "alerts" USING btree ("user_id","created_at");--> statement-breakpoint
CREATE INDEX "alerts_user_message_idx" ON "alerts" USING btree ("user_id","message_id");