# backend/app/services/travel_agent.py

import json
from typing import Any, Dict, List
from openai import OpenAI
from sqlalchemy.orm import Session
from app.core.config import settings

client = OpenAI(api_key=settings.OPENAI_API_KEY)


def run_travel_agent(
    db: Session,
    user_id: str,
    history: List[Dict[str, str]],
    user_message: str,
    preferences: Dict[str, str],
) -> Dict[str, Any]:
    """
    Travel planning agent for flights, hotels, and itineraries.
    Creates travel itinerary proposals that require user approval.
    """

    # Extract relevant preferences
    user_budget = preferences.get("budget", "not specified")
    user_taste = preferences.get("taste", "not specified")

    system = (
        "You are a travel planning assistant inside Orbit.\n"
        "Your role is to help users plan trips, find flights, book hotels, and create itineraries.\n"
        "\n"
        f"USER PREFERENCES:\n"
        f"- Budget: {user_budget}\n"
        f"- Taste: {user_taste}\n"
        "\n"
        "GUIDELINES:\n"
        "- Ask clarifying questions: destination, dates, travelers, budget, priorities (price/comfort/time)\n"
        "- Suggest specific flight options with airlines, times, prices\n"
        "- Recommend hotels with location, price, ratings\n"
        "- Create day-by-day itineraries when appropriate\n"
        "- Be practical about travel times, layovers, and logistics\n"
        "- Keep responses SHORT (2-4 sentences) unless detailed itinerary requested\n"
        "\n"
        "CREATING TRAVEL ITINERARY PROPOSALS:\n"
        "When the user is ready to book, create a proposal with this format:\n"
        'TRAVEL_PROPOSAL {"destination": "city, country", "dates": {"departure": "YYYY-MM-DD", "return": "YYYY-MM-DD"}, "flights": [{"airline": "name", "route": "A to B", "price": 0.0, "notes": "nonstop/layover"}], "hotels": [{"name": "hotel", "nights": 0, "price_per_night": 0.0, "notes": "location/amenities"}], "estimated_total": 0.0, "notes": "special requests or considerations"}\n'
        "\n"
        "Only include TRAVEL_PROPOSAL when:\n"
        "- User has confirmed destination and dates\n"
        "- You have specific flight and/or hotel options\n"
        "- User is ready to book\n"
        "\n"
        "Keep your tone helpful, efficient, and excited about travel. You're helping them plan their trip."
    )

    input_messages = [{"role": "system", "content": system}]
    input_messages.extend(history[-15:])
    input_messages.append({"role": "user", "content": user_message})

    resp = client.responses.create(
        model=settings.OPENAI_MODEL,
        input=input_messages,
    )

    reply = resp.output_text.strip()

    # Check if agent wants to create a travel proposal
    if "TRAVEL_PROPOSAL " in reply:
        lines = reply.split("\n")
        proposal_line = None
        clean_lines = []

        for line in lines:
            if line.strip().startswith("TRAVEL_PROPOSAL "):
                proposal_line = line.strip()
            else:
                clean_lines.append(line)

        reply = "\n".join(clean_lines).strip()

        if proposal_line:
            try:
                proposal_json = proposal_line.replace("TRAVEL_PROPOSAL ", "")
                proposal_data = json.loads(proposal_json)

                return {
                    "proposal": {
                        "type": "travel_itinerary",
                        "summary": reply or "I've created a travel itinerary for you to review.",
                        "payload": proposal_data,
                    }
                }
            except json.JSONDecodeError:
                pass

    return {"assistant_message": reply}
