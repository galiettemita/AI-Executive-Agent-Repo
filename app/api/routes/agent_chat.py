from fastapi import APIRouter, Depends, HTTPException, Request
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.api.deps import get_or_create_user
from app.schemas.chat import ChatRequest, ChatResponse
from app.services.history import get_recent_history, get_or_create_conversation, store_message, trim_history
from app.services.orchestrator import run_orchestrator
from app.services.memory import update_memory_from_turn
from app.services.usage import record_message
from app.middleware.rate_limiter import rate_limit_user

router = APIRouter(prefix="/agent", tags=["agent"])


@rate_limit_user()
@router.post("/chat", response_model=ChatResponse)
def agent_chat(request: Request, req: ChatRequest, db: Session = Depends(get_db)):
    get_or_create_user(db, req.user_id)
    convo = get_or_create_conversation(db, req.user_id)
    conversation_id = req.conversation_id or convo.id

    # Load recent history (minimal)
    history = get_recent_history(db, req.user_id)

    # Save user message
    store_message(db, req.user_id, conversation_id, "user", req.message)

    # Run agent
    try:
        reply = run_orchestrator(db=db, user_id=req.user_id, history=history, user_message=req.message)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

    # Save assistant message
    store_message(db, req.user_id, conversation_id, "assistant", reply)
    trim_history(db, req.user_id)
    record_message(db, req.user_id, count=1)

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
