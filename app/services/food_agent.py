# backend/app/services/food_agent.py

import json
from typing import Any, Dict, List
from app.services.llm_client import OpenAIProxy as OpenAI
from sqlalchemy.orm import Session
from app.core.config import settings

client = OpenAI(api_key=settings.OPENAI_API_KEY)


def run_food_agent(
    db: Session,
    user_id: str,
    history: List[Dict[str, str]],
    user_message: str,
    preferences: Dict[str, str],
) -> Dict[str, Any]:
    """
    Food ordering agent for restaurant selection and meal ordering.
    Creates food order proposals that require user approval.
    """

    # Extract relevant preferences
    user_budget = preferences.get("budget", "not specified")
    user_taste = preferences.get("taste", "not specified")

    system = (
        "You are a food ordering assistant inside Orbit.\n"
        "Your role is to help users find restaurants, build meal orders, and handle food delivery/pickup.\n"
        "\n"
        f"USER PREFERENCES:\n"
        f"- Budget: {user_budget}\n"
        f"- Taste: {user_taste}\n"
        "\n"
        "GUIDELINES:\n"
        "- Ask clarifying questions: cuisine type, dietary restrictions, delivery or pickup, budget per person\n"
        "- Suggest specific restaurants and menu items\n"
        "- Be practical about delivery times and availability\n"
        "- Consider user's location if known\n"
        "- Keep responses SHORT (2-4 sentences) unless detailed recommendations requested\n"
        "\n"
        "CREATING FOOD ORDER PROPOSALS:\n"
        "When the user is ready to order, create a proposal with this format:\n"
        'FOOD_PROPOSAL {"restaurant": "name", "cuisine": "type", "items": [{"name": "item", "quantity": 1, "notes": "customization"}], "delivery_method": "delivery/pickup", "estimated_total": 0.0, "notes": "any special instructions"}\n'
        "\n"
        "Only include FOOD_PROPOSAL when:\n"
        "- User has confirmed what they want to order\n"
        "- You have specific restaurant and menu items\n"
        "- User is ready to place the order\n"
        "\n"
        "Keep your tone friendly, helpful, and efficient. You're helping them get food, not writing a restaurant review."
    )

    input_messages = [{"role": "system", "content": system}]
    input_messages.extend(history[-15:])
    input_messages.append({"role": "user", "content": user_message})

    resp = client.responses.create(
        model=settings.OPENAI_MODEL,
        input=input_messages,
    )

    reply = resp.output_text.strip()

    # Check if agent wants to create a food proposal
    if "FOOD_PROPOSAL " in reply:
        lines = reply.split("\n")
        proposal_line = None
        clean_lines = []

        for line in lines:
            if line.strip().startswith("FOOD_PROPOSAL "):
                proposal_line = line.strip()
            else:
                clean_lines.append(line)

        reply = "\n".join(clean_lines).strip()

        if proposal_line:
            try:
                proposal_json = proposal_line.replace("FOOD_PROPOSAL ", "")
                proposal_data = json.loads(proposal_json)

                return {
                    "proposal": {
                        "type": "food_order",
                        "summary": reply or "I've created a food order for you to review.",
                        "payload": proposal_data,
                    }
                }
            except json.JSONDecodeError:
                pass

    return {"assistant_message": reply}
