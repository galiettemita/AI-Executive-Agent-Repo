from __future__ import annotations

import hashlib
import logging
import secrets
from datetime import datetime, timedelta
from typing import Optional

from sqlalchemy import text
from sqlalchemy.orm import Session
from twilio.rest import Client

from app.core.config import settings
from app.services.profile_service import update_profile
from app.services.preferences import update_preferences

logger = logging.getLogger(__name__)


def _hash_code(code: str) -> str:
    secret = settings.PII_ENCRYPTION_KEY or settings.JWT_SECRET or ""
    raw = f"{secret}:{code}".encode("utf-8")
    return hashlib.sha256(raw).hexdigest()


def _generate_code(length: int) -> str:
    max_val = 10 ** length
    return str(secrets.randbelow(max_val)).zfill(length)


def _twilio_client() -> Client:
    account_sid = settings.TWILIO_ACCOUNT_SID or ""
    auth_token = settings.TWILIO_AUTH_TOKEN or ""
    if not account_sid or not auth_token:
        raise RuntimeError("Twilio credentials not configured")
    return Client(account_sid, auth_token)


def _send_sms(phone_number: str, message: str) -> None:
    from_number = settings.TWILIO_PHONE_NUMBER or ""
    if not from_number:
        raise RuntimeError("TWILIO_PHONE_NUMBER not configured")
    client = _twilio_client()
    client.messages.create(
        to=phone_number,
        from_=from_number,
        body=message,
    )


def _dialect(db: Session) -> str:
    bind = getattr(db, "bind", None)
    dialect = getattr(bind, "dialect", None)
    return str(getattr(dialect, "name", "") or "")


def _column_exists(db: Session, table_name: str, column_name: str) -> bool:
    try:
        if _dialect(db) == "sqlite":
            rows = db.execute(text(f"PRAGMA table_info({table_name})")).mappings().all()
            return any(str(r.get("name") or "") == column_name for r in rows)
        row = db.execute(
            text(
                "select 1 from information_schema.columns "
                "where table_schema = current_schema() and table_name = :table and column_name = :column"
            ),
            {"table": table_name, "column": column_name},
        ).first()
        return bool(row)
    except Exception:
        return False


def _column_is_uuid(db: Session, table_name: str, column_name: str) -> bool:
    if _dialect(db) == "sqlite":
        return False
    try:
        row = db.execute(
            text(
                "select data_type, udt_name from information_schema.columns "
                "where table_schema = current_schema() and table_name = :table and column_name = :column "
                "limit 1"
            ),
            {"table": table_name, "column": column_name},
        ).mappings().first()
        if not row:
            return False
        data_type = str(row.get("data_type") or "").lower()
        udt_name = str(row.get("udt_name") or "").lower()
        return data_type == "uuid" or udt_name == "uuid"
    except Exception:
        return False


def _select_pending_verification(db: Session, *, user_id: str, phone_number: str) -> Optional[dict]:
    if _dialect(db) == "sqlite":
        stmt = text(
            """
            select id, user_id, phone_number, code_hash, status, attempts, max_attempts,
                   expires_at, verified_at, last_sent_at, created_at
            from phone_verifications
            where user_id = :user_id and phone_number = :phone_number and status = 'pending'
            order by created_at desc
            limit 1
            """
        )
    else:
        stmt = text(
            """
            select id, user_id::text as user_id, phone_number, code_hash, status, attempts, max_attempts,
                   expires_at, verified_at, last_sent_at, created_at
            from phone_verifications
            where user_id::text = :user_id and phone_number = :phone_number and status = 'pending'
            order by created_at desc
            limit 1
            """
        )
    row = db.execute(stmt, {"user_id": user_id, "phone_number": phone_number}).mappings().first()
    return dict(row) if row else None


def _update_verification_status(
    db: Session,
    *,
    verification_id: int,
    status: str,
    attempts: Optional[int] = None,
    verified_at: Optional[datetime] = None,
) -> None:
    has_updated_at = _column_exists(db, "phone_verifications", "updated_at")
    assignments = ["status = :status"]
    params: dict[str, object] = {"id": verification_id, "status": status}
    if attempts is not None:
        assignments.append("attempts = :attempts")
        params["attempts"] = int(attempts)
    if verified_at is not None:
        assignments.append("verified_at = :verified_at")
        params["verified_at"] = verified_at
    if has_updated_at:
        assignments.append("updated_at = :updated_at")
        params["updated_at"] = datetime.utcnow()

    db.execute(
        text(f"update phone_verifications set {', '.join(assignments)} where id = :id"),
        params,
    )
    db.commit()


def _insert_phone_verification(
    db: Session,
    *,
    user_id: str,
    phone_number: str,
    code_hash: str,
    status: str,
    attempts: int,
    max_attempts: int,
    expires_at: datetime,
    last_sent_at: datetime,
    created_at: datetime,
) -> None:
    has_updated_at = _column_exists(db, "phone_verifications", "updated_at")
    user_id_is_uuid = _column_is_uuid(db, "phone_verifications", "user_id")

    user_expr = "(:user_id)::uuid" if (_dialect(db) != "sqlite" and user_id_is_uuid) else ":user_id"
    columns = [
        "user_id",
        "phone_number",
        "code_hash",
        "status",
        "attempts",
        "max_attempts",
        "expires_at",
        "last_sent_at",
        "created_at",
    ]
    values = [
        user_expr,
        ":phone_number",
        ":code_hash",
        ":status",
        ":attempts",
        ":max_attempts",
        ":expires_at",
        ":last_sent_at",
        ":created_at",
    ]
    params: dict[str, object] = {
        "user_id": user_id,
        "phone_number": phone_number,
        "code_hash": code_hash,
        "status": status,
        "attempts": int(attempts),
        "max_attempts": int(max_attempts),
        "expires_at": expires_at,
        "last_sent_at": last_sent_at,
        "created_at": created_at,
    }
    if has_updated_at:
        columns.append("updated_at")
        values.append(":updated_at")
        params["updated_at"] = datetime.utcnow()

    db.execute(
        text(f"insert into phone_verifications ({', '.join(columns)}) values ({', '.join(values)})"),
        params,
    )
    db.commit()


def _as_datetime(value: object) -> datetime | None:
    if value is None:
        return None
    if isinstance(value, datetime):
        return value
    if isinstance(value, str):
        raw = value.strip()
        if not raw:
            return None
        # SQLite commonly returns naive ISO strings.
        normalized = raw.replace("Z", "+00:00")
        try:
            return datetime.fromisoformat(normalized)
        except Exception:
            return None
    return None


def request_phone_verification(
    db: Session,
    user_id: str,
    phone_number: str,
    force_code: Optional[str] = None,
) -> dict:
    now = datetime.utcnow()
    phone_number = (phone_number or "").strip()
    if not phone_number:
        raise ValueError("phone_number is required")

    existing = _select_pending_verification(db, user_id=user_id, phone_number=phone_number)

    existing_expires_at = _as_datetime((existing or {}).get("expires_at")) if existing else None
    if existing and existing_expires_at and existing_expires_at < now:
        _update_verification_status(db, verification_id=int(existing["id"]), status="expired")
        existing = None

    existing_last_sent_at = _as_datetime((existing or {}).get("last_sent_at")) if existing else None
    if existing and existing_last_sent_at:
        cooldown = settings.PHONE_VERIFICATION_RESEND_COOLDOWN_SECONDS
        delta = (now - existing_last_sent_at).total_seconds()
        if delta < cooldown:
            return {
                "ok": True,
                "status": "cooldown",
                "retry_after_seconds": int(cooldown - delta),
                "expires_at": existing_expires_at.isoformat() if existing_expires_at else None,
            }

    code = force_code or _generate_code(settings.PHONE_VERIFICATION_CODE_LENGTH)
    code_hash = _hash_code(code)
    expires_at = now + timedelta(minutes=settings.PHONE_VERIFICATION_CODE_TTL_MINUTES)

    _insert_phone_verification(
        db,
        user_id=user_id,
        phone_number=phone_number,
        code_hash=code_hash,
        status="pending",
        attempts=0,
        max_attempts=settings.PHONE_VERIFICATION_MAX_ATTEMPTS,
        expires_at=expires_at,
        last_sent_at=now,
        created_at=now,
    )

    message = f"Your Executive AI Agent verification code is {code}. It expires in {settings.PHONE_VERIFICATION_CODE_TTL_MINUTES} minutes."
    try:
        _send_sms(phone_number, message)
    except Exception as exc:
        if settings.ENV in ("production", "staging"):
            logger.error("Phone verification SMS failed: %s", exc)
            raise
        logger.warning("Phone verification SMS skipped in %s: %s", settings.ENV, exc)

    payload = {
        "ok": True,
        "status": "sent",
        "expires_at": expires_at.isoformat(),
    }
    if settings.ENV == "dev" and settings.PHONE_VERIFICATION_ALLOW_DEV_CODE_ECHO == "1":
        payload["code"] = code
    return payload


def verify_phone_code(
    db: Session,
    user_id: str,
    phone_number: str,
    code: str,
    *,
    apply_profile_updates: bool = True,
) -> dict:
    now = datetime.utcnow()
    phone_number = (phone_number or "").strip()
    if not phone_number or not code:
        raise ValueError("phone_number and code are required")

    record = _select_pending_verification(db, user_id=user_id, phone_number=phone_number)

    if not record:
        raise ValueError("No pending verification found")

    record_expires_at = _as_datetime(record.get("expires_at"))
    if record_expires_at and record_expires_at < now:
        _update_verification_status(db, verification_id=int(record["id"]), status="expired")
        raise ValueError("Verification code has expired")

    attempts = int(record.get("attempts") or 0)
    max_attempts = int(record.get("max_attempts") or settings.PHONE_VERIFICATION_MAX_ATTEMPTS)

    if attempts >= max_attempts:
        _update_verification_status(db, verification_id=int(record["id"]), status="locked", attempts=attempts)
        raise ValueError("Too many attempts. Please request a new code")

    if _hash_code(code) != str(record.get("code_hash") or ""):
        attempts += 1
        status = "locked" if attempts >= max_attempts else "pending"
        _update_verification_status(
            db,
            verification_id=int(record["id"]),
            status=status,
            attempts=attempts,
        )
        raise ValueError("Invalid verification code")

    _update_verification_status(
        db,
        verification_id=int(record["id"]),
        status="verified",
        attempts=attempts,
        verified_at=now,
    )

    if apply_profile_updates:
        # Update user profile + preferences
        update_profile(
            db,
            user_id,
            {
                "phone": phone_number,
                "phone_verified_at": now.isoformat(),
            },
        )
        update_preferences(db, user_id, {"phone_verified": True})

    return {"ok": True, "status": "verified", "verified_at": now.isoformat()}
