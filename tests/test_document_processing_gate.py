from __future__ import annotations

from app.blueprint import multimodal
from app.blueprint.contracts import ContentProvenance, InboundMessage, InputModality
from app.core.config import settings


def _doc_message() -> InboundMessage:
    return InboundMessage(
        channel="web",
        channel_identifier="doc-user",
        content="Please summarize this file",
        input_modality=InputModality.DOCUMENT,
        media_url="https://example.com/fake.pdf",
        user_id="doc-user",
    )


def test_document_ingestion_enabled_extracts_text_and_entities(monkeypatch) -> None:
    monkeypatch.setattr(settings, "FEATURE_DOCUMENT_PROCESSING", True)
    monkeypatch.setattr(
        multimodal,
        "extract_document_text",
        lambda _url: {
            "text": "Quarterly revenue grew 18%. Action: review runway.",
            "entities": {"action_items": ["Action: review runway."]},
        },
    )

    processed = multimodal.preprocess_inbound_message(_doc_message())

    assert processed.content_provenance == ContentProvenance.DOCUMENT
    assert "[Document text]" in processed.normalized_text
    assert "Quarterly revenue grew 18%" in processed.normalized_text
    assert processed.extracted_entities.get("action_items") == ["Action: review runway."]


def test_document_ingestion_disabled_skips_document_fetch(monkeypatch) -> None:
    monkeypatch.setattr(settings, "FEATURE_DOCUMENT_PROCESSING", False)

    def _should_not_run(_url: str):
        raise AssertionError("document extraction should be skipped when flag is off")

    monkeypatch.setattr(multimodal, "extract_document_text", _should_not_run)

    processed = multimodal.preprocess_inbound_message(_doc_message())

    assert processed.content_provenance == ContentProvenance.USER_DIRECT
    assert "[Document text]" not in processed.normalized_text
    assert processed.normalized_text.startswith("Please summarize this file")
