"""add_invoice_pdf_path_to_transactions

Revision ID: 2f664670ccf8
Revises: 8b024b769796
Create Date: 2026-02-04 09:15:28.501520

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = '2f664670ccf8'
down_revision: Union[str, Sequence[str], None] = '8b024b769796'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """Upgrade schema."""
    # Add invoice_pdf_path column to transactions table
    op.add_column('transactions', sa.Column('invoice_pdf_path', sa.String(), nullable=True))


def downgrade() -> None:
    """Downgrade schema."""
    # Remove invoice_pdf_path column from transactions table
    op.drop_column('transactions', 'invoice_pdf_path')
