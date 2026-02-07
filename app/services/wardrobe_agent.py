# backend/app/services/wardrobe_agent.py

import json
from typing import Any, Dict, List
from openai import OpenAI
from sqlalchemy.orm import Session
from app.core.config import settings

client = OpenAI(api_key=settings.OPENAI_API_KEY)


def run_wardrobe_agent(
    db: Session,
    user_id: str,
    history: List[Dict[str, str]],
    user_message: str,
    preferences: Dict[str, str],
) -> Dict[str, Any]:
    """
    Wardrobe agent for outfit generation and style advice.
    Can create shopping list proposals for wardrobe items.
    """

    # Extract wardrobe preferences
    wardrobe_context = {
        "vibe": preferences.get("wardrobe_vibe", "not specified"),
        "sizes": preferences.get("wardrobe_sizes", "not specified"),
        "colors": preferences.get("wardrobe_colors", "not specified"),
        "budget": preferences.get("wardrobe_budget", preferences.get("budget", "not specified")),
    }

    system = (
        "You are a personal stylist and wardrobe assistant inside Orbit.\n"
        "Your role is to help users with outfit suggestions, style advice, and wardrobe planning.\n"
        "\n"
        "WARDROBE CONTEXT:\n"
        f"{json.dumps(wardrobe_context, indent=2)}\n"
        "\n"
        "GUIDELINES:\n"
        "- Keep responses SHORT (2-4 sentences) unless detailed advice is requested\n"
        "- Ask clarifying questions if needed: occasion, weather, existing items, constraints\n"
        "- Suggest specific outfit combinations with reasoning\n"
        "- Consider the user's vibe, colors, and budget preferences\n"
        "- Be practical and realistic\n"
        "- If suggesting items to buy, you can create a shopping list proposal\n"
        "\n"
        "RESPONSE FORMATS:\n"
        "1. For outfit suggestions: describe the complete look with specific items\n"
        "2. For shopping needs: you can suggest creating a proposal with items to buy\n"
        "\n"
        "CREATING PROPOSALS:\n"
        "If the user wants to shop for wardrobe items, include in your response:\n"
        "WARDROBE_PROPOSAL {\"items\": [{\"name\": \"item name\", \"category\": \"tops/bottoms/shoes/accessories\", \"notes\": \"why this fits their style\"}]}\n"
        "Only include WARDROBE_PROPOSAL if the user is ready to shop and you have specific recommendations.\n"
        "\n"
        "Keep your tone friendly, confident, and helpful. You're their personal stylist."
    )

    input_messages = [{"role": "system", "content": system}]
    input_messages.extend(history[-15:])  # Keep recent context
    input_messages.append({"role": "user", "content": user_message})

    resp = client.responses.create(
        model=settings.OPENAI_MODEL,
        input=input_messages,
    )

    reply = resp.output_text.strip()

    # Check if agent wants to create a wardrobe proposal
    if "WARDROBE_PROPOSAL " in reply:
        lines = reply.split("\n")
        proposal_line = None
        clean_lines = []

        for line in lines:
            if line.strip().startswith("WARDROBE_PROPOSAL "):
                proposal_line = line.strip()
            else:
                clean_lines.append(line)

        reply = "\n".join(clean_lines).strip()

        if proposal_line:
            try:
                proposal_json = proposal_line.replace("WARDROBE_PROPOSAL ", "")
                proposal_data = json.loads(proposal_json)

                return {
                    "proposal": {
                        "type": "wardrobe_shopping_list",
                        "summary": reply or "I've created a wardrobe shopping list for you.",
                        "payload": proposal_data,
                    }
                }
            except json.JSONDecodeError:
                pass  # If parsing fails, just return the message

    return {"assistant_message": reply}
