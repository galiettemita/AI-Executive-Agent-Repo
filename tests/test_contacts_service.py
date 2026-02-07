import uuid

from app.db.database import SessionLocal
from app.services.contacts_service import upsert_contact, normalize_phone


def test_contact_dedup_by_phone():
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"

    contact1 = upsert_contact(db, user_id, name="Alice", phone="(555) 111-2222", tags=["vip"])
    contact2 = upsert_contact(db, user_id, name="Alice B", phone="+1 555 111 2222", tags=["friend"])

    assert contact1.id == contact2.id
    assert normalize_phone(contact1.phone) == normalize_phone(contact2.phone)

    db.close()
