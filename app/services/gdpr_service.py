# app/services/gdpr_service.py
"""
GDPR Compliance Service

Provides functionality for:
- Right to be forgotten (data deletion)
- Data export (data portability)
- Consent management

Ensures compliance with GDPR Article 17 (Right to Erasure).
"""

from __future__ import annotations

import json
import logging
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

from sqlalchemy import delete, select, text
from sqlalchemy.orm import Session

from app.db.models import (
    User,
    OAuthToken,
    UserPreference,
    Conversation,
    ChatMessage,
    TaskItem,
    WatchItem,
    WatchOffer,
    TravelerProfile,
    PaymentMethod,
    Transaction,
    SpendingLimit,
    Subscription,
    Usage,
    Proposal,
    ProposalAuditLog,
    Booking,
    ExecutionLog,
    DeviceToken,
    WebhookEndpoint,
    WebhookDelivery,
    InboundEvent,
    NotificationQueue,
    MemoryNote,
    EmailDraft,
    EmailMonitorConfig,
    EmailAlert,
    FileAsset,
    PhotoAsset,
    UsageEvent,
    WardrobeItem,
    WardrobeItemPhoto,
    WardrobeWearEvent,
    RelationshipProfile,
    RelationshipInteraction,
    FitnessWorkout,
    FitnessMealPlan,
    NutritionLog,
    FitnessStepLog,
    EntertainmentItem,
    EntertainmentConsumption,
    GiftOccasion,
    GiftIdea,
    GiftThankYouDraft,
)

logger = logging.getLogger(__name__)


# Models that contain user data, ordered by dependency (children first)
USER_DATA_MODELS = [
    # Notifications and webhooks
    WebhookDelivery,
    WebhookEndpoint,
    NotificationQueue,
    InboundEvent,
    DeviceToken,
    EmailAlert,
    # Chat and tasks
    ChatMessage,
    Conversation,
    TaskItem,
    MemoryNote,
    EmailDraft,
    EmailMonitorConfig,
    FileAsset,
    PhotoAsset,
    WardrobeWearEvent,
    WardrobeItemPhoto,
    WardrobeItem,
    RelationshipInteraction,
    RelationshipProfile,
    NutritionLog,
    FitnessMealPlan,
    FitnessWorkout,
    FitnessStepLog,
    EntertainmentConsumption,
    EntertainmentItem,
    GiftThankYouDraft,
    GiftIdea,
    GiftOccasion,
    # Shopping/watching
    WatchOffer,
    WatchItem,
    # Travel and bookings
    ExecutionLog,
    Booking,
    ProposalAuditLog,
    Proposal,
    TravelerProfile,
    # Payments and billing
    Transaction,
    PaymentMethod,
    SpendingLimit,
    Usage,
    UsageEvent,
    Subscription,
    # User preferences and auth
    UserPreference,
    OAuthToken,
    # Finally, the user
    User,
]

EXTRA_USER_TABLES = (
    "provisioning_requests",
    "provisioning_declined",
    "mcp_user_servers",
)


class GDPRService:
    """Service for GDPR-compliant data operations."""

    def __init__(self, db: Session):
        self.db = db

    def _is_sqlite(self) -> bool:
        bind = getattr(self.db, "bind", None)
        dialect = getattr(bind, "dialect", None)
        return str(getattr(dialect, "name", "") or "") == "sqlite"

    def _get_user_row_mapping(self, user_id: str) -> Optional[Dict[str, Any]]:
        if self._is_sqlite():
            row = self.db.execute(
                text("select * from users where id = :user_id limit 1"),
                {"user_id": user_id},
            ).mappings().first()
        else:
            row = self.db.execute(
                text("select * from users where id::text = :user_id limit 1"),
                {"user_id": user_id},
            ).mappings().first()
        return dict(row) if row else None

    def _delete_user_row(self, user_id: str) -> int:
        if self._is_sqlite():
            result = self.db.execute(
                text("delete from users where id = :user_id"),
                {"user_id": user_id},
            )
        else:
            result = self.db.execute(
                text("delete from users where id::text = :user_id"),
                {"user_id": user_id},
            )
        return int(getattr(result, "rowcount", 0) or 0)

    def _table_exists(self, table_name: str) -> bool:
        if self._is_sqlite():
            row = self.db.execute(
                text("select name from sqlite_master where type='table' and name=:name"),
                {"name": table_name},
            ).first()
            return bool(row)
        row = self.db.execute(
            text(
                "select 1 from information_schema.tables "
                "where table_schema = current_schema() and table_name = :name"
            ),
            {"name": table_name},
        ).first()
        return bool(row)

    def _select_rows_from_extra_table(self, *, table_name: str, user_id: str) -> List[Dict[str, Any]]:
        if not self._table_exists(table_name):
            return []
        if self._is_sqlite():
            rows = self.db.execute(
                text(f"select * from {table_name} where user_id = :user_id"),
                {"user_id": user_id},
            ).mappings().all()
        else:
            rows = self.db.execute(
                text(f"select * from {table_name} where user_id::text = :user_id"),
                {"user_id": user_id},
            ).mappings().all()
        return [dict(row) for row in rows]

    def _delete_rows_from_extra_table(self, *, table_name: str, user_id: str) -> int:
        if not self._table_exists(table_name):
            return 0
        if self._is_sqlite():
            res = self.db.execute(
                text(f"delete from {table_name} where user_id = :user_id"),
                {"user_id": user_id},
            )
        else:
            res = self.db.execute(
                text(f"delete from {table_name} where user_id::text = :user_id"),
                {"user_id": user_id},
            )
        return int(getattr(res, "rowcount", 0) or 0)

    def delete_user_data(
        self,
        user_id: str,
        dry_run: bool = False,
        keep_anonymized_transactions: bool = True,
    ) -> Dict[str, Any]:
        """
        Delete all user data (Right to be Forgotten).

        Args:
            user_id: The user ID to delete data for.
            dry_run: If True, only report what would be deleted without actually deleting.
            keep_anonymized_transactions: If True, anonymize transactions instead of deleting
                                          (required for financial records).

        Returns:
            Dictionary with deletion summary.
        """
        deletion_summary: Dict[str, Any] = {
            "user_id": user_id,
            "dry_run": dry_run,
            "deleted_at": datetime.now(timezone.utc).isoformat(),
            "tables": {},
            "errors": [],
        }

        try:
            for model in USER_DATA_MODELS:
                table_name = model.__tablename__
                count = 0

                # Special handling for transactions (anonymize instead of delete)
                if model == Transaction and keep_anonymized_transactions:
                    count = self._anonymize_transactions(user_id, dry_run)
                    deletion_summary["tables"][table_name] = {
                        "action": "anonymized",
                        "count": count,
                    }
                    continue

                # Special handling for User model
                if model == User:
                    user_row = self._get_user_row_mapping(user_id)
                    if user_row:
                        if not dry_run:
                            self._delete_user_row(user_id)
                        count = 1
                else:
                    # Get user_id field name (most models use 'user_id')
                    user_id_field = self._get_user_id_field(model)
                    if user_id_field is None:
                        continue

                    # Count records
                    stmt = select(model).where(getattr(model, user_id_field) == user_id)
                    records = self.db.execute(stmt).scalars().all()
                    count = len(records)

                    if not dry_run and count > 0:
                        delete_stmt = delete(model).where(
                            getattr(model, user_id_field) == user_id
                        )
                        self.db.execute(delete_stmt)

                deletion_summary["tables"][table_name] = {
                    "action": "deleted",
                    "count": count,
                }

            for table_name in EXTRA_USER_TABLES:
                rows = self._select_rows_from_extra_table(table_name=table_name, user_id=user_id)
                count = len(rows)
                if not dry_run and count > 0:
                    count = self._delete_rows_from_extra_table(table_name=table_name, user_id=user_id)
                deletion_summary["tables"][table_name] = {
                    "action": "deleted",
                    "count": count,
                }

            if not dry_run:
                self.db.commit()
                logger.info(f"GDPR deletion completed for user {user_id}")
            else:
                self.db.rollback()
                logger.info(f"GDPR deletion dry-run completed for user {user_id}")

            deletion_summary["success"] = True

        except Exception as e:
            self.db.rollback()
            logger.error(f"GDPR deletion failed for user {user_id}: {e}")
            deletion_summary["success"] = False
            deletion_summary["errors"].append(str(e))

        return deletion_summary

    def _get_user_id_field(self, model) -> Optional[str]:
        """Get the user_id field name for a model."""
        # Most models use 'user_id'
        if hasattr(model, "user_id"):
            return "user_id"
        # Some models might use 'id' directly (like User)
        if model == User:
            return "id"
        return None

    def _anonymize_transactions(self, user_id: str, dry_run: bool) -> int:
        """
        Anonymize transactions instead of deleting (for financial records).

        Replaces user_id with 'DELETED_USER' and removes any PII.
        """
        stmt = select(Transaction).where(Transaction.user_id == user_id)
        transactions = self.db.execute(stmt).scalars().all()
        count = len(transactions)

        if not dry_run:
            for txn in transactions:
                txn.user_id = f"DELETED_{user_id[:8]}"
                # Clear any PII fields if present
                if hasattr(txn, "metadata_json") and txn.metadata_json:
                    try:
                        metadata = json.loads(txn.metadata_json)
                        # Remove PII from metadata
                        pii_fields = ["email", "phone", "name", "address"]
                        for field in pii_fields:
                            metadata.pop(field, None)
                        txn.metadata_json = json.dumps(metadata)
                    except (json.JSONDecodeError, TypeError):
                        pass

        return count

    def export_user_data(self, user_id: str) -> Dict[str, Any]:
        """
        Export all user data (Data Portability).

        Args:
            user_id: The user ID to export data for.

        Returns:
            Dictionary containing all user data.
        """
        export_data: Dict[str, Any] = {
            "user_id": user_id,
            "exported_at": datetime.now(timezone.utc).isoformat(),
            "data": {},
        }

        for model in USER_DATA_MODELS:
            table_name = model.__tablename__
            user_id_field = self._get_user_id_field(model)

            if user_id_field is None:
                continue

            if model == User:
                user_row = self._get_user_row_mapping(user_id)
                if user_row:
                    export_data["data"][table_name] = [self._serialize_mapping(user_row)]
            else:
                stmt = select(model).where(getattr(model, user_id_field) == user_id)
                records = self.db.execute(stmt).scalars().all()
                if records:
                    export_data["data"][table_name] = [
                        self._model_to_dict(r) for r in records
                    ]

        for table_name in EXTRA_USER_TABLES:
            rows = self._select_rows_from_extra_table(table_name=table_name, user_id=user_id)
            if rows:
                export_data["data"][table_name] = [self._serialize_mapping(row) for row in rows]

        return export_data

    def _model_to_dict(self, model_instance) -> Dict[str, Any]:
        """Convert SQLAlchemy model instance to dictionary."""
        result = {}
        for column in model_instance.__table__.columns:
            value = getattr(model_instance, column.name)
            # Handle datetime serialization
            if isinstance(value, datetime):
                value = value.isoformat()
            # Handle bytes
            elif isinstance(value, bytes):
                value = "<binary data>"
            result[column.name] = value
        return result

    def get_user_data_summary(self, user_id: str) -> Dict[str, int]:
        """
        Get a summary of user data counts across all tables.

        Args:
            user_id: The user ID to summarize.

        Returns:
            Dictionary of table names to record counts.
        """
        summary = {}

        for model in USER_DATA_MODELS:
            table_name = model.__tablename__
            user_id_field = self._get_user_id_field(model)

            if user_id_field is None:
                continue

            if model == User:
                summary[table_name] = 1 if self._get_user_row_mapping(user_id) else 0
            else:
                stmt = select(model).where(getattr(model, user_id_field) == user_id)
                records = self.db.execute(stmt).scalars().all()
                summary[table_name] = len(records)

        for table_name in EXTRA_USER_TABLES:
            rows = self._select_rows_from_extra_table(table_name=table_name, user_id=user_id)
            summary[table_name] = len(rows)

        return summary

    def _serialize_mapping(self, row: Dict[str, Any]) -> Dict[str, Any]:
        result: Dict[str, Any] = {}
        for key, value in row.items():
            if isinstance(value, datetime):
                result[key] = value.isoformat()
            elif isinstance(value, bytes):
                result[key] = "<binary data>"
            else:
                result[key] = value
        return result


def delete_user_data(
    db: Session,
    user_id: str,
    dry_run: bool = False,
    keep_anonymized_transactions: bool = True,
) -> Dict[str, Any]:
    """Convenience function for deleting user data."""
    service = GDPRService(db)
    return service.delete_user_data(user_id, dry_run, keep_anonymized_transactions)


def export_user_data(db: Session, user_id: str) -> Dict[str, Any]:
    """Convenience function for exporting user data."""
    service = GDPRService(db)
    return service.export_user_data(user_id)


def get_user_data_summary(db: Session, user_id: str) -> Dict[str, int]:
    """Convenience function for getting user data summary."""
    service = GDPRService(db)
    return service.get_user_data_summary(user_id)
