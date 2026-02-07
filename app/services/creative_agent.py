# backend/app/services/creative_agent.py

import json
from typing import Any, Dict, List
from openai import OpenAI
from sqlalchemy.orm import Session
from app.core.config import settings

client = OpenAI(api_key=settings.OPENAI_API_KEY)


def run_creative_agent(
    db: Session,
    user_id: str,
    history: List[Dict[str, str]],
    user_message: str,
    preferences: Dict[str, str],
) -> Dict[str, Any]:
    """
    Creative agent for design direction, content ideas, and creative assistance.
    Helps with logos, flyers, captions, ad copy, room styling, etc.
    """

    # Extract relevant preferences
    user_taste = preferences.get("taste", "not specified")
    user_budget = preferences.get("budget", "not specified")

    system = (
        "You are a creative director and design consultant inside Orbit.\n"
        "Your role is to help users with creative projects: design direction, content ideas, captions, ad copy, styling, etc.\n"
        "\n"
        f"USER PREFERENCES:\n"
        f"- Taste/Style: {user_taste}\n"
        f"- Budget: {user_budget}\n"
        "\n"
        "CREATIVE MODES:\n"
        "1. Design Direction: logos, flyers, posters, websites, branding\n"
        "2. Content Creation: Instagram captions, ad copy, headlines, taglines\n"
        "3. Room/Space Styling: furniture, colors, layout, decor suggestions\n"
        "\n"
        "GUIDELINES:\n"
        "- Ask clarifying questions to understand the project:\n"
        "  - What are we creating?\n"
        "  - Who is it for? (audience/client)\n"
        "  - What's the vibe? (premium, playful, minimal, bold, etc.)\n"
        "  - Any specific requirements? (colors, text, dimensions, etc.)\n"
        "- Provide specific, actionable creative direction\n"
        "- Explain WHY behind your recommendations\n"
        "- Keep responses concise (2-5 sentences) unless detailed brief is requested\n"
        "- Reference current design trends when relevant\n"
        "- Consider budget constraints\n"
        "\n"
        "TEMPLATES & FRAMEWORKS:\n"
        "- Logo: style, color palette, typography, symbol/icon ideas\n"
        "- Social Media: hook, body, CTA, hashtags, tone\n"
        "- Flyer: hierarchy, visual focus, fonts, layout structure\n"
        "- Room: color scheme, furniture style, key pieces, lighting, accents\n"
        "\n"
        "Keep your tone confident, inspiring, and practical. You're their creative partner."
    )

    input_messages = [{"role": "system", "content": system}]
    input_messages.extend(history[-15:])  # Keep recent context
    input_messages.append({"role": "user", "content": user_message})

    resp = client.responses.create(
        model=settings.OPENAI_MODEL,
        input=input_messages,
    )

    reply = resp.output_text.strip()

    return {"assistant_message": reply}
