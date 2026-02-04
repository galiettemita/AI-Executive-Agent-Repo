"""add_webhook_tables

Revision ID: a78319edcc71
Revises: 2f664670ccf8
Create Date: 2026-02-04 09:21:19.010936

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = 'a78319edcc71'
down_revision: Union[str, Sequence[str], None] = '2f664670ccf8'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """Upgrade schema."""
    # Create webhook_endpoints table
    op.create_table(
        'webhook_endpoints',
        sa.Column('id', sa.Integer(), autoincrement=True, nullable=False),
        sa.Column('user_id', sa.String(), nullable=False),
        sa.Column('url', sa.String(), nullable=False),
        sa.Column('secret', sa.String(), nullable=True),
        sa.Column('is_active', sa.Boolean(), nullable=False),
        sa.Column('event_types', sa.Text(), nullable=True),
        sa.Column('description', sa.String(), nullable=True),
        sa.Column('total_deliveries', sa.Integer(), nullable=False),
        sa.Column('failed_deliveries', sa.Integer(), nullable=False),
        sa.Column('last_delivery_at', sa.DateTime(), nullable=True),
        sa.Column('last_failure_at', sa.DateTime(), nullable=True),
        sa.Column('created_at', sa.DateTime(), nullable=False),
        sa.Column('updated_at', sa.DateTime(), nullable=False),
        sa.ForeignKeyConstraint(['user_id'], ['users.id'], ),
        sa.PrimaryKeyConstraint('id')
    )
    op.create_index(op.f('ix_webhook_endpoints_created_at'), 'webhook_endpoints', ['created_at'], unique=False)
    op.create_index(op.f('ix_webhook_endpoints_is_active'), 'webhook_endpoints', ['is_active'], unique=False)
    op.create_index(op.f('ix_webhook_endpoints_user_id'), 'webhook_endpoints', ['user_id'], unique=False)

    # Create webhook_deliveries table
    op.create_table(
        'webhook_deliveries',
        sa.Column('id', sa.Integer(), autoincrement=True, nullable=False),
        sa.Column('webhook_endpoint_id', sa.Integer(), nullable=False),
        sa.Column('event_type', sa.String(), nullable=False),
        sa.Column('event_id', sa.String(), nullable=False),
        sa.Column('proposal_id', sa.Integer(), nullable=True),
        sa.Column('transaction_id', sa.Integer(), nullable=True),
        sa.Column('payload_json', sa.Text(), nullable=False),
        sa.Column('status', sa.String(), nullable=False),
        sa.Column('response_status_code', sa.Integer(), nullable=True),
        sa.Column('response_body', sa.Text(), nullable=True),
        sa.Column('error_message', sa.Text(), nullable=True),
        sa.Column('retry_count', sa.Integer(), nullable=False),
        sa.Column('next_retry_at', sa.DateTime(), nullable=True),
        sa.Column('created_at', sa.DateTime(), nullable=False),
        sa.Column('delivered_at', sa.DateTime(), nullable=True),
        sa.ForeignKeyConstraint(['proposal_id'], ['proposals.id'], ),
        sa.ForeignKeyConstraint(['transaction_id'], ['transactions.id'], ),
        sa.ForeignKeyConstraint(['webhook_endpoint_id'], ['webhook_endpoints.id'], ),
        sa.PrimaryKeyConstraint('id')
    )
    op.create_index(op.f('ix_webhook_deliveries_created_at'), 'webhook_deliveries', ['created_at'], unique=False)
    op.create_index(op.f('ix_webhook_deliveries_event_id'), 'webhook_deliveries', ['event_id'], unique=False)
    op.create_index(op.f('ix_webhook_deliveries_event_type'), 'webhook_deliveries', ['event_type'], unique=False)
    op.create_index(op.f('ix_webhook_deliveries_proposal_id'), 'webhook_deliveries', ['proposal_id'], unique=False)
    op.create_index(op.f('ix_webhook_deliveries_status'), 'webhook_deliveries', ['status'], unique=False)
    op.create_index(op.f('ix_webhook_deliveries_transaction_id'), 'webhook_deliveries', ['transaction_id'], unique=False)
    op.create_index(op.f('ix_webhook_deliveries_webhook_endpoint_id'), 'webhook_deliveries', ['webhook_endpoint_id'], unique=False)


def downgrade() -> None:
    """Downgrade schema."""
    op.drop_index(op.f('ix_webhook_deliveries_webhook_endpoint_id'), table_name='webhook_deliveries')
    op.drop_index(op.f('ix_webhook_deliveries_transaction_id'), table_name='webhook_deliveries')
    op.drop_index(op.f('ix_webhook_deliveries_status'), table_name='webhook_deliveries')
    op.drop_index(op.f('ix_webhook_deliveries_proposal_id'), table_name='webhook_deliveries')
    op.drop_index(op.f('ix_webhook_deliveries_event_type'), table_name='webhook_deliveries')
    op.drop_index(op.f('ix_webhook_deliveries_event_id'), table_name='webhook_deliveries')
    op.drop_index(op.f('ix_webhook_deliveries_created_at'), table_name='webhook_deliveries')
    op.drop_table('webhook_deliveries')
    op.drop_index(op.f('ix_webhook_endpoints_user_id'), table_name='webhook_endpoints')
    op.drop_index(op.f('ix_webhook_endpoints_is_active'), table_name='webhook_endpoints')
    op.drop_index(op.f('ix_webhook_endpoints_created_at'), table_name='webhook_endpoints')
    op.drop_table('webhook_endpoints')
