from __future__ import annotations

import asyncio
import json
import os
import time
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Awaitable, Callable, Protocol
from urllib.parse import urlparse

import httpx

from app.blueprint.mcp.contracts import MCPTransportConfig, MCPTransportType
from app.core.config import settings


class MCPTransportError(RuntimeError):
    pass


class MCPServerError(MCPTransportError):
    pass


def _csv_values(raw: str | None) -> set[str]:
    return {item.strip().lower() for item in str(raw or "").split(",") if item.strip()}


def _validate_network_allowlist(url: str) -> None:
    allowlist = _csv_values(settings.MCP_NETWORK_ALLOWLIST)
    if not allowlist:
        return
    host = (urlparse(url).hostname or "").lower().strip()
    if not host:
        raise MCPTransportError("MCP transport URL has no hostname")
    if host in allowlist:
        return
    if any(host.endswith("." + suffix) for suffix in allowlist):
        return
    raise MCPTransportError(f"MCP transport host '{host}' not in MCP_NETWORK_ALLOWLIST")


def _validate_stdio_allowlist(command: str) -> None:
    allowlist = _csv_values(settings.MCP_STDIO_ALLOWED_COMMANDS)
    if not allowlist:
        return
    name = Path(command).name.lower()
    if name not in allowlist:
        raise MCPTransportError(f"MCP stdio command '{name}' not in MCP_STDIO_ALLOWED_COMMANDS")


class MCPTransport(Protocol):
    async def connect(self) -> None: ...

    async def send_request(self, method: str, params: dict[str, Any]) -> dict[str, Any]: ...

    async def send_notification(self, method: str, params: dict[str, Any]) -> None: ...

    async def close(self) -> None: ...

    def on_notification(self, handler: Callable[[dict[str, Any]], Awaitable[None] | None]) -> None: ...

    @property
    def is_connected(self) -> bool: ...


@dataclass
class JsonRpcRequest:
    method: str
    params: dict[str, Any]
    request_id: str

    def to_payload(self) -> dict[str, Any]:
        return {
            "jsonrpc": "2.0",
            "id": self.request_id,
            "method": self.method,
            "params": self.params,
        }


class StreamableHTTPTransport:
    def __init__(self, config: MCPTransportConfig):
        if not config.url:
            raise MCPTransportError("streamable_http transport requires url")
        _validate_network_allowlist(config.url)
        self._url = config.url
        self._headers = dict(config.headers or {})
        self._timeout = max(1, int(config.timeout_ms)) / 1000
        self._session_id: str | None = None
        self._client: httpx.AsyncClient | None = None
        self._notification_handler: Callable[[dict[str, Any]], Awaitable[None] | None] | None = None

    async def connect(self) -> None:
        if self._client is None:
            self._client = httpx.AsyncClient(
                timeout=self._timeout,
                limits=httpx.Limits(max_connections=20, max_keepalive_connections=5),
            )

    @property
    def is_connected(self) -> bool:
        return self._client is not None

    def on_notification(self, handler: Callable[[dict[str, Any]], Awaitable[None] | None]) -> None:
        self._notification_handler = handler

    async def send_request(self, method: str, params: dict[str, Any]) -> dict[str, Any]:
        await self.connect()
        assert self._client is not None

        body = JsonRpcRequest(method=method, params=params, request_id=str(uuid.uuid4())).to_payload()
        headers = {"Content-Type": "application/json", **self._headers}
        if self._session_id:
            headers["Mcp-Session-Id"] = self._session_id

        response = await self._client.post(self._url, json=body, headers=headers)
        if "Mcp-Session-Id" in response.headers:
            self._session_id = response.headers.get("Mcp-Session-Id")

        if response.status_code >= 400:
            raise MCPServerError(f"HTTP {response.status_code}: {response.text[:300]}")

        payload = response.json()
        if "error" in payload:
            err = payload.get("error") or {}
            raise MCPServerError(str(err))
        return payload.get("result") or {}

    async def send_notification(self, method: str, params: dict[str, Any]) -> None:
        await self.connect()
        assert self._client is not None

        body = {
            "jsonrpc": "2.0",
            "method": method,
            "params": params,
        }
        headers = {"Content-Type": "application/json", **self._headers}
        if self._session_id:
            headers["Mcp-Session-Id"] = self._session_id
        await self._client.post(self._url, json=body, headers=headers)

    async def close(self) -> None:
        if self._client is not None:
            await self._client.aclose()
        self._client = None
        self._session_id = None


class StdioTransport:
    def __init__(self, config: MCPTransportConfig):
        if not config.command:
            raise MCPTransportError("stdio transport requires command")
        _validate_stdio_allowlist(config.command)
        self._command = config.command
        self._args = list(config.args or [])
        self._env = dict(config.env or {})
        self._timeout_ms = max(1000, int(config.timeout_ms))
        self._process: asyncio.subprocess.Process | None = None
        self._notification_handler: Callable[[dict[str, Any]], Awaitable[None] | None] | None = None
        self._stderr_task: asyncio.Task | None = None

    @property
    def is_connected(self) -> bool:
        return bool(self._process and self._process.returncode is None)

    def on_notification(self, handler: Callable[[dict[str, Any]], Awaitable[None] | None]) -> None:
        self._notification_handler = handler

    async def connect(self) -> None:
        if self.is_connected:
            return

        env = os.environ.copy()
        env.update({k: str(v) for k, v in self._env.items()})
        self._process = await asyncio.create_subprocess_exec(
            self._command,
            *self._args,
            stdin=asyncio.subprocess.PIPE,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
            env=env,
        )
        self._stderr_task = asyncio.create_task(self._consume_stderr())

    async def _consume_stderr(self) -> None:
        if not self._process or not self._process.stderr:
            return
        try:
            while True:
                line = await self._process.stderr.readline()
                if not line:
                    break
        except Exception:
            return

    async def send_request(self, method: str, params: dict[str, Any]) -> dict[str, Any]:
        await self.connect()
        assert self._process is not None and self._process.stdin is not None and self._process.stdout is not None

        req_id = str(uuid.uuid4())
        payload = JsonRpcRequest(method=method, params=params, request_id=req_id).to_payload()
        self._process.stdin.write((json.dumps(payload, ensure_ascii=False) + "\n").encode("utf-8"))
        await self._process.stdin.drain()

        deadline = time.monotonic() + (self._timeout_ms / 1000)
        while True:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise MCPTransportError(f"stdio timeout waiting for {method}")
            raw = await asyncio.wait_for(self._process.stdout.readline(), timeout=remaining)
            if not raw:
                raise MCPTransportError("stdio process closed stdout")
            msg = json.loads(raw.decode("utf-8"))

            if "id" not in msg and self._notification_handler:
                maybe = self._notification_handler(msg)
                if asyncio.iscoroutine(maybe):
                    await maybe
                continue

            if str(msg.get("id") or "") != req_id:
                continue

            if "error" in msg:
                raise MCPServerError(str(msg.get("error") or {}))
            return msg.get("result") or {}

    async def send_notification(self, method: str, params: dict[str, Any]) -> None:
        await self.connect()
        assert self._process is not None and self._process.stdin is not None
        payload = {
            "jsonrpc": "2.0",
            "method": method,
            "params": params,
        }
        self._process.stdin.write((json.dumps(payload, ensure_ascii=False) + "\n").encode("utf-8"))
        await self._process.stdin.drain()

    async def close(self) -> None:
        if self._process is not None and self._process.returncode is None:
            try:
                self._process.terminate()
                await asyncio.wait_for(self._process.wait(), timeout=1.0)
            except Exception:
                try:
                    self._process.kill()
                except Exception:
                    pass
        if self._stderr_task and not self._stderr_task.done():
            self._stderr_task.cancel()
        self._process = None


class SSETransport(StreamableHTTPTransport):
    """
    Compatibility transport for legacy MCP servers that still expose SSE-style endpoints.
    We treat this as streamable HTTP for request/response and keep transport type explicit.
    """


def create_transport(config: MCPTransportConfig) -> MCPTransport:
    if config.type == MCPTransportType.STREAMABLE_HTTP:
        return StreamableHTTPTransport(config)
    if config.type == MCPTransportType.STDIO:
        return StdioTransport(config)
    if config.type == MCPTransportType.SSE:
        return SSETransport(config)
    raise MCPTransportError(f"Unsupported transport type: {config.type}")
