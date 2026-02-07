# backend/app/api/routes/smart_home.py

from __future__ import annotations

from typing import Any, Dict, List, Optional

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel, Field
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.services.smart_home_service import (
    discover_devices,
    list_devices,
    get_device,
    execute_device_command,
    create_scene,
    list_scenes,
    activate_scene,
    list_provider_scenes,
    activate_provider_scene,
    create_energy_alert,
    list_energy_alerts,
    get_energy_readings,
)


router = APIRouter(prefix="/smart_home", tags=["smart_home"])


class DiscoverRequest(BaseModel):
    user_id: str
    provider: str = "home_assistant"


class ExecuteRequest(BaseModel):
    user_id: str
    device_id: int
    command: str
    params: Optional[Dict[str, Any]] = None


class SceneAction(BaseModel):
    device_id: int
    command: str
    params: Optional[Dict[str, Any]] = None


class SceneCreateRequest(BaseModel):
    user_id: str
    name: str
    description: Optional[str] = None
    actions: List[SceneAction]


class SceneActivateRequest(BaseModel):
    user_id: str


class ProviderSceneActivateRequest(BaseModel):
    user_id: str
    provider: str = "home_assistant"
    scene_id: str


class EnergyAlertRequest(BaseModel):
    user_id: str
    provider: str = "home_assistant"
    entity_id: str
    comparison: str = "gt"
    threshold_value: float = Field(..., description="Threshold value for alerting")
    unit: Optional[str] = None


@router.post("/devices/discover")
def discover(user: DiscoverRequest, db: Session = Depends(get_db)):
    get_or_create_user(db, user.user_id)
    devices = discover_devices(db, user.user_id, user.provider)
    return {"ok": True, "count": len(devices)}


@router.get("/devices")
def devices(user_id: str, provider: Optional[str] = None, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    items = list_devices(db, user_id, provider=provider)
    return {
        "ok": True,
        "devices": [
            {
                "id": d.id,
                "name": d.name,
                "type": d.device_type,
                "provider": d.provider,
                "online": d.online,
            }
            for d in items
        ],
    }


@router.post("/execute")
def execute(payload: ExecuteRequest, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    device = get_device(db, payload.user_id, payload.device_id)
    if not device:
        raise HTTPException(status_code=404, detail="Device not found")
    response = execute_device_command(db, payload.user_id, device, payload.command, payload.params)
    return {"ok": True, "response": response}


@router.get("/scenes")
def scenes(user_id: str, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    items = list_scenes(db, user_id)
    return {
        "ok": True,
        "scenes": [
            {"id": s.id, "name": s.name, "description": s.description}
            for s in items
        ],
    }


@router.post("/scenes")
def create_scene_route(payload: SceneCreateRequest, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    scene = create_scene(
        db,
        payload.user_id,
        payload.name,
        [a.dict() for a in payload.actions],
        description=payload.description,
    )
    return {"ok": True, "scene_id": scene.id}


@router.post("/scenes/{scene_id}/activate")
def activate_scene_route(scene_id: int, payload: SceneActivateRequest, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    activate_scene(db, payload.user_id, scene_id)
    return {"ok": True}


@router.get("/provider_scenes")
def provider_scenes(user_id: str, provider: str = "home_assistant", db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    scenes = list_provider_scenes(db, user_id, provider)
    return {"ok": True, "scenes": scenes}


@router.post("/provider_scenes/activate")
def activate_provider_scene_route(payload: ProviderSceneActivateRequest, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    activate_provider_scene(db, payload.user_id, payload.provider, payload.scene_id)
    return {"ok": True}


@router.post("/energy/alerts")
def create_energy_alert_route(payload: EnergyAlertRequest, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    alert = create_energy_alert(
        db=db,
        user_id=payload.user_id,
        provider=payload.provider,
        entity_id=payload.entity_id,
        comparison=payload.comparison,
        threshold_value=payload.threshold_value,
        unit=payload.unit,
    )
    return {"ok": True, "alert_id": alert.id}


@router.get("/energy/alerts")
def list_energy_alerts_route(user_id: str, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    alerts = list_energy_alerts(db, user_id)
    return {
        "ok": True,
        "alerts": [
            {
                "id": a.id,
                "provider": a.provider,
                "entity_id": a.entity_id,
                "comparison": a.comparison,
                "threshold_value": a.threshold_value,
                "unit": a.unit,
                "last_triggered_at": a.last_triggered_at.isoformat() if a.last_triggered_at else None,
            }
            for a in alerts
        ],
    }


@router.get("/energy/readings")
def list_energy_readings_route(user_id: str, entity_id: str, limit: int = 20, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    readings = get_energy_readings(db, user_id, entity_id, limit=limit)
    return {
        "ok": True,
        "readings": [
            {
                "value": r.value,
                "unit": r.unit,
                "reading_time": r.reading_time.isoformat() if r.reading_time else None,
            }
            for r in readings
        ],
    }
