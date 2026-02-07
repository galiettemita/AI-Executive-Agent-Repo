# backend/app/services/smart_home/providers/home_assistant.py

from __future__ import annotations

import json
from datetime import datetime
from typing import Any, Dict, List, Optional

import httpx
from sqlalchemy.orm import Session

from app.services.integration_credentials import get_integration_credential, get_decrypted_secret


SUPPORTED_DOMAINS = {
    "light",
    "switch",
    "climate",
    "lock",
    "cover",
    "fan",
    "media_player",
    "vacuum",
    "sensor",
    "binary_sensor",
}


class HomeAssistantProvider:
    def __init__(self, db: Session):
        self._db = db

    def _get_credentials(self, user_id: str) -> tuple[str, str]:
        row = get_integration_credential(self._db, user_id, "home_assistant")
        token = get_decrypted_secret(row) if row else None
        base_url = row.server_url if row else None
        if not base_url or not token:
            raise RuntimeError("Home Assistant not connected. Connect via /admin/smart_home/connect.")
        return base_url.rstrip("/"), token

    def _headers(self, token: str) -> Dict[str, str]:
        return {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}

    def _client(self) -> httpx.Client:
        return httpx.Client(timeout=10.0)

    def _domain(self, entity_id: str) -> str:
        return entity_id.split(".", 1)[0] if entity_id else "unknown"

    def discover_devices(self, user_id: str) -> List[Dict[str, Any]]:
        base_url, token = self._get_credentials(user_id)
        with self._client() as client:
            resp = client.get(f"{base_url}/api/states", headers=self._headers(token))
            if resp.status_code >= 400:
                raise RuntimeError(f"Home Assistant discovery failed: {resp.text}")
            states = resp.json()

        devices = []
        for state in states:
            entity_id = state.get("entity_id", "")
            domain = self._domain(entity_id)
            if domain not in SUPPORTED_DOMAINS:
                continue

            attributes = state.get("attributes") or {}
            devices.append(
                {
                    "provider": "home_assistant",
                    "provider_device_id": entity_id,
                    "name": attributes.get("friendly_name") or entity_id,
                    "device_type": domain,
                    "room": attributes.get("area_id"),
                    "traits": {
                        "domain": domain,
                        "device_class": attributes.get("device_class"),
                        "unit_of_measurement": attributes.get("unit_of_measurement"),
                    },
                    "state": {
                        "state": state.get("state"),
                        "attributes": attributes,
                        "last_changed": state.get("last_changed"),
                        "last_updated": state.get("last_updated"),
                    },
                    "online": state.get("state") != "unavailable",
                }
            )
        return devices

    def execute_command(self, user_id: str, device: dict, command: str, params: dict | None = None) -> dict:
        base_url, token = self._get_credentials(user_id)
        params = params or {}

        entity_id = device.get("provider_device_id")
        if not entity_id:
            raise RuntimeError("Missing provider_device_id for device")

        domain = device.get("device_type") or self._domain(entity_id)
        service_domain = domain
        service = None
        payload = {"entity_id": entity_id}

        if command in {"turn_on", "turn_off"}:
            service = command
        elif command == "set_brightness":
            service_domain = "light"
            service = "turn_on"
            payload["brightness_pct"] = int(params.get("brightness_pct") or params.get("brightness") or 50)
        elif command == "set_color":
            service_domain = "light"
            service = "turn_on"
            if "color_name" in params:
                payload["color_name"] = params["color_name"]
            elif "rgb_color" in params:
                payload["rgb_color"] = params["rgb_color"]
        elif command == "set_temperature":
            service_domain = "climate"
            service = "set_temperature"
            payload["temperature"] = float(params.get("temperature"))
        elif command == "set_mode":
            service_domain = "climate"
            service = "set_hvac_mode"
            payload["hvac_mode"] = params.get("hvac_mode") or params.get("mode")
        elif command == "set_fan_speed":
            service_domain = "fan"
            service = "set_percentage"
            payload["percentage"] = int(params.get("percentage") or 50)
        elif command == "lock":
            service_domain = "lock"
            service = "lock"
        elif command == "unlock":
            service_domain = "lock"
            service = "unlock"
        elif command == "open":
            service_domain = "cover"
            service = "open_cover"
        elif command == "close":
            service_domain = "cover"
            service = "close_cover"
        elif command == "set_position":
            service_domain = "cover"
            service = "set_cover_position"
            payload["position"] = int(params.get("position"))
        elif command == "set_volume":
            service_domain = "media_player"
            service = "volume_set"
            payload["volume_level"] = float(params.get("volume_level"))
        elif command in {"play", "pause", "stop"}:
            service_domain = "media_player"
            service = command
        elif command == "service":
            service_domain = params.get("domain") or domain
            service = params.get("service")
            payload.update(params.get("service_data") or {})

        if not service:
            raise RuntimeError(f"Unsupported command: {command}")

        with self._client() as client:
            resp = client.post(
                f"{base_url}/api/services/{service_domain}/{service}",
                headers=self._headers(token),
                content=json.dumps(payload),
            )
            if resp.status_code >= 400:
                raise RuntimeError(f"Home Assistant command failed: {resp.text}")

        return {"ok": True, "service": f"{service_domain}.{service}", "entity_id": entity_id, "params": payload}

    def list_scenes(self, user_id: str) -> List[Dict[str, Any]]:
        base_url, token = self._get_credentials(user_id)
        with self._client() as client:
            resp = client.get(f"{base_url}/api/scenes", headers=self._headers(token))
            if resp.status_code >= 400:
                raise RuntimeError(f"Home Assistant scenes failed: {resp.text}")
            scenes = resp.json()
        return [
            {
                "id": scene.get("entity_id"),
                "name": scene.get("name") or scene.get("entity_id"),
                "provider": "home_assistant",
            }
            for scene in scenes
        ]

    def activate_scene(self, user_id: str, scene_id: str) -> dict:
        base_url, token = self._get_credentials(user_id)
        payload = {"entity_id": scene_id}
        with self._client() as client:
            resp = client.post(
                f"{base_url}/api/services/scene/turn_on",
                headers=self._headers(token),
                content=json.dumps(payload),
            )
            if resp.status_code >= 400:
                raise RuntimeError(f"Home Assistant scene activate failed: {resp.text}")
        return {"ok": True, "scene_id": scene_id}

    def read_energy(self, user_id: str, entity_ids: List[str]) -> List[Dict[str, Any]]:
        base_url, token = self._get_credentials(user_id)
        readings = []
        with self._client() as client:
            for entity_id in entity_ids:
                resp = client.get(f"{base_url}/api/states/{entity_id}", headers=self._headers(token))
                if resp.status_code >= 400:
                    continue
                state = resp.json()
                attributes = state.get("attributes") or {}
                value_raw = state.get("state")
                try:
                    value = float(value_raw)
                except Exception:
                    value = None
                readings.append(
                    {
                        "entity_id": entity_id,
                        "value": value,
                        "unit": attributes.get("unit_of_measurement"),
                        "raw_state": value_raw,
                        "attributes": attributes,
                        "timestamp": datetime.utcnow().isoformat(),
                    }
                )
        return readings
