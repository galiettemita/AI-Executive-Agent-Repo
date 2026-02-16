"""add mcp_result content provenance enum value

Revision ID: q1w2e3r4t5y6
Revises: w7x8y9z0a1b2
Create Date: 2026-02-16 10:05:00.000000
"""

from __future__ import annotations

from alembic import op


# revision identifiers, used by Alembic.
revision = "q1w2e3r4t5y6"
down_revision = "w7x8y9z0a1b2"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.execute(
        """
        DO $$
        BEGIN
            IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'content_provenance')
               AND NOT EXISTS (
                   SELECT 1
                   FROM pg_enum e
                   JOIN pg_type t ON t.oid = e.enumtypid
                   WHERE t.typname = 'content_provenance' AND e.enumlabel = 'mcp_result'
               )
            THEN
                ALTER TYPE content_provenance ADD VALUE 'mcp_result';
            END IF;
        END $$;
        """
    )
    op.execute(
        """
        DO $$
        BEGIN
            IF to_regclass('messages') IS NOT NULL THEN
                UPDATE messages
                SET content_provenance = 'mcp_result'::content_provenance
                WHERE content_provenance::text = 'mcp_response';
            END IF;
        END $$;
        """
    )
    op.execute(
        """
        DO $$
        BEGIN
            IF to_regclass('tool_executions') IS NOT NULL THEN
                UPDATE tool_executions
                SET input_provenance = 'mcp_result'::content_provenance
                WHERE input_provenance::text = 'mcp_response';
            END IF;
        END $$;
        """
    )


def downgrade() -> None:
    # PostgreSQL enums do not support dropping individual labels safely.
    pass

