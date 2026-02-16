from __future__ import annotations

import base64
import hashlib
import os
from typing import Iterable

from cryptography.fernet import Fernet, InvalidToken
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

from app.core.config import settings


def _material_candidates() -> list[str]:
    out: list[str] = []
    if settings.TOKEN_ENCRYPTION_KEY:
        out.append(settings.TOKEN_ENCRYPTION_KEY)
    if settings.PII_ENCRYPTION_KEYS:
        out.extend([k.strip() for k in settings.PII_ENCRYPTION_KEYS.split(",") if k.strip()])
    if settings.PII_ENCRYPTION_KEY:
        out.append(settings.PII_ENCRYPTION_KEY)
    if settings.JWT_SECRET:
        out.append(settings.JWT_SECRET)
    if not out:
        env_secret = os.getenv("STATE_SIGNING_SECRET", "")
        if env_secret:
            out.append(env_secret)

    deduped: list[str] = []
    seen: set[str] = set()
    for item in out:
        if item and item not in seen:
            seen.add(item)
            deduped.append(item)
    return deduped


def _derive_aes_key(material: str) -> bytes:
    # Accept urlsafe-base64 32-byte keys when available; otherwise derive with SHA-256.
    raw = material.encode("utf-8")
    try:
        decoded = base64.urlsafe_b64decode(raw)
        if len(decoded) == 32:
            return decoded
    except Exception:
        pass
    return hashlib.sha256(raw).digest()


def _aes_keys() -> list[bytes]:
    return [_derive_aes_key(m) for m in _material_candidates()]


def _legacy_fernet_keys() -> Iterable[Fernet]:
    for material in _material_candidates():
        digest = hashlib.sha256(material.encode("utf-8")).digest()
        yield Fernet(base64.urlsafe_b64encode(digest))


def encrypt_token(plaintext: str) -> str:
    """
    AES-256-GCM encryption for OAuth/connectors token storage with key-rotation support.
    """
    value = plaintext or ""
    keys = _aes_keys()
    if not keys:
        raise RuntimeError("No encryption key material configured")
    nonce = os.urandom(12)
    cipher = AESGCM(keys[0]).encrypt(nonce, value.encode("utf-8"), None)
    payload = base64.urlsafe_b64encode(nonce + cipher).decode("utf-8")
    return f"v2:{payload}"


def decrypt_token(ciphertext: str) -> str:
    if not ciphertext:
        return ""

    if ciphertext.startswith("v2:"):
        payload = ciphertext.split(":", 1)[1]
        blob = base64.urlsafe_b64decode(payload.encode("utf-8"))
        nonce, cipher = blob[:12], blob[12:]
        for key in _aes_keys():
            try:
                plain = AESGCM(key).decrypt(nonce, cipher, None)
                return plain.decode("utf-8")
            except Exception:
                continue
        raise RuntimeError("Could not decrypt token with configured AES-GCM keys")

    # Backward-compatibility for existing Fernet-encrypted tokens.
    for f in _legacy_fernet_keys():
        try:
            return f.decrypt(ciphertext.encode("utf-8")).decode("utf-8")
        except InvalidToken:
            continue
        except Exception:
            continue
    raise RuntimeError("Could not decrypt legacy token")
