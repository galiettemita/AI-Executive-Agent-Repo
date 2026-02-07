from fastapi import APIRouter, Request
from app.schemas.chat import ChatRequest, ChatResponse
from app.middleware.rate_limiter import rate_limit_user

router = APIRouter()

@rate_limit_user()
@router.post("", response_model=ChatResponse)
def chat(request: Request, req: ChatRequest):
    return ChatResponse(reply=f"You said: {req.message}")
