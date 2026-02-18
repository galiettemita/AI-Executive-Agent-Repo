"""add mcp server registry tables

Revision ID: y8z9a0b1c2d3
Revises: q1w2e3r4t5y6
Create Date: 2026-02-17 20:25:00.000000
"""

from __future__ import annotations

from alembic import op


# revision identifiers, used by Alembic.
revision = "y8z9a0b1c2d3"
down_revision = "q1w2e3r4t5y6"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.execute(
        """
        CREATE TABLE IF NOT EXISTS mcp_servers (
          id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
          server_id TEXT UNIQUE NOT NULL,
          display_name TEXT NOT NULL,
          description TEXT,
          transport JSONB NOT NULL,
          tags TEXT[] DEFAULT '{}',
          expected_tools TEXT[] DEFAULT '{}',
          expected_resources TEXT[] DEFAULT '{}',
          expected_prompts TEXT[] DEFAULT '{}',
          tools JSONB DEFAULT '[]'::jsonb,
          resources JSONB DEFAULT '[]'::jsonb,
          prompts JSONB DEFAULT '[]'::jsonb,
          state TEXT DEFAULT 'registered',
          rate_limit_per_min INT DEFAULT 60,
          daily_budget_cents INT DEFAULT 500,
          version TEXT DEFAULT '1.0.0',
          health_status TEXT DEFAULT 'unknown',
          last_health_check_at TIMESTAMPTZ,
          consecutive_failures INT DEFAULT 0,
          total_calls INT DEFAULT 0,
          total_errors INT DEFAULT 0,
          total_cost_cents FLOAT DEFAULT 0,
          created_at TIMESTAMPTZ DEFAULT now(),
          updated_at TIMESTAMPTZ DEFAULT now()
        )
        """
    )

    op.execute(
        """
        CREATE TABLE IF NOT EXISTS mcp_user_servers (
          id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
          user_id UUID NOT NULL REFERENCES accounts(id),
          server_id TEXT NOT NULL REFERENCES mcp_servers(server_id),
          is_enabled BOOLEAN DEFAULT true,
          custom_config JSONB DEFAULT '{}'::jsonb,
          daily_budget_override FLOAT,
          created_at TIMESTAMPTZ DEFAULT now(),
          UNIQUE(user_id, server_id)
        )
        """
    )

    op.execute("CREATE INDEX IF NOT EXISTS idx_mcp_servers_state ON mcp_servers(state)")
    op.execute("CREATE INDEX IF NOT EXISTS idx_mcp_user_servers_user ON mcp_user_servers(user_id)")

    op.execute("ALTER TABLE mcp_servers ENABLE ROW LEVEL SECURITY")
    op.execute("DROP POLICY IF EXISTS mcp_servers_user_access ON mcp_servers")
    op.execute(
        """
        CREATE POLICY mcp_servers_user_access ON mcp_servers
        USING (
          EXISTS (
            SELECT 1
            FROM mcp_user_servers mus
            WHERE mus.server_id = mcp_servers.server_id
              AND mus.user_id::text = current_setting('app.user_id', true)
          )
        )
        """
    )

    op.execute("ALTER TABLE mcp_user_servers ENABLE ROW LEVEL SECURITY")
    op.execute("DROP POLICY IF EXISTS mcp_user_servers_user_access ON mcp_user_servers")
    op.execute(
        """
        CREATE POLICY mcp_user_servers_user_access ON mcp_user_servers
        USING (user_id::text = current_setting('app.user_id', true))
        WITH CHECK (user_id::text = current_setting('app.user_id', true))
        """
    )


def downgrade() -> None:
    op.execute("DROP TABLE IF EXISTS mcp_user_servers")
    op.execute("DROP TABLE IF EXISTS mcp_servers")
