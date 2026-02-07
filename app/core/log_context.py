from __future__ import annotations

from contextvars import ContextVar

request_id_var: ContextVar[str] = ContextVar("request_id", default="")
user_id_var: ContextVar[str] = ContextVar("user_id", default="")


def set_request_id(request_id: str) -> None:
    request_id_var.set(request_id or "")


def set_user_id(user_id: str) -> None:
    user_id_var.set(user_id or "")


def get_request_id() -> str:
    return request_id_var.get()


def get_user_id() -> str:
    return user_id_var.get()


def clear_context() -> None:
    request_id_var.set("")
    user_id_var.set("")
