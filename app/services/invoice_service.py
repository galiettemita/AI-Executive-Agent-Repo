# app/services/invoice_service.py

from __future__ import annotations

import os
from datetime import datetime
from typing import Dict, Optional
from sqlalchemy.orm import Session

from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.units import inch
from reportlab.platypus import SimpleDocTemplate, Table, TableStyle, Paragraph, Spacer
from reportlab.lib import colors
from reportlab.lib.enums import TA_RIGHT, TA_CENTER

from app.db.models import Transaction, User, Proposal


class InvoiceService:
    """Service for generating PDF invoices"""

    @staticmethod
    def generate_invoice_pdf(db: Session, transaction_id: int) -> str:
        """
        Generate a PDF invoice for a transaction.

        Args:
            db: Database session
            transaction_id: ID of the transaction

        Returns:
            str: Path to the generated PDF file

        Raises:
            ValueError: If transaction not found or invoice cannot be generated
        """
        # Load transaction with related data
        transaction = db.query(Transaction).filter(Transaction.id == transaction_id).first()
        if not transaction:
            raise ValueError(f"Transaction {transaction_id} not found")

        # Load user
        user = db.query(User).filter(User.id == transaction.user_id).first()
        if not user:
            raise ValueError(f"User {transaction.user_id} not found")

        # Load proposal if exists
        proposal = None
        if transaction.proposal_id:
            proposal = db.query(Proposal).filter(Proposal.id == transaction.proposal_id).first()

        # Ensure invoices directory exists
        invoices_dir = os.path.join(os.getcwd(), "invoices")
        os.makedirs(invoices_dir, exist_ok=True)

        # Generate filename
        timestamp = datetime.utcnow().strftime("%Y%m%d_%H%M%S")
        filename = f"invoice_{transaction.id}_{timestamp}.pdf"
        filepath = os.path.join(invoices_dir, filename)

        # Create PDF
        doc = SimpleDocTemplate(filepath, pagesize=letter)
        elements = []
        styles = getSampleStyleSheet()

        # Custom styles
        title_style = ParagraphStyle(
            'CustomTitle',
            parent=styles['Heading1'],
            fontSize=24,
            textColor=colors.HexColor('#1a1a1a'),
            spaceAfter=30,
            alignment=TA_CENTER,
        )

        header_style = ParagraphStyle(
            'CustomHeader',
            parent=styles['Heading2'],
            fontSize=14,
            textColor=colors.HexColor('#333333'),
            spaceAfter=12,
        )

        right_align_style = ParagraphStyle(
            'RightAlign',
            parent=styles['Normal'],
            alignment=TA_RIGHT,
        )

        # Title
        elements.append(Paragraph("INVOICE", title_style))
        elements.append(Spacer(1, 0.3*inch))

        # Company Info (Left) and Invoice Info (Right)
        company_info = [
            ["<b>AI Shopping Assistant</b>", ""],
            ["Your AI-Powered Shopping Companion", ""],
            ["support@aishoppingassistant.com", ""],
        ]

        invoice_info_data = [
            ["", f"<b>Invoice #:</b> {transaction.id}"],
            ["", f"<b>Date:</b> {transaction.created_at.strftime('%B %d, %Y')}"],
            ["", f"<b>Status:</b> {transaction.status.upper()}"],
        ]

        # Combine both for layout
        header_data = []
        for i in range(max(len(company_info), len(invoice_info_data))):
            left = company_info[i] if i < len(company_info) else ["", ""]
            right = invoice_info_data[i] if i < len(invoice_info_data) else ["", ""]
            header_data.append([left[0], "", right[1]])

        header_table = Table(header_data, colWidths=[3*inch, 1*inch, 2.5*inch])
        header_table.setStyle(TableStyle([
            ('ALIGN', (0, 0), (0, -1), 'LEFT'),
            ('ALIGN', (2, 0), (2, -1), 'RIGHT'),
            ('FONTNAME', (0, 0), (0, 0), 'Helvetica-Bold'),
            ('FONTSIZE', (0, 0), (0, 0), 12),
            ('TEXTCOLOR', (0, 0), (-1, -1), colors.HexColor('#333333')),
        ]))
        elements.append(header_table)
        elements.append(Spacer(1, 0.4*inch))

        # Bill To Section
        elements.append(Paragraph("<b>Bill To:</b>", header_style))
        bill_to_data = [
            [f"User ID: {user.id}"],
        ]
        if hasattr(user, 'phone_number') and user.phone_number:
            bill_to_data.append([f"Phone: {user.phone_number}"])

        bill_to_table = Table(bill_to_data, colWidths=[6.5*inch])
        bill_to_table.setStyle(TableStyle([
            ('FONTNAME', (0, 0), (-1, -1), 'Helvetica'),
            ('FONTSIZE', (0, 0), (-1, -1), 10),
            ('TEXTCOLOR', (0, 0), (-1, -1), colors.HexColor('#555555')),
        ]))
        elements.append(bill_to_table)
        elements.append(Spacer(1, 0.3*inch))

        # Transaction Details Section
        elements.append(Paragraph("<b>Transaction Details:</b>", header_style))

        # Build transaction items
        transaction_items = [
            ['Description', 'Type', 'Amount'],
        ]

        description = transaction.description or f"Transaction #{transaction.id}"
        if proposal:
            import json
            try:
                payload = json.loads(proposal.payload_json)
                if 'description' in payload:
                    description = payload['description']
                elif 'items' in payload and isinstance(payload['items'], list):
                    description = ', '.join([item.get('name', 'Item') for item in payload['items'][:3]])
                    if len(payload['items']) > 3:
                        description += f" (+{len(payload['items']) - 3} more)"
            except:
                pass

        transaction_items.append([
            description,
            transaction.transaction_type.replace('_', ' ').title(),
            f"${transaction.amount:.2f}",
        ])

        # Add refund row if refunded
        if transaction.refund_amount and transaction.refund_amount > 0:
            transaction_items.append([
                f"Refund: {transaction.refund_reason or 'N/A'}",
                "Refund",
                f"-${transaction.refund_amount:.2f}",
            ])

        # Add total row
        final_amount = transaction.amount
        if transaction.refund_amount:
            final_amount -= transaction.refund_amount

        transaction_items.append(['', '<b>Total</b>', f"<b>${final_amount:.2f}</b>"])

        items_table = Table(transaction_items, colWidths=[3.5*inch, 2*inch, 1*inch])
        items_table.setStyle(TableStyle([
            # Header row
            ('BACKGROUND', (0, 0), (-1, 0), colors.HexColor('#f0f0f0')),
            ('TEXTCOLOR', (0, 0), (-1, 0), colors.HexColor('#333333')),
            ('ALIGN', (0, 0), (-1, 0), 'LEFT'),
            ('FONTNAME', (0, 0), (-1, 0), 'Helvetica-Bold'),
            ('FONTSIZE', (0, 0), (-1, 0), 10),
            ('BOTTOMPADDING', (0, 0), (-1, 0), 12),

            # Data rows
            ('TEXTCOLOR', (0, 1), (-1, -2), colors.HexColor('#555555')),
            ('ALIGN', (0, 1), (-1, -1), 'LEFT'),
            ('ALIGN', (2, 1), (2, -1), 'RIGHT'),
            ('FONTNAME', (0, 1), (-1, -2), 'Helvetica'),
            ('FONTSIZE', (0, 1), (-1, -2), 10),
            ('TOPPADDING', (0, 1), (-1, -2), 8),
            ('BOTTOMPADDING', (0, 1), (-1, -2), 8),

            # Total row
            ('BACKGROUND', (0, -1), (-1, -1), colors.HexColor('#f0f0f0')),
            ('TEXTCOLOR', (0, -1), (-1, -1), colors.HexColor('#1a1a1a')),
            ('FONTNAME', (0, -1), (-1, -1), 'Helvetica-Bold'),
            ('FONTSIZE', (0, -1), (-1, -1), 12),
            ('TOPPADDING', (0, -1), (-1, -1), 12),
            ('BOTTOMPADDING', (0, -1), (-1, -1), 12),
            ('ALIGN', (2, -1), (2, -1), 'RIGHT'),

            # Grid
            ('GRID', (0, 0), (-1, -1), 0.5, colors.HexColor('#cccccc')),
        ]))
        elements.append(items_table)
        elements.append(Spacer(1, 0.4*inch))

        # Payment Information
        elements.append(Paragraph("<b>Payment Information:</b>", header_style))
        payment_info_data = [
            [f"Payment Method: Card"],
            [f"Currency: {transaction.currency}"],
            [f"Status: {transaction.status.upper()}"],
        ]

        if transaction.stripe_payment_intent_id:
            payment_info_data.append([f"Payment Intent ID: {transaction.stripe_payment_intent_id}"])

        if transaction.stripe_charge_id:
            payment_info_data.append([f"Charge ID: {transaction.stripe_charge_id}"])

        payment_info_table = Table(payment_info_data, colWidths=[6.5*inch])
        payment_info_table.setStyle(TableStyle([
            ('FONTNAME', (0, 0), (-1, -1), 'Helvetica'),
            ('FONTSIZE', (0, 0), (-1, -1), 9),
            ('TEXTCOLOR', (0, 0), (-1, -1), colors.HexColor('#555555')),
        ]))
        elements.append(payment_info_table)
        elements.append(Spacer(1, 0.5*inch))

        # Footer
        footer_text = """
        <para align=center>
        <font size=8 color="#888888">
        Thank you for using AI Shopping Assistant!<br/>
        For questions about this invoice, please contact support@aishoppingassistant.com
        </font>
        </para>
        """
        elements.append(Paragraph(footer_text, styles['Normal']))

        # Build PDF
        doc.build(elements)

        return filepath

    @staticmethod
    def get_invoice_path(db: Session, transaction_id: int) -> Optional[str]:
        """
        Get the path to an existing invoice PDF.

        Args:
            db: Database session
            transaction_id: ID of the transaction

        Returns:
            str: Path to the PDF file, or None if not generated yet
        """
        transaction = db.query(Transaction).filter(Transaction.id == transaction_id).first()
        if not transaction:
            return None

        return transaction.invoice_pdf_path

    @staticmethod
    def update_invoice_path(db: Session, transaction_id: int, pdf_path: str) -> bool:
        """
        Update the invoice_pdf_path for a transaction.

        Args:
            db: Database session
            transaction_id: ID of the transaction
            pdf_path: Path to the generated PDF

        Returns:
            bool: True if successful
        """
        transaction = db.query(Transaction).filter(Transaction.id == transaction_id).first()
        if not transaction:
            return False

        transaction.invoice_pdf_path = pdf_path
        db.commit()
        return True
