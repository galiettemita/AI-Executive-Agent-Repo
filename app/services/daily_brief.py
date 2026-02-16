# backend/app/services/daily_brief.py

from __future__ import annotations

import json
from datetime import datetime
import logging
from typing import Any, Dict

from app.services.llm_client import OpenAIProxy as OpenAI
from sqlalchemy.orm import Session

from app.db.models import ChatMessage, MemoryNote
from app.services.calendar_router import get_events_for_daily_brief
from app.services.email_router import get_recent_emails_for_daily_brief
from app.core.config import settings

logger = logging.getLogger(__name__)

client = OpenAI(api_key=settings.OPENAI_API_KEY)


def generate_daily_brief(
    db: Session,
    user_id: str,
    days_ahead: int = 1,
    hours_back_emails: int = 24,
) -> Dict[str, Any]:
    """
    Generate a daily brief for a user by fetching calendar events and recent emails,
    then summarizing them with AI.

    Args:
        db: Database session
        user_id: User ID
        days_ahead: Number of days ahead for calendar events
        hours_back_emails: Hours back to look for emails

    Returns:
        Dict with:
            - success: bool
            - brief_text: str (AI-generated summary)
            - calendar_events: list
            - emails: list
            - error: str (if any)
    """
    result = {
        "success": False,
        "brief_text": "",
        "calendar_events": [],
        "emails": [],
        "error": None,
    }

    try:
        # Fetch calendar events
        calendar_events = get_events_for_daily_brief(
            db=db,
            user_id=user_id,
            days_ahead=days_ahead,
        )
        result["calendar_events"] = calendar_events

        # Fetch recent emails
        emails = get_recent_emails_for_daily_brief(
            db=db,
            user_id=user_id,
            hours_back=hours_back_emails,
        )
        result["emails"] = emails

        # If no data, return early
        if not calendar_events and not emails:
            result["brief_text"] = "Good morning! You have no upcoming calendar events or unread emails. Have a great day!"
            result["success"] = True
            return result

        # Build context for AI
        context = {
            "calendar_events": calendar_events,
            "emails": emails,
        }

        # Generate brief with AI
        system_prompt = (
            "You are a helpful personal assistant creating a concise daily brief.\n"
            "Summarize the user's upcoming calendar events and recent unread emails.\n"
            "Keep it friendly, concise, and actionable.\n"
            "Format the output in a clear, easy-to-read way with bullet points.\n"
            "Start with a greeting and overview, then cover:\n"
            "1. Important emails (highlight urgent or important senders)\n"
            "2. Today's schedule (focus on what's coming up soon)\n"
            "3. Optional: Any tasks or action items you notice\n"
            "Keep it under 200 words total.\n"
        )

        user_prompt = (
            f"Create my daily brief based on this data:\n\n"
            f"```json\n{json.dumps(context, indent=2, ensure_ascii=False)}\n```"
        )

        response = client.chat.completions.create(
            model=settings.OPENAI_MODEL,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            temperature=0.7,
            max_tokens=500,
        )

        brief_text = response.choices[0].message.content.strip()
        result["brief_text"] = brief_text
        result["success"] = True

        return result

    except Exception as e:
        result["error"] = str(e)
        result["brief_text"] = f"Sorry, I couldn't generate your daily brief. Error: {str(e)}"
        logger.error("Error generating daily brief for user %s: %s", user_id, e)
        return result


def store_daily_brief_as_message(
    db: Session,
    user_id: str,
    brief_text: str,
    conversation_id: int | None = None,
) -> ChatMessage:
    """
    Store the daily brief as an assistant message in chat history.

    Args:
        db: Database session
        user_id: User ID
        brief_text: The brief text to store
        conversation_id: Optional conversation ID (if None, uses 0 as default)

    Returns:
        The created ChatMessage
    """
    message = ChatMessage(
        conversation_id=conversation_id or 0,
        user_id=user_id,
        role="assistant",
        content=brief_text,
        created_at=datetime.utcnow(),
    )
    db.add(message)
    db.commit()
    db.refresh(message)
    return message


def update_memory_note_with_brief(
    db: Session,
    user_id: str,
    brief_summary: str,
) -> None:
    """
    Update the user's memory note to include daily brief summary.
    This helps the agent remember what the user was told in their brief.

    Args:
        db: Database session
        user_id: User ID
        brief_summary: Summary to add to memory notes
    """
    note = db.query(MemoryNote).filter(MemoryNote.user_id == user_id).one_or_none()

    today = datetime.utcnow().strftime("%Y-%m-%d")
    addition = f"\n\n[Daily Brief {today}]\n{brief_summary}"

    if note is None:
        note = MemoryNote(user_id=user_id, summary=addition)
        db.add(note)
    else:
        # Append to existing summary
        note.summary = (note.summary or "") + addition
        note.updated_at = datetime.utcnow()

    db.commit()


def generate_and_store_daily_brief(
    db: Session,
    user_id: str,
) -> Dict[str, Any]:
    """
    Generate a daily brief and store it as both a message and in memory notes.

    Args:
        db: Database session
        user_id: User ID

    Returns:
        Dict with generation result and storage info
    """
    # Generate the brief
    result = generate_daily_brief(db, user_id)

    if not result["success"]:
        return result

    # Store as message
    message = store_daily_brief_as_message(
        db=db,
        user_id=user_id,
        brief_text=result["brief_text"],
    )

    # Update memory notes with a shorter summary
    # Extract key points for memory (shorter version)
    memory_summary = f"Today's brief: {len(result['calendar_events'])} events, {len(result['emails'])} unread emails."
    update_memory_note_with_brief(
        db=db,
        user_id=user_id,
        brief_summary=memory_summary,
    )

    result["message_id"] = message.id
    result["stored"] = True

    return result
