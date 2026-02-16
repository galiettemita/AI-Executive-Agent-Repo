from __future__ import annotations

import base64
import io
import json
import logging
import tempfile
from typing import Any

import httpx
from PyPDF2 import PdfReader

from app.blueprint.contracts import (
    ContentProvenance,
    EmotionState,
    InboundMessage,
    InputModality,
    ProcessedMessage,
)
from app.blueprint.emotion import detect_emotion
from app.core.config import settings
from app.services.llm_client import OpenAIProxy as OpenAI

logger = logging.getLogger(__name__)


def _client() -> OpenAI:
    if not settings.OPENAI_API_KEY:
        raise RuntimeError("OPENAI_API_KEY not configured")
    return OpenAI(api_key=settings.OPENAI_API_KEY, timeout=60, max_retries=1)


def _fetch_bytes(url: str, timeout_s: int = 30) -> bytes:
    with httpx.Client(timeout=timeout_s, follow_redirects=True) as client:
        resp = client.get(url)
        resp.raise_for_status()
        return resp.content


def transcribe_audio_url(audio_url: str) -> dict[str, Any]:
    """
    Whisper transcription helper used by /api/v1/voice/transcribe and gateway multimodal preprocessing.
    """
    data = _fetch_bytes(audio_url, timeout_s=45)
    with tempfile.NamedTemporaryFile(suffix=".ogg") as tmp:
        tmp.write(data)
        tmp.flush()
        with open(tmp.name, "rb") as f:
            resp = _client().audio.transcriptions.create(
                model="whisper-1",
                file=f,
            )

    # OpenAI SDK shapes can vary by version; normalize defensively.
    text = getattr(resp, "text", None) or ""
    language = getattr(resp, "language", None)
    confidence = getattr(resp, "confidence", None)

    return {
        "text": str(text or "").strip(),
        "confidence": float(confidence) if confidence is not None else None,
        "language": language,
    }


def _extract_text_from_pdf_bytes(data: bytes) -> str:
    with io.BytesIO(data) as bio:
        reader = PdfReader(bio)
        chunks: list[str] = []
        for page in reader.pages[:20]:
            page_text = page.extract_text() or ""
            if page_text.strip():
                chunks.append(page_text)
        return "\n".join(chunks).strip()


def extract_document_text(document_url: str) -> dict[str, Any]:
    data = _fetch_bytes(document_url, timeout_s=45)
    text = _extract_text_from_pdf_bytes(data)
    entities = {
        "action_items": [],
        "dates": [],
        "contacts": [],
    }
    if text:
        # Lightweight extraction to keep phase-1 deterministic.
        lines = [ln.strip() for ln in text.splitlines() if ln.strip()]
        entities["action_items"] = [ln for ln in lines if ln.lower().startswith(("todo", "action", "next"))][:10]
    return {"text": text[:16000], "entities": entities}


def extract_image_entities(image_url: str, prompt: str | None = None) -> dict[str, Any]:
    if not settings.FEATURE_IMAGE_PROCESSING:
        return {"summary": "", "entities": {}, "raw": {}}

    system_prompt = (
        "Extract useful entities from this image for an executive assistant. "
        "Return compact JSON with keys: summary, entities."
    )
    if prompt:
        system_prompt = f"{system_prompt} User context: {prompt[:500]}"

    resp = _client().chat.completions.create(
        model="gpt-4o-mini",
        messages=[
            {"role": "system", "content": system_prompt},
            {
                "role": "user",
                "content": [
                    {"type": "text", "text": "Analyze this image."},
                    {"type": "image_url", "image_url": {"url": image_url}},
                ],
            },
        ],
        temperature=0.2,
        max_tokens=500,
    )

    content = (resp.choices[0].message.content or "").strip()
    try:
        parsed = json.loads(content)
        if isinstance(parsed, dict):
            return {
                "summary": parsed.get("summary") or "",
                "entities": parsed.get("entities") or {},
                "raw": parsed,
            }
    except Exception:
        pass

    return {
        "summary": content,
        "entities": {},
        "raw": {"unstructured": content},
    }


def preprocess_inbound_message(msg: InboundMessage) -> ProcessedMessage:
    normalized_text = (msg.content or msg.text or "").strip()
    modality = msg.input_modality
    extracted_entities: dict[str, Any] = {}
    transcription_confidence: float | None = None
    content_provenance = ContentProvenance.USER_DIRECT

    if modality == InputModality.VOICE and msg.media_url and settings.FEATURE_VOICE_INPUT:
        try:
            tr = transcribe_audio_url(msg.media_url)
            normalized_text = tr.get("text") or normalized_text
            transcription_confidence = tr.get("confidence")
            extracted_entities.update({"language": tr.get("language")})
        except Exception as exc:
            logger.warning("voice transcription failed: %s", exc)

    elif modality == InputModality.IMAGE and msg.media_url and settings.FEATURE_IMAGE_PROCESSING:
        try:
            image_info = extract_image_entities(msg.media_url, prompt=normalized_text)
            extracted_entities.update(image_info.get("entities") or {})
            summary = str(image_info.get("summary") or "").strip()
            if summary:
                normalized_text = f"{normalized_text}\n\n[Image summary]\n{summary}".strip()
        except Exception as exc:
            logger.warning("image extraction failed: %s", exc)

    elif modality == InputModality.DOCUMENT and msg.media_url and settings.FEATURE_DOCUMENT_PROCESSING:
        try:
            doc_info = extract_document_text(msg.media_url)
            extracted_entities.update(doc_info.get("entities") or {})
            doc_text = str(doc_info.get("text") or "").strip()
            if doc_text:
                normalized_text = f"{normalized_text}\n\n[Document text]\n{doc_text}".strip()
            content_provenance = ContentProvenance.DOCUMENT
        except Exception as exc:
            logger.warning("document extraction failed: %s", exc)

    if not normalized_text:
        normalized_text = "(empty message)"

    detected_emotion = detect_emotion(
        text=normalized_text,
        transcription_confidence=transcription_confidence,
    )

    return ProcessedMessage(
        original=msg,
        normalized_text=normalized_text,
        modality=modality,
        transcription_confidence=transcription_confidence,
        extracted_entities=extracted_entities,
        emotion_detected=detected_emotion if isinstance(detected_emotion, EmotionState) else EmotionState.NEUTRAL,
        content_provenance=content_provenance,
    )
