# app/services/gift_thankyou.py

from __future__ import annotations

import json
from datetime import datetime
from typing import Optional

from openai import OpenAI
from sqlalchemy.orm import Session

from app.core.config import settings
from app.db.models import GiftThankYouDraft, GiftOccasion, GiftIdea


def _fallback_note(recipient: str, occasion_type: Optional[str], tone: str, length: str) -> str:
    base = f"Thank you so much, {recipient}."
    if occasion_type:
        base += f" I really appreciate the {occasion_type} gift."
    if tone == "formal":
        base += " It was incredibly thoughtful."
    if length == "long":
        base += " Your kindness means a lot to me and I’m grateful to have you in my life."
    return base


def generate_thank_you_note(
    db: Session,
    user_id: str,
    occasion_id: Optional[int],
    gift_idea_id: Optional[int],
    tone: str,
    length: str,
    extra_notes: Optional[str],
) -> GiftThankYouDraft:
    occasion = None
    idea = None
    if occasion_id:
        occasion = db.query(GiftOccasion).filter(GiftOccasion.user_id == user_id, GiftOccasion.id == occasion_id).one_or_none()
    if gift_idea_id:
        idea = db.query(GiftIdea).filter(GiftIdea.user_id == user_id, GiftIdea.id == gift_idea_id).one_or_none()

    recipient = occasion.recipient_name if occasion else "there"
    occasion_type = occasion.occasion_type if occasion else None

    message = None
    if settings.OPENAI_API_KEY and getattr(settings, "GIFT_LLM_ENABLED", "1") == "1":
        client = OpenAI(api_key=settings.OPENAI_API_KEY)
        prompt = {
            "recipient": recipient,
            "occasion_type": occasion_type,
            "gift": idea.title if idea else None,
            "tone": tone,
            "length": length,
            "extra_notes": extra_notes,
        }
        system = (
            "You write concise thank-you notes. Respond with only the note text. "
            "Keep it personal and warm."
        )
        try:
            resp = client.responses.create(
                model=settings.OPENAI_MODEL,
                input=[
                    {"role": "system", "content": system},
                    {"role": "user", "content": json.dumps(prompt)},
                ],
                temperature=0.4,
            )
            message = resp.output_text.strip()
        except Exception:
            message = None

    if not message:
        message = _fallback_note(recipient, occasion_type, tone, length)
        if extra_notes:
            message += f" {extra_notes.strip()}"

    draft = GiftThankYouDraft(
        user_id=user_id,
        occasion_id=occasion_id,
        gift_idea_id=gift_idea_id,
        message=message,
        status="draft",
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(draft)
    db.commit()
    db.refresh(draft)
    return draft
