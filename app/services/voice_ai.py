# app/services/voice_ai.py

from __future__ import annotations

import json
from typing import Dict, List, Optional

from app.services.llm_client import OpenAIProxy as OpenAI

from app.services.voice_scenarios import get_scenario
from app.core.config import settings


def _client() -> OpenAI:
    return OpenAI(api_key=settings.OPENAI_API_KEY)


def generate_call_response(
    purpose: Optional[str],
    conversation: List[Dict[str, str]],
    last_user_utterance: str,
    script: Optional[str] = None,
) -> str:
    """
    Generate the next assistant response during a call.
    """
    scenario = get_scenario(purpose)

    script_block = ""
    if script:
        try:
            script_data = json.loads(script) if isinstance(script, str) else script
            script_block = (
                f"Call script (use as guidance): {json.dumps(script_data, ensure_ascii=False)}. "
            )
        except Exception:
            script_block = f"Call script (use as guidance): {script}. "

    system = (
        "You are an AI phone agent speaking in real time. "
        "Be concise, polite, and goal-oriented. "
        "Confirm key details before ending. "
        f"Scenario: {scenario['title']}. Goal: {scenario['goal']}. "
        f"Required fields: {scenario['required_fields']}. "
        f"{script_block}"
        "If you need missing details, ask one question at a time. "
        "If you confirm a detail, ask: 'Did I get that right?' "
        "Never mention you are an AI unless asked."
    )

    messages = [{"role": "system", "content": system}] + conversation
    messages.append({"role": "user", "content": last_user_utterance})

    model = settings.OPENAI_MODEL
    resp = _client().chat.completions.create(
        model=model,
        messages=messages,
        temperature=0.3,
        max_tokens=200,
    )
    return resp.choices[0].message.content.strip()


def summarize_call(transcript: str, purpose: Optional[str]) -> Dict[str, object]:
    """
    Summarize the call and extract action items.
    """
    scenario = get_scenario(purpose)
    system = (
        "You summarize phone calls for a personal assistant. "
        "Return JSON with keys: summary (string), action_items (array of short strings). "
        f"Scenario: {scenario['title']}. Goal: {scenario['goal']}."
    )

    model = settings.OPENAI_MODEL
    resp = _client().chat.completions.create(
        model=model,
        messages=[
            {"role": "system", "content": system},
            {"role": "user", "content": transcript[:12000]},
        ],
        temperature=0.2,
        max_tokens=250,
    )

    text = resp.choices[0].message.content.strip()
    try:
        return json.loads(text)
    except Exception:
        return {"summary": text, "action_items": []}
