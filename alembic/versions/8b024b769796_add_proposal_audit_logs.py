"""add_proposal_audit_logs

Revision ID: 8b024b769796
Revises: 2f9c0c1b4a7e
Create Date: 2026-02-03 16:04:57.565722

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = '8b024b769796'
down_revision: Union[str, Sequence[str], None] = '2f9c0c1b4a7e'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """Upgrade schema."""
    op.create_table(
        'proposal_audit_logs',
        sa.Column('id', sa.Integer(), autoincrement=True, nullable=False),
        sa.Column('proposal_id', sa.Integer(), nullable=False),
        sa.Column('user_id', sa.String(), nullable=False),
        sa.Column('action', sa.String(), nullable=False),
        sa.Column('old_status', sa.String(), nullable=True),
        sa.Column('new_status', sa.String(), nullable=True),
        sa.Column('changes_json', sa.Text(), nullable=True),
        sa.Column('metadata_json', sa.Text(), nullable=True),
        sa.Column('created_at', sa.DateTime(), nullable=False),
        sa.ForeignKeyConstraint(['proposal_id'], ['proposals.id'], ),
        sa.ForeignKeyConstraint(['user_id'], ['users.id'], ),
        sa.PrimaryKeyConstraint('id')
    )
    op.create_index(op.f('ix_proposal_audit_logs_proposal_id'), 'proposal_audit_logs', ['proposal_id'], unique=False)
    op.create_index(op.f('ix_proposal_audit_logs_user_id'), 'proposal_audit_logs', ['user_id'], unique=False)
    op.create_index(op.f('ix_proposal_audit_logs_action'), 'proposal_audit_logs', ['action'], unique=False)
    op.create_index(op.f('ix_proposal_audit_logs_created_at'), 'proposal_audit_logs', ['created_at'], unique=False)


def downgrade() -> None:
    """Downgrade schema."""
    op.drop_index(op.f('ix_proposal_audit_logs_created_at'), table_name='proposal_audit_logs')
    op.drop_index(op.f('ix_proposal_audit_logs_action'), table_name='proposal_audit_logs')
    op.drop_index(op.f('ix_proposal_audit_logs_user_id'), table_name='proposal_audit_logs')
    op.drop_index(op.f('ix_proposal_audit_logs_proposal_id'), table_name='proposal_audit_logs')
    op.drop_table('proposal_audit_logs')
