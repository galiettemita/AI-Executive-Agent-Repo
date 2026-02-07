from __future__ import annotations

from typing import Optional

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
