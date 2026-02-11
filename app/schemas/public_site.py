from __future__ import annotations

from typing import Optional

from pydantic import BaseModel


class LocationShareRequest(BaseModel):
    user_id: str
    consent: bool = True
    source: Optional[str] = "browser"
    latitude: Optional[float] = None
    longitude: Optional[float] = None
    accuracy_m: Optional[float] = None
    city: Optional[str] = None
    region: Optional[str] = None
    country: Optional[str] = None
    timezone: Optional[str] = None
    location: Optional[str] = None
    ip: Optional[str] = None


class LocationIpRequest(BaseModel):
    user_id: str
    consent: bool = True
