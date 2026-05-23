CREATE TABLE "gmail_cursors" (
	"user_id" text PRIMARY KEY NOT NULL,
	"history_id" text NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL
);
