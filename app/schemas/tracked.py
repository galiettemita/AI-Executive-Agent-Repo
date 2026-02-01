from pydantic import BaseModel
from typing import Optional

class TrackProductRequest(BaseModel):
    user_id: str
    walmart_item_id: str
    target_price: Optional[float] = None
    zip_code: Optional[str] = None

class TrackedItemOut(BaseModel):
    walmart_item_id: str
    last_price: Optional[float] = None
    target_price: Optional[float] = None
    zip_code: Optional[str] = None

class ListTrackedResponse(BaseModel):
    items: list[TrackedItemOut]
