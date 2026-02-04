"""add_traveler_profiles_table

Revision ID: 7acebebdb2cb
Revises: a78319edcc71
Create Date: 2026-02-04 09:34:56.595200

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = '7acebebdb2cb'
down_revision: Union[str, Sequence[str], None] = 'a78319edcc71'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """Upgrade schema."""
    # Create traveler_profiles table
    op.create_table(
        'traveler_profiles',
        sa.Column('id', sa.Integer(), autoincrement=True, nullable=False),
        sa.Column('user_id', sa.String(), nullable=False),
        sa.Column('first_name', sa.String(), nullable=False),
        sa.Column('last_name', sa.String(), nullable=False),
        sa.Column('middle_name', sa.String(), nullable=True),
        sa.Column('date_of_birth', sa.String(), nullable=False),
        sa.Column('gender', sa.String(), nullable=False),
        sa.Column('email', sa.String(), nullable=True),
        sa.Column('phone', sa.String(), nullable=True),
        sa.Column('passport_number', sa.String(), nullable=True),
        sa.Column('passport_country', sa.String(), nullable=True),
        sa.Column('passport_expiry', sa.String(), nullable=True),
        sa.Column('nationality', sa.String(), nullable=True),
        sa.Column('known_traveler_number', sa.String(), nullable=True),
        sa.Column('redress_number', sa.String(), nullable=True),
        sa.Column('seat_preference', sa.String(), nullable=True),
        sa.Column('meal_preference', sa.String(), nullable=True),
        sa.Column('loyalty_programs', sa.Text(), nullable=True),
        sa.Column('is_default', sa.Boolean(), nullable=False),
        sa.Column('created_at', sa.DateTime(), nullable=False),
        sa.Column('updated_at', sa.DateTime(), nullable=False),
        sa.ForeignKeyConstraint(['user_id'], ['users.id'], ),
        sa.PrimaryKeyConstraint('id')
    )
    op.create_index(op.f('ix_traveler_profiles_created_at'), 'traveler_profiles', ['created_at'], unique=False)
    op.create_index(op.f('ix_traveler_profiles_user_id'), 'traveler_profiles', ['user_id'], unique=False)


def downgrade() -> None:
    """Downgrade schema."""
    op.drop_index(op.f('ix_traveler_profiles_user_id'), table_name='traveler_profiles')
    op.drop_index(op.f('ix_traveler_profiles_created_at'), table_name='traveler_profiles')
    op.drop_table('traveler_profiles')
