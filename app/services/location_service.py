from __future__ import annotations

import logging
from datetime import datetime
from typing import Any, Dict, Optional

import httpx

logger = logging.getLogger(__name__)


def _coalesce_location(city: Optional[str], region: Optional[str], country: Optional[str]) -> Optional[str]:
    parts = [p for p in [city, region, country] if p]
    if not parts:
        return None
    return ", ".join(parts)


def build_location_patch(
    *,
    source: str,
    latitude: Optional[float] = None,
    longitude: Optional[float] = None,
    accuracy_m: Optional[float] = None,
    city: Optional[str] = None,
    region: Optional[str] = None,
    country: Optional[str] = None,
    timezone: Optional[str] = None,
    ip: Optional[str] = None,
    location_label: Optional[str] = None,
) -> Dict[str, Any]:
    patch: Dict[str, Any] = {
        "location_share_opt_in": True,
        "location_source": source,
        "location_updated_at": datetime.utcnow().isoformat(),
    }

    if latitude is not None and longitude is not None:
        patch["home_lat"] = float(latitude)
        patch["home_lon"] = float(longitude)
    if accuracy_m is not None:
        patch["location_accuracy_m"] = float(accuracy_m)
    if city:
        patch["home_city"] = city
    if region:
        patch["home_region"] = region
    if country:
        patch["home_country"] = country
    if timezone:
        patch["timezone"] = timezone
    if ip:
        patch["location_ip"] = ip

    if location_label:
        patch["location"] = location_label
    else:
        derived_label = _coalesce_location(city, region, country)
        if derived_label:
            patch["location"] = derived_label

    return patch


async def resolve_ip_location(ip: Optional[str]) -> Dict[str, Any]:
    """
    Resolve an IP address to approximate location using ipapi.co.
    Returns a dict with keys: ip, city, region, country, latitude, longitude, timezone.
    """
    if not ip:
        return {"ip": None}

    url = f"https://ipapi.co/{ip}/json/"
    try:
        async with httpx.AsyncClient(timeout=5.0) as client:
            resp = await client.get(url)
            if resp.status_code != 200:
                logger.warning("ipapi lookup failed (status=%s)", resp.status_code)
                return {"ip": ip}
            data = resp.json() if resp.content else {}
    except Exception as exc:
        logger.warning("ipapi lookup error: %s", exc)
        return {"ip": ip}

    return {
        "ip": ip,
        "city": data.get("city"),
        "region": data.get("region"),
        "country": data.get("country_name") or data.get("country"),
        "latitude": data.get("latitude"),
        "longitude": data.get("longitude"),
        "timezone": data.get("timezone"),
    }
