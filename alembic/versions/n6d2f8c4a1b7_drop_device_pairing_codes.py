"""drop device pairing codes

Revision ID: n6d2f8c4a1b7
Revises: m5a1c7d9b2e4
Create Date: 2026-02-08
"""

from alembic import op

# revision identifiers, used by Alembic.
revision = "n6d2f8c4a1b7"
down_revision = "m5a1c7d9b2e4"
branch_labels = None
depends_on = None


def upgrade():
    op.execute("DROP TABLE IF EXISTS device_pairing_codes")


def downgrade():
    # No-op; table intentionally removed.
    pass
