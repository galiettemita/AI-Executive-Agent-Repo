from __future__ import annotations

import io
import json
import logging
import re
import tempfile
import zipfile
from pathlib import Path
from typing import Any
from urllib.parse import urlparse
from xml.etree import ElementTree as ET

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


def _fetch_document_payload(url: str, timeout_s: int = 45) -> tuple[bytes, str | None]:
    with httpx.Client(timeout=timeout_s, follow_redirects=True) as client:
        resp = client.get(url)
        resp.raise_for_status()
        content_type = str(resp.headers.get("content-type") or "").strip() or None
        return resp.content, content_type


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


def _extract_text_from_docx_bytes(data: bytes) -> str:
    with zipfile.ZipFile(io.BytesIO(data)) as zf:
        if "word/document.xml" not in zf.namelist():
            return ""
        xml_data = zf.read("word/document.xml")
    root = ET.fromstring(xml_data)
    namespace = {"w": "http://schemas.openxmlformats.org/wordprocessingml/2006/main"}
    lines: list[str] = []
    for paragraph in root.findall(".//w:p", namespace):
        fragments = []
        for text_node in paragraph.findall(".//w:t", namespace):
            if text_node.text:
                fragments.append(text_node.text)
        line = "".join(fragments).strip()
        if line:
            lines.append(line)
    return "\n".join(lines).strip()


def _extract_text_from_xlsx_bytes(data: bytes) -> str:
    with zipfile.ZipFile(io.BytesIO(data)) as zf:
        names = set(zf.namelist())
        shared_strings: list[str] = []
        if "xl/sharedStrings.xml" in names:
            root = ET.fromstring(zf.read("xl/sharedStrings.xml"))
            for node in root.findall(".//{*}t"):
                if node.text:
                    shared_strings.append(node.text)

        lines: list[str] = []
        for name in sorted(names):
            if not name.startswith("xl/worksheets/sheet") or not name.endswith(".xml"):
                continue
            root = ET.fromstring(zf.read(name))
            for row in root.findall(".//{*}row"):
                row_values: list[str] = []
                for cell in row.findall("{*}c"):
                    cell_type = str(cell.attrib.get("t") or "").strip().lower()
                    value_node = cell.find("{*}v")
                    if value_node is None:
                        continue
                    raw_value = str(value_node.text or "").strip()
                    if not raw_value:
                        continue
                    if cell_type == "s":
                        try:
                            idx = int(raw_value)
                            raw_value = shared_strings[idx]
                        except Exception:
                            pass
                    row_values.append(raw_value)
                if row_values:
                    lines.append(", ".join(row_values))
    return "\n".join(lines).strip()


def _extract_text_from_plain_bytes(data: bytes) -> str:
    try:
        return data.decode("utf-8", errors="ignore").strip()
    except Exception:
        return ""


def _extract_action_items(lines: list[str]) -> list[str]:
    actions: list[str] = []
    patterns = (
        r"^\s*(?:todo|to do|action|next step|next steps)\s*[:\-]\s+",
        r"^\s*(?:-|\*|\d+\.)\s+\[(?:x| )\]\s+",
        r"^\s*(?:-|\*|\d+\.)\s+(?:review|send|schedule|follow up|call|draft|prepare)\b",
    )
    compiled = [re.compile(pattern, re.IGNORECASE) for pattern in patterns]
    for line in lines:
        if not line:
            continue
        if any(pattern.search(line) for pattern in compiled):
            actions.append(line.strip())
        if len(actions) >= 25:
            break
    return actions


def _extract_dates(text_value: str) -> list[str]:
    matches = []
    patterns = [
        r"\b\d{1,2}/\d{1,2}/\d{2,4}\b",
        r"\b\d{4}-\d{2}-\d{2}\b",
        r"\b(?:jan|feb|mar|apr|may|jun|jul|aug|sep|sept|oct|nov|dec)[a-z]*\s+\d{1,2}(?:,\s*\d{4})?\b",
    ]
    for pattern in patterns:
        for match in re.findall(pattern, text_value, flags=re.IGNORECASE):
            clean = str(match).strip()
            if clean and clean not in matches:
                matches.append(clean)
            if len(matches) >= 30:
                return matches
    return matches


def _extract_contacts(text_value: str) -> list[str]:
    out: list[str] = []
    for match in re.findall(r"\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b", text_value, flags=re.IGNORECASE):
        value = str(match).strip()
        if value and value not in out:
            out.append(value)
    for match in re.findall(r"\b(?:\+1[\s.-]?)?\(?\d{3}\)?[\s.-]\d{3}[\s.-]\d{4}\b", text_value):
        value = str(match).strip()
        if value and value not in out:
            out.append(value)
    return out[:30]


def _chunk_text(text_value: str, *, max_chars: int = 1200, overlap: int = 120) -> list[dict[str, Any]]:
    source = str(text_value or "").strip()
    if not source:
        return []
    chunks: list[dict[str, Any]] = []
    cursor = 0
    idx = 0
    total = len(source)
    while cursor < total and idx < 200:
        upper = min(total, cursor + max_chars)
        snippet = source[cursor:upper]
        if upper < total:
            newline_idx = snippet.rfind("\n")
            if newline_idx > max_chars // 3:
                upper = cursor + newline_idx
                snippet = source[cursor:upper]
        snippet = snippet.strip()
        if snippet:
            chunks.append(
                {
                    "index": idx,
                    "start": cursor,
                    "end": upper,
                    "text": snippet,
                }
            )
            idx += 1
        if upper >= total:
            break
        next_cursor = max(upper - overlap, cursor + 1)
        cursor = next_cursor
    return chunks


def _extract_via_unstructured(
    *,
    data: bytes,
    filename: str,
    content_type: str | None,
) -> tuple[str, list[dict[str, Any]]] | None:
    api_key = str(settings.UNSTRUCTURED_API_KEY or "").strip()
    endpoint = str(settings.UNSTRUCTURED_API_URL or "").strip()
    if not api_key or not endpoint:
        return None

    files = {
        "files": (
            filename,
            data,
            content_type or "application/octet-stream",
        )
    }
    payload = {"strategy": str(settings.UNSTRUCTURED_STRATEGY or "fast").strip() or "fast"}
    headers = {
        "Accept": "application/json",
        "unstructured-api-key": api_key,
        "Authorization": f"Bearer {api_key}",
    }
    with httpx.Client(timeout=60, follow_redirects=True) as client:
        resp = client.post(endpoint, headers=headers, data=payload, files=files)
        resp.raise_for_status()
        parsed = resp.json()
    if not isinstance(parsed, list):
        return None

    chunks: list[dict[str, Any]] = []
    text_parts: list[str] = []
    for idx, item in enumerate(parsed):
        if not isinstance(item, dict):
            continue
        text_value = str(item.get("text") or "").strip()
        if not text_value:
            continue
        metadata = item.get("metadata") if isinstance(item.get("metadata"), dict) else {}
        chunks.append(
            {
                "index": idx,
                "text": text_value,
                "type": str(item.get("type") or item.get("category") or "text").strip().lower(),
                "metadata": metadata,
            }
        )
        text_parts.append(text_value)
    combined = "\n".join(text_parts).strip()
    if not combined:
        return None
    return combined, chunks


def _infer_document_name(document_url: str, content_type: str | None) -> str:
    parsed = urlparse(document_url)
    path_name = Path(parsed.path or "").name
    if path_name:
        return path_name
    if content_type and "pdf" in content_type.lower():
        return "document.pdf"
    if content_type and "word" in content_type.lower():
        return "document.docx"
    if content_type and "spreadsheet" in content_type.lower():
        return "document.xlsx"
    return "document.bin"


def _extract_text_with_fallbacks(*, data: bytes, filename: str, content_type: str | None) -> str:
    name = str(filename or "").strip().lower()
    ctype = str(content_type or "").strip().lower()
    try:
        if name.endswith(".pdf") or "pdf" in ctype:
            return _extract_text_from_pdf_bytes(data)
        if name.endswith(".docx") or "wordprocessingml" in ctype:
            return _extract_text_from_docx_bytes(data)
        if name.endswith(".xlsx") or name.endswith(".xls") or "spreadsheet" in ctype:
            return _extract_text_from_xlsx_bytes(data)
    except Exception as exc:
        logger.warning("document parser fallback failed: %s", exc)
    return _extract_text_from_plain_bytes(data)


def extract_document_text(document_url: str) -> dict[str, Any]:
    data, content_type = _fetch_document_payload(document_url, timeout_s=45)
    filename = _infer_document_name(document_url, content_type)

    extracted_text = ""
    unstructured_chunks: list[dict[str, Any]] = []
    provider = "fallback"
    try:
        unstructured_result = _extract_via_unstructured(
            data=data,
            filename=filename,
            content_type=content_type,
        )
        if unstructured_result is not None:
            extracted_text, unstructured_chunks = unstructured_result
            provider = "unstructured"
    except Exception as exc:
        logger.warning("unstructured extraction failed: %s", exc)

    if not extracted_text:
        extracted_text = _extract_text_with_fallbacks(
            data=data,
            filename=filename,
            content_type=content_type,
        )

    normalized = str(extracted_text or "").strip()
    chunks = unstructured_chunks or _chunk_text(normalized)
    lines = [ln.strip() for ln in normalized.splitlines() if ln.strip()]
    entities = {
        "action_items": _extract_action_items(lines),
        "dates": _extract_dates(normalized),
        "contacts": _extract_contacts(normalized),
    }
    return {
        "text": normalized[:24000],
        "chunks": chunks[:200],
        "entities": entities,
        "provider": provider,
        "filename": filename,
    }


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
            doc_chunks = doc_info.get("chunks")
            if isinstance(doc_chunks, list) and doc_chunks:
                extracted_entities["chunks"] = doc_chunks[:50]
            provider = str(doc_info.get("provider") or "").strip()
            if provider:
                extracted_entities["parser_provider"] = provider
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
