# backend/app/services/smart_home/providers/stub.py

from __future__ import annotations


class NotConfiguredProvider:
    def __init__(self, name: str):
        self._name = name

    def _err(self) -> RuntimeError:
        return RuntimeError(f"{self._name} smart home provider not configured yet.")

    def discover_devices(self, user_id: str):
        raise self._err()

    def execute_command(self, user_id: str, device: dict, command: str, params: dict | None = None):
        raise self._err()

    def list_scenes(self, user_id: str):
        raise self._err()

    def activate_scene(self, user_id: str, scene_id: str):
        raise self._err()

    def read_energy(self, user_id: str, entity_ids: list[str]):
        raise self._err()
