-- BREVIO V9.3 addendum closure migration
-- Forward-only additive migration.

CREATE TABLE IF NOT EXISTS whatsapp_message_templates (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  name text NOT NULL,
  language_code text NOT NULL,
  components_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, name, language_code)
);

ALTER TABLE whatsapp_message_templates ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
      FROM pg_policies
     WHERE schemaname = 'public'
       AND tablename = 'whatsapp_message_templates'
       AND policyname = 'whatsapp_message_templates_workspace_isolation'
  ) THEN
    CREATE POLICY whatsapp_message_templates_workspace_isolation
      ON whatsapp_message_templates
      USING (workspace_id = current_setting('app.workspace_id')::uuid);
  END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_whatsapp_message_templates_workspace_id
  ON whatsapp_message_templates(workspace_id);
CREATE INDEX IF NOT EXISTS idx_whatsapp_message_templates_name_lang
  ON whatsapp_message_templates(workspace_id, name, language_code);
