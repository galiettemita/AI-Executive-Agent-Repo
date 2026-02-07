# tests/test_smart_home_service.py

import uuid

from app.db.database import SessionLocal
from app.services.smart_home.providers.registry import register_provider
from app.services.smart_home_service import (
    discover_devices,
    list_devices,
    execute_device_command,
    create_scene,
    activate_scene,
    create_energy_alert,
    evaluate_energy_alerts,
)


class MockProvider:
    def discover_devices(self, user_id: str):
        return [
            {
                "provider": "mock",
                "provider_device_id": "light.living_room",
                "name": "Living Room Light",
                "device_type": "light",
                "traits": {"domain": "light"},
                "state": {"state": "off"},
                "online": True,
            }
        ]

    def execute_command(self, user_id: str, device: dict, command: str, params: dict | None = None):
        return {"ok": True, "command": command}

    def list_scenes(self, user_id: str):
        return []

    def activate_scene(self, user_id: str, scene_id: str):
        return {"ok": True}

    def read_energy(self, user_id: str, entity_ids: list[str]):
        return [
            {"entity_id": entity_ids[0], "value": 120.0, "unit": "W"}
        ]


def test_smart_home_device_flow():
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"

    register_provider("mock", MockProvider())
    devices = discover_devices(db, user_id, "mock")
    assert len(devices) == 1

    stored = list_devices(db, user_id, provider="mock")
    assert stored[0].name == "Living Room Light"

    resp = execute_device_command(db, user_id, stored[0], "turn_on", {})
    assert resp["ok"] is True
    db.close()


def test_smart_home_scene_and_energy():
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"

    register_provider("mock", MockProvider())
    devices = discover_devices(db, user_id, "mock")
    device = devices[0]

    scene = create_scene(
        db,
        user_id=user_id,
        name="Movie Night",
        actions=[{"device_id": device.id, "command": "turn_off", "params": {}}],
    )
    result = activate_scene(db, user_id, scene.id)
    assert result["ok"] is True

    alert = create_energy_alert(
        db,
        user_id=user_id,
        provider="mock",
        entity_id="sensor.energy",
        comparison="gt",
        threshold_value=100,
        unit="W",
    )
    assert alert.id is not None

    triggered = evaluate_energy_alerts(db, provider_name="mock", dry_run=True)
    assert triggered >= 1
    db.close()
