from __future__ import annotations

from pathlib import Path

from fastapi.testclient import TestClient

from app.core.config import settings
from app.main import app
from app.services.document_generation import generate_document


def test_generate_document_service_pdf_and_docx(tmp_path, monkeypatch):
    monkeypatch.setattr(settings, "STORAGE_BACKEND", "local")
    monkeypatch.setattr(settings, "LOCAL_STORAGE_PATH", str(tmp_path))

    pdf = generate_document(
        user_id="doc-user",
        title="Q4 Plan",
        markdown="## Goals\n- Launch pilot\n- Review retention",
        output_format="pdf",
        template="report",
    )
    docx = generate_document(
        user_id="doc-user",
        title="Q4 Plan",
        markdown="## Goals\n- Launch pilot\n- Review retention",
        output_format="docx",
        template="memo",
    )

    assert pdf["output_format"] == "pdf"
    assert docx["output_format"] == "docx"
    assert pdf["size_bytes"] > 0
    assert docx["size_bytes"] > 0
    assert Path(pdf["storage_key"]).exists()
    assert Path(docx["storage_key"]).exists()


def test_document_generate_endpoint(tmp_path, monkeypatch):
    monkeypatch.setattr(settings, "FEATURE_DOCUMENT_GENERATION", True)
    monkeypatch.setattr(settings, "STORAGE_BACKEND", "local")
    monkeypatch.setattr(settings, "LOCAL_STORAGE_PATH", str(tmp_path))

    client = TestClient(app)
    resp = client.post(
        "/api/v1/documents/generate",
        json={
            "user_id": "doc-endpoint-user",
            "title": "Weekly Brief",
            "markdown": "### Priorities\n- Client update\n- Team sync",
            "output_format": "pdf",
            "template": "brief",
            "metadata": {"team": "ops"},
        },
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["ok"] is True
    assert body["document"]["output_format"] == "pdf"
    assert Path(body["document"]["storage_key"]).exists()
