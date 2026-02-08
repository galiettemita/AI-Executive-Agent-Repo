from __future__ import annotations

from typing import Optional, Iterable, Dict

from sqlalchemy.orm import Session

from app.db.models import BetaTester


def list_beta_testers(db: Session, status: Optional[str] = None, limit: int = 200):
    query = db.query(BetaTester)
    if status:
        query = query.filter(BetaTester.status == status)
    return query.order_by(BetaTester.created_at.desc()).limit(limit).all()


def upsert_beta_tester(
    db: Session,
    user_id: str,
    email: Optional[str] = None,
    status: str = "active",
    notes: Optional[str] = None,
) -> BetaTester:
    tester = db.query(BetaTester).filter(BetaTester.user_id == user_id).first()
    if tester:
        if email is not None:
            tester.email = email
        if status:
            tester.status = status
        if notes is not None:
            tester.notes = notes
    else:
        tester = BetaTester(
            user_id=user_id,
            email=email,
            status=status or "active",
            notes=notes,
        )
        db.add(tester)
    db.commit()
    db.refresh(tester)
    return tester


def upsert_beta_testers_bulk(
    db: Session,
    testers: Iterable[Dict[str, Optional[str]]],
) -> int:
    count = 0
    for item in testers:
        user_id = (item.get("user_id") or "").strip() if item else ""
        if not user_id:
            continue
        upsert_beta_tester(
            db=db,
            user_id=user_id,
            email=item.get("email"),
            status=item.get("status") or "active",
            notes=item.get("notes"),
        )
        count += 1
    return count


def summarize_beta_testers(db: Session) -> Dict[str, int]:
    total = db.query(BetaTester.id).count()
    active = db.query(BetaTester.id).filter(BetaTester.status == "active").count()
    paused = db.query(BetaTester.id).filter(BetaTester.status == "paused").count()
    removed = db.query(BetaTester.id).filter(BetaTester.status == "removed").count()
    return {
        "total": total,
        "active": active,
        "paused": paused,
        "removed": removed,
    }


def delete_beta_tester(db: Session, tester_id: int) -> bool:
    tester = db.query(BetaTester).filter(BetaTester.id == tester_id).first()
    if not tester:
        return False
    db.delete(tester)
    db.commit()
    return True


def is_beta_user(db: Session, user_id: str) -> bool:
    if not user_id:
        return False
    tester = db.query(BetaTester).filter(
        BetaTester.user_id == user_id,
        BetaTester.status == "active",
    ).first()
    return tester is not None


def has_beta_testers(db: Session) -> bool:
    return db.query(BetaTester.id).first() is not None
