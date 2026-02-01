#Stores the iOS device’s FCM token for push later.
from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session
from app.db.database import get_db
from app.db.models import DeviceToken
from app.schemas.device import UpsertDeviceTokenRequest
from app.api.deps import get_or_create_user

router = APIRouter()

@router.post("/token")
def upsert_device_token(req: UpsertDeviceTokenRequest, db: Session = Depends(get_db)):
    get_or_create_user(db, req.user_id)

    existing = db.query(DeviceToken).filter(DeviceToken.fcm_token == req.fcm_token).first()
    if existing is None:
        db.add(DeviceToken(user_id=req.user_id, platform=req.platform, fcm_token=req.fcm_token))
    else:
        existing.user_id = req.user_id
        existing.platform = req.platform

    db.commit()
    return {"ok": True}
