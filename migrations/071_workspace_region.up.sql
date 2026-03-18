-- Add workspace region for multi-region routing (P3-09).
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS workspace_region TEXT DEFAULT 'us-east-1'
    CHECK (workspace_region IN ('us-east-1', 'eu-west-1'));
