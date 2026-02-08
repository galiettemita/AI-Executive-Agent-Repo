# app/services/weather_service.py

from __future__ import annotations

from datetime import date, datetime
from typing import Any, Dict, Optional, Tuple

import httpx

from app.core.config import settings


class WeatherError(RuntimeError):
    pass


def _parse_lat_lon(value: str) -> Optional[Tuple[float, float]]:
    if not value:
        return None
    parts = [p.strip() for p in value.split(",")]
    if len(parts) != 2:
        return None
    try:
        return float(parts[0]), float(parts[1])
    except ValueError:
        return None


def resolve_location(profile: Dict[str, Any], override_location: Optional[str] = None) -> Dict[str, Any]:
    """
    Resolve user location for weather queries.
    Priority:
    1) override_location param (lat,lon or city string)
    2) profile["home_lat"] + profile["home_lon"]
    3) profile["home_city"] or profile["location"]
    4) settings.WEATHER_DEFAULT_LOCATION
    """
    if override_location:
        latlon = _parse_lat_lon(override_location)
        if latlon:
            return {"name": override_location, "latitude": latlon[0], "longitude": latlon[1]}
        return _geocode_location(override_location)

    if profile:
        lat = profile.get("home_lat")
        lon = profile.get("home_lon")
        if lat is not None and lon is not None:
            try:
                return {"name": profile.get("home_city") or "home", "latitude": float(lat), "longitude": float(lon)}
            except Exception:
                pass
        name = profile.get("home_city") or profile.get("location")
        if name:
            return _geocode_location(str(name))

    if settings.WEATHER_DEFAULT_LOCATION:
        return _geocode_location(settings.WEATHER_DEFAULT_LOCATION)

    raise WeatherError("No location available. Set profile home_city/home_lat/home_lon or pass location.")


def _geocode_location(location: str) -> Dict[str, Any]:
    if settings.WEATHER_PROVIDER != "open_meteo":
        raise WeatherError("Geocoding only supported for open_meteo provider")

    params = {"name": location, "count": 1, "language": "en", "format": "json"}
    url = "https://geocoding-api.open-meteo.com/v1/search"
    try:
        resp = httpx.get(url, params=params, timeout=10)
        resp.raise_for_status()
        data = resp.json()
    except Exception as exc:
        raise WeatherError(f"Failed to geocode location: {exc}")

    results = data.get("results") or []
    if not results:
        raise WeatherError("Location not found")
    best = results[0]
    return {
        "name": best.get("name") or location,
        "latitude": best.get("latitude"),
        "longitude": best.get("longitude"),
        "timezone": best.get("timezone"),
        "country": best.get("country"),
    }


def _c_to_f(c: Optional[float]) -> Optional[float]:
    if c is None:
        return None
    return (c * 9 / 5) + 32


def get_daily_weather(
    location: Dict[str, Any],
    forecast_date: date,
) -> Dict[str, Any]:
    if settings.WEATHER_PROVIDER != "open_meteo":
        raise WeatherError("Only open_meteo provider is implemented")

    lat = location.get("latitude")
    lon = location.get("longitude")
    if lat is None or lon is None:
        raise WeatherError("Location missing latitude/longitude")

    params = {
        "latitude": lat,
        "longitude": lon,
        "daily": "temperature_2m_max,temperature_2m_min,precipitation_sum,weathercode",
        "timezone": "auto",
        "start_date": forecast_date.isoformat(),
        "end_date": forecast_date.isoformat(),
    }
    url = "https://api.open-meteo.com/v1/forecast"

    try:
        resp = httpx.get(url, params=params, timeout=10)
        resp.raise_for_status()
        data = resp.json()
    except Exception as exc:
        raise WeatherError(f"Failed to fetch weather: {exc}")

    daily = data.get("daily") or {}
    dates = daily.get("time") or []
    if not dates:
        raise WeatherError("Weather response missing daily data")

    idx = 0
    if forecast_date.isoformat() in dates:
        idx = dates.index(forecast_date.isoformat())

    temp_max = _safe_float(daily.get("temperature_2m_max"), idx)
    temp_min = _safe_float(daily.get("temperature_2m_min"), idx)
    precip = _safe_float(daily.get("precipitation_sum"), idx)
    code = _safe_float(daily.get("weathercode"), idx)

    return {
        "provider": settings.WEATHER_PROVIDER,
        "location": {
            "name": location.get("name"),
            "latitude": lat,
            "longitude": lon,
            "timezone": data.get("timezone") or location.get("timezone"),
            "country": location.get("country"),
        },
        "date": forecast_date.isoformat(),
        "temp_c_min": temp_min,
        "temp_c_max": temp_max,
        "temp_f_min": _c_to_f(temp_min),
        "temp_f_max": _c_to_f(temp_max),
        "precip_mm": precip,
        "weather_code": code,
    }


def _safe_float(values: Any, idx: int) -> Optional[float]:
    if not isinstance(values, list):
        return None
    if idx < 0 or idx >= len(values):
        return None
    try:
        return float(values[idx])
    except Exception:
        return None


def parse_date(value: Optional[str]) -> date:
    if not value:
        return datetime.utcnow().date()
    try:
        return datetime.fromisoformat(value).date()
    except Exception:
        raise WeatherError("Invalid date format. Use YYYY-MM-DD.")
