# backend/app/services/smart_home_agent.py

from __future__ import annotations

import json
from typing import Any, Dict, List, Optional

from openai import OpenAI
from sqlalchemy.orm import Session

from app.core.config import settings
from app.services.smart_home_service import (
    list_devices,
    discover_devices,
    find_device_by_name,
    execute_device_command,
    list_scenes,
    create_scene,
    activate_scene,
    list_provider_scenes,
    activate_provider_scene,
    create_energy_alert,
    get_energy_readings,
)


client = OpenAI(api_key=settings.OPENAI_API_KEY)


def _format_devices(devices) -> str:
    if not devices:
        return "No devices discovered."
    lines = []
    for d in devices:
        lines.append(f"- [{d.id}] {d.name} ({d.device_type})")
    return "\n".join(lines)


def _format_scenes(scenes) -> str:
    if not scenes:
        return "No custom scenes."
    lines = []
    for s in scenes:
        lines.append(f"- [{s.id}] {s.name}")
    return "\n".join(lines)


def _default_provider() -> str:
    return getattr(settings, "SMART_HOME_DEFAULT_PROVIDER", None) or "home_assistant"


def run_smart_home_agent(
    db: Session,
    user_id: str,
    history: List[Dict[str, str]],
    user_message: str,
) -> str:
    devices = list_devices(db, user_id)
    scenes = list_scenes(db, user_id)

    system = (
        "You are a smart home assistant. Output ONLY valid JSON.\n"
        "Pick one action:\n"
        "list_devices | discover_devices | control_device | list_scenes | create_scene | activate_scene | "
        "activate_provider_scene | set_energy_alert | energy_status | need_clarification\n"
        "Rules:\n"
        "- control_device: require device_id or device_name, command, optional params.\n"
        "- create_scene: require name and actions. actions is a list of {device_name|device_id, command, params}.\n"
        "- activate_scene: require scene_id.\n"
        "- activate_provider_scene: require provider and scene_id (for provider scenes like Home Assistant).\n"
        "- set_energy_alert: require provider, entity_id, comparison (gt|lt), threshold_value.\n"
        "- energy_status: optional entity_id (if missing, ask for it).\n"
        "- discover_devices: optional provider (default home_assistant).\n"
        "- If missing details, action=need_clarification with a short question.\n"
        "Available devices:\n"
        f"{_format_devices(devices)}\n"
        "Custom scenes:\n"
        f"{_format_scenes(scenes)}\n"
    )

    resp = client.chat.completions.create(
        model=settings.OPENAI_MODEL,
        messages=[
            {"role": "system", "content": system},
            *history[-10:],
            {"role": "user", "content": user_message},
        ],
        temperature=0.1,
    )

    raw = resp.choices[0].message.content or "{}"
    try:
        data = json.loads(raw)
    except Exception:
        return "I had trouble understanding that. Try saying: “Turn off the living room lights.”"

    action = data.get("action")
    if action == "need_clarification":
        return str(data.get("question") or "What would you like me to do?")

    try:
        if action == "list_devices":
            devices = list_devices(db, user_id)
            return _format_devices(devices)

        if action == "discover_devices":
            provider = data.get("provider") or _default_provider()
            devices = discover_devices(db, user_id, provider)
            return f"Discovered {len(devices)} devices.\n{_format_devices(devices)}"

        if action == "control_device":
            device_id = data.get("device_id")
            device_name = data.get("device_name")
            command = data.get("command")
            params = data.get("params") or {}
            if not command:
                return "What command should I run? Example: turn_on, turn_off, set_temperature."
            device = None
            if device_id:
                device = next((d for d in devices if str(d.id) == str(device_id)), None)
            if not device and device_name:
                device = find_device_by_name(db, user_id, str(device_name))
            if not device:
                return "I couldn’t find that device. Try “list my devices” first."
            execute_device_command(db, user_id, device, str(command), params)
            return f"✅ Done. {device.name} → {command}."

        if action == "list_scenes":
            scenes = list_scenes(db, user_id)
            return _format_scenes(scenes)

        if action == "create_scene":
            name = data.get("name")
            actions = data.get("actions") or []
            if not name or not actions:
                return "Tell me the scene name and which devices to control."

            normalized = []
            for act in actions:
                device_id = act.get("device_id")
                device_name = act.get("device_name")
                command = act.get("command")
                params = act.get("params") or {}
                device = None
                if device_id:
                    device = next((d for d in devices if str(d.id) == str(device_id)), None)
                if not device and device_name:
                    device = find_device_by_name(db, user_id, str(device_name))
                if not device or not command:
                    continue
                normalized.append(
                    {"device_id": device.id, "command": command, "params": params}
                )
            scene = create_scene(db, user_id, str(name), normalized, description=data.get("description"))
            return f"Scene '{scene.name}' created."

        if action == "activate_scene":
            scene_id = data.get("scene_id")
            if not scene_id:
                return "Which scene should I activate?"
            activate_scene(db, user_id, int(scene_id))
            return "Scene activated."

        if action == "activate_provider_scene":
            provider = data.get("provider") or _default_provider()
            scene_id = data.get("scene_id")
            if not scene_id:
                return "Which provider scene should I activate?"
            activate_provider_scene(db, user_id, provider, str(scene_id))
            return "Scene activated."

        if action == "set_energy_alert":
            provider = data.get("provider") or _default_provider()
            entity_id = data.get("entity_id")
            comparison = data.get("comparison") or "gt"
            threshold_value = data.get("threshold_value")
            if not entity_id or threshold_value is None:
                return "Tell me the energy sensor entity_id and threshold."
            create_energy_alert(
                db,
                user_id=user_id,
                provider=provider,
                entity_id=str(entity_id),
                comparison=str(comparison),
                threshold_value=float(threshold_value),
                unit=data.get("unit"),
            )
            return "Energy alert set."

        if action == "energy_status":
            entity_id = data.get("entity_id")
            if not entity_id:
                return "Which energy sensor should I check? Provide the entity_id."
            readings = get_energy_readings(db, user_id, str(entity_id), limit=1)
            if readings:
                r = readings[0]
                return f"{entity_id}: {r.value} {r.unit or ''}".strip()
            return "No recent readings yet."

        return "I can help control devices, manage scenes, or set energy alerts."

    except Exception as e:
        return f"Smart home error: {str(e)}"
