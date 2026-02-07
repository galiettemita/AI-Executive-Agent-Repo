# backend/app/services/smart_home_service.py

from __future__ import annotations

import json
from datetime import datetime
from typing import Any, Dict, List, Optional

from sqlalchemy.orm import Session

from app.core.alerting import send_alert
from app.db.models import (
    SmartHomeDevice,
    SmartHomeScene,
    SmartHomeActionLog,
    SmartHomeEnergyAlert,
    SmartHomeEnergyReading,
)
from app.services.smart_home.providers.registry import get_provider
from app.services.consent_service import require_consent


def _serialize(obj: Any) -> str:
    return json.dumps(obj, ensure_ascii=False)


def _deserialize(text: str | None, default: Any) -> Any:
    if not text:
        return default
    try:
        return json.loads(text)
    except Exception:
        return default


def upsert_device(db: Session, user_id: str, payload: Dict[str, Any]) -> SmartHomeDevice:
    provider = payload.get("provider") or "unknown"
    provider_device_id = payload.get("provider_device_id") or ""
    name = payload.get("name") or provider_device_id

    row = (
        db.query(SmartHomeDevice)
        .filter(
            SmartHomeDevice.user_id == user_id,
            SmartHomeDevice.provider == provider,
            SmartHomeDevice.provider_device_id == provider_device_id,
        )
        .one_or_none()
    )
    if row is None:
        row = SmartHomeDevice(
            user_id=user_id,
            provider=provider,
            provider_device_id=provider_device_id,
            name=name,
        )
        db.add(row)

    row.name = name
    row.device_type = payload.get("device_type")
    row.room = payload.get("room")
    row.traits_json = _serialize(payload.get("traits") or {})
    row.state_json = _serialize(payload.get("state") or {})
    row.online = bool(payload.get("online", True))
    row.last_state_at = datetime.utcnow()
    row.last_seen_at = datetime.utcnow()
    row.updated_at = datetime.utcnow()

    db.commit()
    db.refresh(row)
    return row


def discover_devices(db: Session, user_id: str, provider_name: str) -> List[SmartHomeDevice]:
    require_consent(db, user_id, "smart_home")
    provider = get_provider(db, provider_name)
    devices = provider.discover_devices(user_id)
    stored = []
    for d in devices:
        stored.append(upsert_device(db, user_id, d))
    return stored


def list_devices(db: Session, user_id: str, provider: Optional[str] = None) -> List[SmartHomeDevice]:
    q = db.query(SmartHomeDevice).filter(SmartHomeDevice.user_id == user_id)
    if provider:
        q = q.filter(SmartHomeDevice.provider == provider)
    return q.order_by(SmartHomeDevice.name.asc()).all()


def get_device(db: Session, user_id: str, device_id: int) -> Optional[SmartHomeDevice]:
    return (
        db.query(SmartHomeDevice)
        .filter(SmartHomeDevice.user_id == user_id, SmartHomeDevice.id == device_id)
        .one_or_none()
    )


def find_device_by_name(db: Session, user_id: str, name: str) -> Optional[SmartHomeDevice]:
    if not name:
        return None
    q = (
        db.query(SmartHomeDevice)
        .filter(SmartHomeDevice.user_id == user_id)
        .filter(SmartHomeDevice.name.ilike(f"%{name}%"))
        .order_by(SmartHomeDevice.name.asc())
        .all()
    )
    if not q:
        return None
    return q[0]


def log_action(
    db: Session,
    user_id: str,
    action_type: str,
    status: str,
    request: Dict[str, Any],
    response: Optional[Dict[str, Any]] = None,
    device_id: Optional[int] = None,
    scene_id: Optional[int] = None,
    error_message: Optional[str] = None,
) -> None:
    db.add(
        SmartHomeActionLog(
            user_id=user_id,
            device_id=device_id,
            scene_id=scene_id,
            action_type=action_type,
            status=status,
            request_json=_serialize(request),
            response_json=_serialize(response) if response else None,
            error_message=error_message,
            created_at=datetime.utcnow(),
        )
    )
    db.commit()


def execute_device_command(
    db: Session,
    user_id: str,
    device: SmartHomeDevice,
    command: str,
    params: Optional[Dict[str, Any]] = None,
) -> Dict[str, Any]:
    require_consent(db, user_id, "smart_home")
    provider = get_provider(db, device.provider)
    try:
        response = provider.execute_command(
            user_id,
            device={
                "provider": device.provider,
                "provider_device_id": device.provider_device_id,
                "device_type": device.device_type,
            },
            command=command,
            params=params or {},
        )
        log_action(db, user_id, "execute", "ok", {"command": command, "params": params}, response, device_id=device.id)
        return response
    except Exception as e:
        log_action(db, user_id, "execute", "failed", {"command": command, "params": params}, error_message=str(e), device_id=device.id)
        raise


def create_scene(
    db: Session,
    user_id: str,
    name: str,
    actions: List[Dict[str, Any]],
    description: Optional[str] = None,
) -> SmartHomeScene:
    scene = SmartHomeScene(
        user_id=user_id,
        name=name,
        description=description,
        actions_json=_serialize(actions or []),
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(scene)
    db.commit()
    db.refresh(scene)
    return scene


def list_scenes(db: Session, user_id: str) -> List[SmartHomeScene]:
    return (
        db.query(SmartHomeScene)
        .filter(SmartHomeScene.user_id == user_id)
        .order_by(SmartHomeScene.name.asc())
        .all()
    )


def list_provider_scenes(db: Session, user_id: str, provider: str) -> List[Dict[str, Any]]:
    require_consent(db, user_id, "smart_home")
    provider_obj = get_provider(db, provider)
    return provider_obj.list_scenes(user_id)


def activate_scene(
    db: Session,
    user_id: str,
    scene_id: int,
) -> Dict[str, Any]:
    require_consent(db, user_id, "smart_home")
    scene = (
        db.query(SmartHomeScene)
        .filter(SmartHomeScene.user_id == user_id, SmartHomeScene.id == scene_id)
        .one_or_none()
    )
    if not scene:
        raise RuntimeError("Scene not found")

    actions = _deserialize(scene.actions_json, [])
    responses = []
    for action in actions:
        device_id = action.get("device_id")
        command = action.get("command")
        params = action.get("params") or {}
        device = get_device(db, user_id, int(device_id)) if device_id else None
        if not device or not command:
            continue
        responses.append(execute_device_command(db, user_id, device, command, params))

    log_action(db, user_id, "scene", "ok", {"scene_id": scene.id}, {"responses": responses}, scene_id=scene.id)
    return {"ok": True, "scene_id": scene.id, "responses": responses}


def activate_provider_scene(db: Session, user_id: str, provider: str, scene_id: str) -> Dict[str, Any]:
    require_consent(db, user_id, "smart_home")
    provider_obj = get_provider(db, provider)
    try:
        response = provider_obj.activate_scene(user_id, scene_id)
        log_action(db, user_id, "scene", "ok", {"provider": provider, "scene_id": scene_id}, response)
        return response
    except Exception as e:
        log_action(db, user_id, "scene", "failed", {"provider": provider, "scene_id": scene_id}, error_message=str(e))
        raise


def create_energy_alert(
    db: Session,
    user_id: str,
    provider: str,
    entity_id: str,
    comparison: str,
    threshold_value: float,
    unit: Optional[str] = None,
) -> SmartHomeEnergyAlert:
    alert = SmartHomeEnergyAlert(
        user_id=user_id,
        provider=provider,
        entity_id=entity_id,
        comparison=comparison,
        threshold_value=threshold_value,
        unit=unit,
        created_at=datetime.utcnow(),
    )
    db.add(alert)
    db.commit()
    db.refresh(alert)
    return alert


def list_energy_alerts(db: Session, user_id: str) -> List[SmartHomeEnergyAlert]:
    return (
        db.query(SmartHomeEnergyAlert)
        .filter(SmartHomeEnergyAlert.user_id == user_id)
        .order_by(SmartHomeEnergyAlert.created_at.desc())
        .all()
    )


def record_energy_reading(
    db: Session,
    user_id: str,
    provider: str,
    entity_id: str,
    value: float,
    unit: Optional[str],
    metadata: Optional[Dict[str, Any]] = None,
) -> SmartHomeEnergyReading:
    row = SmartHomeEnergyReading(
        user_id=user_id,
        provider=provider,
        entity_id=entity_id,
        value=value,
        unit=unit,
        metadata_json=_serialize(metadata or {}),
        reading_time=datetime.utcnow(),
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def get_energy_readings(db: Session, user_id: str, entity_id: str, limit: int = 50) -> List[SmartHomeEnergyReading]:
    return (
        db.query(SmartHomeEnergyReading)
        .filter(SmartHomeEnergyReading.user_id == user_id, SmartHomeEnergyReading.entity_id == entity_id)
        .order_by(SmartHomeEnergyReading.reading_time.desc())
        .limit(limit)
        .all()
    )


def evaluate_energy_alerts(db: Session, provider_name: str, dry_run: bool = False) -> int:
    alerts = db.query(SmartHomeEnergyAlert).filter(SmartHomeEnergyAlert.provider == provider_name).all()
    if not alerts:
        return 0

    provider = get_provider(db, provider_name)
    triggered = 0

    # Group by user_id for efficient read
    by_user: Dict[str, List[SmartHomeEnergyAlert]] = {}
    for alert in alerts:
        by_user.setdefault(alert.user_id, []).append(alert)

    for user_id, user_alerts in by_user.items():
        entity_ids = [a.entity_id for a in user_alerts]
        try:
            readings = provider.read_energy(user_id, entity_ids)
        except Exception:
            continue
        readings_map = {r.get("entity_id"): r for r in readings}
        for alert in user_alerts:
            reading = readings_map.get(alert.entity_id)
            if not reading:
                continue
            value = reading.get("value")
            if value is None:
                continue
            record_energy_reading(
                db,
                user_id=user_id,
                provider=provider_name,
                entity_id=alert.entity_id,
                value=float(value),
                unit=reading.get("unit"),
                metadata=reading,
            )

            breached = False
            if alert.comparison == "gt" and value > alert.threshold_value:
                breached = True
            if alert.comparison == "lt" and value < alert.threshold_value:
                breached = True

            if breached:
                triggered += 1
                alert.last_triggered_at = datetime.utcnow()
                db.commit()
                if not dry_run:
                    send_alert(
                        f"Smart home energy alert for {user_id}: {alert.entity_id} "
                        f"{alert.comparison} {alert.threshold_value} ({value} {reading.get('unit')})"
                    )
    return triggered
