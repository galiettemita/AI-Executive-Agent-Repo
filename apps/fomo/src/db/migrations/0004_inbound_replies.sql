CREATE TABLE "inbound_replies" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"provider_message_id" text NOT NULL,
	"user_id" text NOT NULL,
	"received_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE UNIQUE INDEX "inbound_replies_provider_message_id_uq" ON "inbound_replies" USING btree ("provider_message_id");--> statement-breakpoint
CREATE INDEX "inbound_replies_user_received_idx" ON "inbound_replies" USING btree ("user_id","received_at");
