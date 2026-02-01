from fastapi import APIRouter
from app.schemas.assist import CheckoutPlanRequest, CheckoutPlanResponse
from app.services.checkout_planner import build_checkout_plan

router = APIRouter()

@router.post("/checkout-plan", response_model=CheckoutPlanResponse)
def checkout_plan(req: CheckoutPlanRequest):
    return build_checkout_plan(url=req.url, product_hint=req.product_hint)
