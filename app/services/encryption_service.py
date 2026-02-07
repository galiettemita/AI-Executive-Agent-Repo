# app/services/encryption_service.py
"""
PII Encryption Service

Provides encryption/decryption for sensitive user data including:
- Passport numbers
- Date of birth
- Phone numbers
- Email addresses
- Known traveler numbers (TSA PreCheck)
- Redress numbers

Uses Fernet symmetric encryption (AES-128-CBC with HMAC-SHA256).
"""

from __future__ import annotations

import base64
import hashlib
from functools import lru_cache
from typing import Optional

from cryptography.fernet import Fernet, InvalidToken
from app.core.config import settings


class EncryptionService:
    """Handles encryption and decryption of PII data."""

    def __init__(self, key: Optional[str] = None):
        """
        Initialize encryption service.

        Args:
            key: Optional Fernet key. If not provided, uses PII_ENCRYPTION_KEY env var
                 or derives from JWT_SECRET.
        """
        self._primary_fernet, self._all_fernets = self._get_fernets(key)

    def _get_fernets(self, key: Optional[str] = None) -> tuple[Fernet, list[Fernet]]:
        """
        Get primary and fallback Fernet instances.

        Supports key rotation via PII_ENCRYPTION_KEYS (comma-separated).
        The first key is used for encryption; all keys are tried for decryption.
        """
        if key:
            f = Fernet(key.encode("utf-8"))
            return f, [f]

        keys: list[str] = []

        # Rotation keys (comma-separated)
        if settings.PII_ENCRYPTION_KEYS:
            for raw in settings.PII_ENCRYPTION_KEYS.split(","):
                candidate = raw.strip()
                if candidate:
                    keys.append(candidate)

        # Primary key (preferred)
        if settings.PII_ENCRYPTION_KEY:
            if settings.PII_ENCRYPTION_KEY not in keys:
                keys.insert(0, settings.PII_ENCRYPTION_KEY)

        # Fall back to deriving from JWT_SECRET if no keys
        if not keys:
            jwt_secret = settings.JWT_SECRET
            if not jwt_secret:
                raise RuntimeError(
                    "Missing encryption key. Set PII_ENCRYPTION_KEY (preferred) or JWT_SECRET env var."
                )
            digest = hashlib.sha256(jwt_secret.encode("utf-8")).digest()
            keys = [base64.urlsafe_b64encode(digest).decode("utf-8")]

        fernets = [Fernet(k.encode("utf-8")) for k in keys]
        return fernets[0], fernets

    def encrypt(self, plaintext: Optional[str]) -> Optional[str]:
        """
        Encrypt a plaintext string.

        Args:
            plaintext: The string to encrypt.

        Returns:
            Base64-encoded encrypted string, or None if input is None/empty.
        """
        if not plaintext:
            return None

        encrypted = self._primary_fernet.encrypt(plaintext.encode("utf-8"))
        return encrypted.decode("utf-8")

    def decrypt(self, ciphertext: Optional[str]) -> Optional[str]:
        """
        Decrypt an encrypted string.

        Args:
            ciphertext: The encrypted string to decrypt.

        Returns:
            Decrypted plaintext string, or None if input is None/empty.

        Raises:
            ValueError: If decryption fails (invalid token or key mismatch).
        """
        if not ciphertext:
            return None

        for f in self._all_fernets:
            try:
                decrypted = f.decrypt(ciphertext.encode("utf-8"))
                return decrypted.decode("utf-8")
            except InvalidToken:
                continue
        raise ValueError(
            "Failed to decrypt data. PII_ENCRYPTION_KEY may have changed."
        )

    def is_encrypted(self, value: Optional[str]) -> bool:
        """
        Check if a value appears to be encrypted.

        Fernet tokens start with 'gAAAAA' (base64-encoded version byte).
        """
        if not value:
            return False
        return value.startswith("gAAAAA")


@lru_cache(maxsize=1)
def get_encryption_service() -> EncryptionService:
    """Get singleton encryption service instance."""
    return EncryptionService()


# Convenience functions for direct use
def encrypt_pii(plaintext: Optional[str]) -> Optional[str]:
    """Encrypt PII data."""
    return get_encryption_service().encrypt(plaintext)


def decrypt_pii(ciphertext: Optional[str]) -> Optional[str]:
    """Decrypt PII data."""
    return get_encryption_service().decrypt(ciphertext)


def is_encrypted(value: Optional[str]) -> bool:
    """Check if value is encrypted."""
    return get_encryption_service().is_encrypted(value)


# Field names that should be encrypted in TravelerProfile
ENCRYPTED_FIELDS = {
    "passport_number",
    "date_of_birth",
    "phone",
    "email",
    "known_traveler_number",
    "redress_number",
}


def encrypt_traveler_data(data: dict) -> dict:
    """
    Encrypt sensitive fields in traveler profile data.

    Args:
        data: Dictionary containing traveler profile fields.

    Returns:
        Dictionary with sensitive fields encrypted.
    """
    result = data.copy()
    service = get_encryption_service()

    for field in ENCRYPTED_FIELDS:
        if field in result and result[field]:
            # Don't re-encrypt if already encrypted
            if not service.is_encrypted(result[field]):
                result[field] = service.encrypt(result[field])

    return result


def decrypt_traveler_data(data: dict) -> dict:
    """
    Decrypt sensitive fields in traveler profile data.

    Args:
        data: Dictionary containing traveler profile fields.

    Returns:
        Dictionary with sensitive fields decrypted.
    """
    result = data.copy()
    service = get_encryption_service()

    for field in ENCRYPTED_FIELDS:
        if field in result and result[field]:
            # Only decrypt if encrypted
            if service.is_encrypted(result[field]):
                result[field] = service.decrypt(result[field])

    return result


def generate_encryption_key() -> str:
    """
    Generate a new Fernet encryption key.

    Returns:
        A base64-encoded 32-byte key suitable for PII_ENCRYPTION_KEY.
    """
    return Fernet.generate_key().decode("utf-8")
