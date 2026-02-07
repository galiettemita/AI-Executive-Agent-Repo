from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path
from typing import Protocol

from app.core.config import settings


class StorageClient(Protocol):
    def put_bytes(self, key: str, data: bytes, content_type: str | None = None) -> str: ...
    def get_url(self, key: str, expires_seconds: int = 3600) -> str: ...


@dataclass
class LocalStorage:
    base_path: Path

    def put_bytes(self, key: str, data: bytes, content_type: str | None = None) -> str:
        path = self.base_path / key
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)
        return str(path)

    def get_url(self, key: str, expires_seconds: int = 3600) -> str:
        path = self.base_path / key
        return str(path)


@dataclass
class S3Storage:
    bucket: str
    region: str | None = None
    endpoint_url: str | None = None

    def _client(self):
        import boto3

        return boto3.client(
            "s3",
            aws_access_key_id=settings.S3_ACCESS_KEY_ID,
            aws_secret_access_key=settings.S3_SECRET_ACCESS_KEY,
            region_name=self.region,
            endpoint_url=self.endpoint_url,
        )

    def put_bytes(self, key: str, data: bytes, content_type: str | None = None) -> str:
        client = self._client()
        extra = {"ContentType": content_type} if content_type else {}
        client.put_object(Bucket=self.bucket, Key=key, Body=data, **extra)
        return key

    def get_url(self, key: str, expires_seconds: int = 3600) -> str:
        client = self._client()
        return client.generate_presigned_url(
            "get_object",
            Params={"Bucket": self.bucket, "Key": key},
            ExpiresIn=expires_seconds,
        )


def get_storage() -> StorageClient:
    backend = (settings.STORAGE_BACKEND or "local").lower()
    if backend == "s3":
        if not settings.S3_BUCKET:
            raise RuntimeError("S3_BUCKET must be set when STORAGE_BACKEND=s3")
        return S3Storage(
            bucket=settings.S3_BUCKET,
            region=settings.S3_REGION,
            endpoint_url=settings.S3_ENDPOINT_URL,
        )

    base_path = Path(settings.LOCAL_STORAGE_PATH or "./storage")
    return LocalStorage(base_path=base_path)
