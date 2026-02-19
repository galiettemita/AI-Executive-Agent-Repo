from __future__ import annotations

import io
import zipfile

from app.blueprint import multimodal


def _docx_bytes(text_lines: list[str]) -> bytes:
    xml = (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">'
        "<w:body>"
        + "".join(f"<w:p><w:r><w:t>{line}</w:t></w:r></w:p>" for line in text_lines)
        + "</w:body></w:document>"
    )
    out = io.BytesIO()
    with zipfile.ZipFile(out, mode="w", compression=zipfile.ZIP_DEFLATED) as zf:
        zf.writestr("word/document.xml", xml)
    return out.getvalue()


def test_extract_document_text_docx_fallback(monkeypatch):
    blob = _docx_bytes(
        [
            "Q1 update for board",
            "Action: send revised forecast by Friday",
            "Reach me at ceo@example.com",
        ]
    )
    monkeypatch.setattr(multimodal, "_fetch_document_payload", lambda _url, timeout_s=45: (blob, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"))
    monkeypatch.setattr(multimodal, "_extract_via_unstructured", lambda **kwargs: None)

    extracted = multimodal.extract_document_text("https://example.com/report.docx")

    assert "Q1 update for board" in extracted["text"]
    assert extracted["provider"] == "fallback"
    assert extracted["entities"]["action_items"]
    assert "ceo@example.com" in extracted["entities"]["contacts"]
    assert extracted["chunks"]
