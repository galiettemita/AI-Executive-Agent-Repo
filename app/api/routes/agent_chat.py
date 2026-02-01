from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.db.models import ChatMessage
from app.schemas.chat import ChatRequest, ChatResponse
from app.services.orchestrator import run_orchestrator
from app.services.memory import update_memory_from_turn

router = APIRouter(prefix="/agent", tags=["agent"])


@router.post("/chat", response_model=ChatResponse)
def agent_chat(req: ChatRequest, db: Session = Depends(get_db)):
    # If client didn't provide a conversation_id, create a simple numeric one.
    # MVP approach: just use max(conversation_id)+1 for this user.
    if req.conversation_id is None:
        last = (
            db.query(ChatMessage)
            .filter(ChatMessage.user_id == req.user_id)
            .order_by(ChatMessage.conversation_id.desc())
            .first()
        )
        conversation_id = (last.conversation_id + 1) if last else 1
    else:
        conversation_id = req.conversation_id

    # Load recent history
    msgs = (
        db.query(ChatMessage)
        .filter(
            ChatMessage.user_id == req.user_id,
            ChatMessage.conversation_id == conversation_id,
        )
        .order_by(ChatMessage.id.desc())
        .limit(20)
        .all()
    )
    msgs = list(reversed(msgs))
    history = [{"role": m.role, "content": m.content} for m in msgs]

    # Save user message
    db.add(
        ChatMessage(
            conversation_id=conversation_id,
            user_id=req.user_id,
            role="user",
            content=req.message,
        )
    )
    db.commit()

    # Run agent
    try:
        reply = run_orchestrator(db=db, user_id=req.user_id, history=history, user_message=req.message)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

    # Save assistant message
    db.add(
        ChatMessage(
            conversation_id=conversation_id,
            user_id=req.user_id,
            role="assistant",
            content=reply,
        )
    )
    db.commit()

    # Update memory (non-blocking)
    try:
        update_memory_from_turn(
            db=db,
            user_id=req.user_id,
            user_message=req.message,
            assistant_message=reply,
        )
    except Exception:
        pass

    return ChatResponse(conversation_id=conversation_id, reply=reply)