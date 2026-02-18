from __future__ import annotations

import base64
import json
import logging
from datetime import datetime, timezone
from typing import Any
from urllib.parse import parse_qs, unquote

from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from fastapi import APIRouter, BackgroundTasks, HTTPException, Request

from app.blueprint.contracts import Channel, ContentProvenance, InboundMessage, InputModality
from app.blueprint.db import get_or_create_user_by_channel_identifier
from app.blueprint.session import dedup_inbound
from app.channels.imessage import normalize_imessage_webhook
from app.channels.slack import normalize_slack_event, verify_slack_signature
from app.core.config import settings
from app.core.redis import get_redis
from app.api.routes.v1_gateway import enqueue_inbound_message
from app.db.database import SessionLocal

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/webhook", tags=["phase3-webhooks"])


def _split_pem_chain(raw: str) -> list[str]:
    chain: list[str] = []
    data = str(raw or "").strip()
    if not data:
        return chain
    marker_start = "-----BEGIN CERTIFICATE-----"
    marker_end = "-----END CERTIFICATE-----"
    idx = 0
    while True:
        start = data.find(marker_start, idx)
        if start < 0:
            break
        end = data.find(marker_end, start)
        if end < 0:
            break
        end += len(marker_end)
        chain.append(data[start:end])
        idx = end
    return chain


def _load_cert(value: str) -> x509.Certificate:
    raw = str(value or "").strip()
    if "BEGIN CERTIFICATE" in raw:
        return x509.load_pem_x509_certificate(raw.encode("utf-8"))
    try:
        decoded = base64.b64decode(raw)
        return x509.load_der_x509_certificate(decoded)
    except Exception as exc:
        raise ValueError("Invalid certificate payload") from exc


def _cert_fingerprint_hex(cert: x509.Certificate) -> str:
    return cert.fingerprint(hashes.SHA256()).hex().lower()


def _validate_imessage_cert_chain(request: Request) -> None:
    required = str(settings.IMESSAGE_CERT_VALIDATION_REQUIRED or "0").strip() == "1"
    if not required:
        return

    cert_header = request.headers.get("x-client-cert", "")
    chain_header = request.headers.get("x-client-cert-chain", "")
    if not cert_header:
        raise HTTPException(status_code=403, detail="Missing client certificate header")

    leaf = _load_cert(unquote(cert_header))
    now = datetime.now(timezone.utc)
    not_valid_before = getattr(leaf, "not_valid_before_utc", None) or leaf.not_valid_before.replace(tzinfo=timezone.utc)
    not_valid_after = getattr(leaf, "not_valid_after_utc", None) or leaf.not_valid_after.replace(tzinfo=timezone.utc)
    if not_valid_before > now or not_valid_after < now:
        raise HTTPException(status_code=403, detail="Client certificate expired or not yet valid")

    allowlist = [x.strip().lower() for x in str(settings.IMESSAGE_CERT_SHA256_ALLOWLIST or "").split(",") if x.strip()]
    if allowlist:
        fp = _cert_fingerprint_hex(leaf)
        if fp not in allowlist:
            raise HTTPException(status_code=403, detail="Client certificate fingerprint not allowlisted")

    if chain_header:
        chain = [_load_cert(unquote(item)) for item in _split_pem_chain(unquote(chain_header))]
        if chain:
            current = leaf
            for parent in chain:
                if current.issuer != parent.subject:
                    raise HTTPException(status_code=403, detail="Invalid certificate chain issuer sequence")
                current = parent

    # Optional trusted root pinning.
    root_pem = str(settings.IMESSAGE_TRUSTED_ROOT_PEM or "").strip()
    if root_pem:
        root = x509.load_pem_x509_certificate(root_pem.encode("utf-8"))
        root_pub = root.public_key().public_bytes(
            serialization.Encoding.DER,
            serialization.PublicFormat.SubjectPublicKeyInfo,
        )
        leaf_chain_pub = leaf.public_key().public_bytes(
            serialization.Encoding.DER,
            serialization.PublicFormat.SubjectPublicKeyInfo,
        )
        if root_pub == leaf_chain_pub:
            return
        # If a chain is present we require the final certificate to match the pinned root.
        if chain_header:
            chain = [_load_cert(item) for item in _split_pem_chain(unquote(chain_header))]
            if chain:
                final = chain[-1]
                final_pub = final.public_key().public_bytes(
                    serialization.Encoding.DER,
                    serialization.PublicFormat.SubjectPublicKeyInfo,
                )
                if final_pub != root_pub:
                    raise HTTPException(status_code=403, detail="Certificate chain root does not match pinned root")


@router.post("/imessage")
async def webhook_imessage(request: Request, background_tasks: BackgroundTasks):
    if not settings.FEATURE_IMESSAGE_ENABLED:
        raise HTTPException(status_code=503, detail="iMessage feature disabled")

    _validate_imessage_cert_chain(request)
    payload = await request.json()
    event = normalize_imessage_webhook(payload)
    if not event:
        return {"ok": True, "ignored": True}

    external_id = str(event.get("external_id") or "").strip()
    sender = str(event.get("from") or "").strip()
    if not sender:
        raise HTTPException(status_code=400, detail="Missing sender")

    db = SessionLocal()
    try:
        user = get_or_create_user_by_channel_identifier(db, channel=Channel.IMESSAGE, channel_identifier=sender)
    finally:
        db.close()

    redis_client = get_redis()
    if redis_client and external_id:
        if not dedup_inbound(redis_client, channel=Channel.IMESSAGE, channel_msg_id=external_id):
            return {"ok": True, "deduped": True}

    inbound = InboundMessage(
        channel=Channel.IMESSAGE,
        channel_identifier=sender,
        content=str(event.get("text") or ""),
        input_modality=InputModality.TEXT if not event.get("media_url") else InputModality.IMAGE,
        media_url=event.get("media_url"),
        user_id=user.id,
        conversation_id=f"imessage:{user.id}",
        run_id="",
        channel_msg_id=external_id,
        raw={
            "channel": "imessage",
            "content_provenance": ContentProvenance.USER_DIRECT.value,
            "external_id": external_id,
        },
    )
    run_id = enqueue_inbound_message(background_tasks=background_tasks, inbound=inbound)
    return {"ok": True, "queued": True, "run_id": run_id}


@router.post("/slack")
async def webhook_slack(request: Request, background_tasks: BackgroundTasks):
    raw = await request.body()
    content_type = (request.headers.get("content-type") or "").lower()

    payload: dict[str, Any] = {}
    if "application/json" in content_type:
        payload = json.loads(raw.decode("utf-8")) if raw else {}
    else:
        form = parse_qs(raw.decode("utf-8"))
        if "payload" in form:
            payload = json.loads(form["payload"][0])

    if payload.get("type") == "url_verification":
        return {"challenge": payload.get("challenge")}

    signing_secret = str(settings.SLACK_SIGNING_SECRET or "").strip()
    if settings.ENV in {"staging", "production"} and signing_secret:
        valid = verify_slack_signature(
            raw_body=raw,
            signature=str(request.headers.get("x-slack-signature") or ""),
            request_ts=str(request.headers.get("x-slack-request-timestamp") or ""),
            signing_secret=signing_secret,
        )
        if not valid:
            raise HTTPException(status_code=403, detail="Invalid Slack signature")

    event = normalize_slack_event(payload)
    if not event:
        return {"ok": True, "ignored": True}

    external_id = str(payload.get("event_id") or event.get("external_id") or "").strip()
    sender = str(event.get("from") or "").strip()
    channel_id = str(event.get("channel_id") or "").strip()
    if not sender or not channel_id:
        raise HTTPException(status_code=400, detail="Invalid Slack event payload")

    redis_client = get_redis()
    if redis_client and external_id:
        if not dedup_inbound(redis_client, channel=Channel.SLACK, channel_msg_id=external_id):
            return {"ok": True, "deduped": True}

    inbound = InboundMessage(
        channel=Channel.SLACK,
        channel_identifier=channel_id,
        content=str(event.get("text") or ""),
        input_modality=InputModality.TEXT,
        user_id=sender,
        conversation_id=f"slack:{channel_id}:{sender}",
        run_id="",
        channel_msg_id=external_id,
        raw={
            "channel": "slack",
            "slack_channel_id": channel_id,
            "content_provenance": ContentProvenance.USER_DIRECT.value,
        },
    )
    run_id = enqueue_inbound_message(background_tasks=background_tasks, inbound=inbound)
    return {"ok": True, "queued": True, "run_id": run_id}
