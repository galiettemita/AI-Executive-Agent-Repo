from pydantic import BaseModel
from typing import Optional

class CheckoutPlanRequest(BaseModel):
    user_id: str
    url: str
    product_hint: Optional[str] = None

class CheckoutPlanResponse(BaseModel):
    retailer_domain: str
    steps: list[str]
    notes: list[str] = []
