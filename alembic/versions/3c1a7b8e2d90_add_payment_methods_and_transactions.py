"""add_payment_methods_and_transactions

Revision ID: 3c1a7b8e2d90
Revises: 8b024b769796
Create Date: 2026-02-06 16:45:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = '3c1a7b8e2d90'
down_revision: Union[str, Sequence[str], None] = '8b024b769796'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """Upgrade schema."""
    # payment_methods table
    op.create_table(
        'payment_methods',
        sa.Column('id', sa.Integer(), autoincrement=True, nullable=False),
        sa.Column('user_id', sa.String(), nullable=False),
        sa.Column('stripe_payment_method_id', sa.String(), nullable=False),
        sa.Column('type', sa.String(), nullable=False),
        sa.Column('brand', sa.String(), nullable=True),
        sa.Column('last4', sa.String(), nullable=True),
        sa.Column('exp_month', sa.Integer(), nullable=True),
        sa.Column('exp_year', sa.Integer(), nullable=True),
        sa.Column('is_default', sa.Boolean(), nullable=False),
        sa.Column('created_at', sa.DateTime(), nullable=False),
        sa.Column('updated_at', sa.DateTime(), nullable=False),
        sa.ForeignKeyConstraint(['user_id'], ['users.id'], ),
        sa.PrimaryKeyConstraint('id')
    )
    op.create_index(op.f('ix_payment_methods_created_at'), 'payment_methods', ['created_at'], unique=False)
    op.create_index(op.f('ix_payment_methods_stripe_payment_method_id'), 'payment_methods', ['stripe_payment_method_id'], unique=True)
    op.create_index(op.f('ix_payment_methods_user_id'), 'payment_methods', ['user_id'], unique=False)

    # transactions table (without invoice_pdf_path; added in next migration)
    op.create_table(
        'transactions',
        sa.Column('id', sa.Integer(), autoincrement=True, nullable=False),
        sa.Column('user_id', sa.String(), nullable=False),
        sa.Column('proposal_id', sa.Integer(), nullable=True),
        sa.Column('amount', sa.Float(), nullable=False),
        sa.Column('currency', sa.String(), nullable=False),
        sa.Column('status', sa.String(), nullable=False),
        sa.Column('stripe_payment_intent_id', sa.String(), nullable=True),
        sa.Column('stripe_charge_id', sa.String(), nullable=True),
        sa.Column('payment_method_id', sa.Integer(), nullable=True),
        sa.Column('transaction_type', sa.String(), nullable=False),
        sa.Column('description', sa.Text(), nullable=True),
        sa.Column('metadata_json', sa.Text(), nullable=True),
        sa.Column('refund_amount', sa.Float(), nullable=True),
        sa.Column('refund_reason', sa.String(), nullable=True),
        sa.Column('refunded_at', sa.DateTime(), nullable=True),
        sa.Column('created_at', sa.DateTime(), nullable=False),
        sa.Column('updated_at', sa.DateTime(), nullable=False),
        sa.ForeignKeyConstraint(['payment_method_id'], ['payment_methods.id'], ),
        sa.ForeignKeyConstraint(['proposal_id'], ['proposals.id'], ),
        sa.ForeignKeyConstraint(['user_id'], ['users.id'], ),
        sa.PrimaryKeyConstraint('id')
    )
    op.create_index(op.f('ix_transactions_created_at'), 'transactions', ['created_at'], unique=False)
    op.create_index(op.f('ix_transactions_proposal_id'), 'transactions', ['proposal_id'], unique=False)
    op.create_index(op.f('ix_transactions_status'), 'transactions', ['status'], unique=False)
    op.create_index(op.f('ix_transactions_stripe_payment_intent_id'), 'transactions', ['stripe_payment_intent_id'], unique=True)
    op.create_index(op.f('ix_transactions_transaction_type'), 'transactions', ['transaction_type'], unique=False)
    op.create_index(op.f('ix_transactions_user_id'), 'transactions', ['user_id'], unique=False)


def downgrade() -> None:
    """Downgrade schema."""
    op.drop_index(op.f('ix_transactions_user_id'), table_name='transactions')
    op.drop_index(op.f('ix_transactions_transaction_type'), table_name='transactions')
    op.drop_index(op.f('ix_transactions_stripe_payment_intent_id'), table_name='transactions')
    op.drop_index(op.f('ix_transactions_status'), table_name='transactions')
    op.drop_index(op.f('ix_transactions_proposal_id'), table_name='transactions')
    op.drop_index(op.f('ix_transactions_created_at'), table_name='transactions')
    op.drop_table('transactions')
    op.drop_index(op.f('ix_payment_methods_user_id'), table_name='payment_methods')
    op.drop_index(op.f('ix_payment_methods_stripe_payment_method_id'), table_name='payment_methods')
    op.drop_index(op.f('ix_payment_methods_created_at'), table_name='payment_methods')
    op.drop_table('payment_methods')
