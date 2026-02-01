from fastapi import APIRouter
from app.schemas.chat import ChatRequest, ChatResponse

router = APIRouter()

@router.post("", response_model=ChatResponse)
def chat(req: ChatRequest):
    return ChatResponse(reply=f"You said: {req.message}")
