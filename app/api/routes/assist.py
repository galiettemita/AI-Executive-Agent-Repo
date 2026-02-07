from fastapi import APIRouter, Request
from app.schemas.assist import CheckoutPlanRequest, CheckoutPlanResponse
from app.services.checkout_planner import build_checkout_plan
from app.middleware.rate_limiter import rate_limit_user

router = APIRouter()

@rate_limit_user()
@router.post("/checkout-plan", response_model=CheckoutPlanResponse)
def checkout_plan(request: Request, req: CheckoutPlanRequest):
    return build_checkout_plan(url=req.url, product_hint=req.product_hint)
