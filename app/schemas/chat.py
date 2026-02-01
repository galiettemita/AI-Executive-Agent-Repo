from __future__ import annotations
from pydantic import BaseModel
from typing import Optional


class ChatRequest(BaseModel):
    user_id: str
    message: str
    conversation_id: Optional[int] = None  # if None, backend creates one


class ChatResponse(BaseModel):
    conversation_id: int
    reply: str
