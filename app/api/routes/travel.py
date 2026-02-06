# app/api/routes/travel.py

from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy.orm import Session
from typing import List, Optional
import json

from app.db.database import get_db
from app.db.models import TravelerProfile
from app.services.amadeus_service import AmadeusService
from app.services.encryption_service import encrypt_pii, decrypt_pii

router = APIRouter(prefix="/travel", tags=["travel"])


# -------------------
# REQUEST MODELS
# -------------------

class FlightSearchRequest(BaseModel):
    origin: str
    destination: str
    departure_date: str  # YYYY-MM-DD
    return_date: Optional[str] = None
    adults: int = 1
    travel_class: str = "ECONOMY"
    max_results: int = 5


class HotelSearchByCityRequest(BaseModel):
    city_code: str
    check_in_date: str  # YYYY-MM-DD
    check_out_date: str
    adults: int = 1
    max_results: int = 10


class HotelSearchByLocationRequest(BaseModel):
    latitude: float
    longitude: float
    check_in_date: str
    check_out_date: str
    adults: int = 1
    radius: int = 20
    max_results: int = 10


class TravelerProfileRequest(BaseModel):
    first_name: str
    last_name: str
    middle_name: Optional[str] = None
    date_of_birth: str  # YYYY-MM-DD
    gender: str  # MALE, FEMALE, OTHER
    email: Optional[str] = None
    phone: Optional[str] = None
    passport_number: Optional[str] = None
    passport_country: Optional[str] = None
    passport_expiry: Optional[str] = None
    nationality: Optional[str] = None
    known_traveler_number: Optional[str] = None
    redress_number: Optional[str] = None
    seat_preference: Optional[str] = None
    meal_preference: Optional[str] = None
    loyalty_programs: Optional[List[dict]] = None
    is_default: bool = False


# -------------------
# FLIGHT ENDPOINTS
# -------------------

@router.post("/flights/search")
def search_flights(
    request: FlightSearchRequest,
    db: Session = Depends(get_db),
):
    """
    Search for flight offers.

    Args:
        request: FlightSearchRequest with search parameters
        db: Database session

    Returns:
        Dict with list of flight offers
    """
    try:
        amadeus = AmadeusService()

        offers = amadeus.search_flights(
            origin=request.origin,
            destination=request.destination,
            departure_date=request.departure_date,
            return_date=request.return_date,
            adults=request.adults,
            travel_class=request.travel_class,
            max_results=request.max_results,
        )

        return {
            "ok": True,
            "offers": offers,
            "count": len(offers),
        }

    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/airports/{iata_code}")
def get_airport_info(
    iata_code: str,
    db: Session = Depends(get_db),
):
    """Get airport information by IATA code"""
    try:
        amadeus = AmadeusService()
        airport = amadeus.get_airport_info(iata_code)

        if not airport:
            raise HTTPException(status_code=404, detail="Airport not found")

        return {
            "ok": True,
            "airport": airport,
        }

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


# -------------------
# HOTEL ENDPOINTS
# -------------------

@router.post("/hotels/search/city")
def search_hotels_by_city(
    request: HotelSearchByCityRequest,
    db: Session = Depends(get_db),
):
    """
    Search for hotels by city code.

    Args:
        request: HotelSearchByCityRequest with search parameters
        db: Database session

    Returns:
        Dict with list of hotel offers
    """
    try:
        amadeus = AmadeusService()

        hotels = amadeus.search_hotels_by_city(
            city_code=request.city_code,
            check_in_date=request.check_in_date,
            check_out_date=request.check_out_date,
            adults=request.adults,
            max_results=request.max_results,
        )

        return {
            "ok": True,
            "hotels": hotels,
            "count": len(hotels),
        }

    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/hotels/search/location")
def search_hotels_by_location(
    request: HotelSearchByLocationRequest,
    db: Session = Depends(get_db),
):
    """
    Search for hotels by geographic coordinates.

    Args:
        request: HotelSearchByLocationRequest with search parameters
        db: Database session

    Returns:
        Dict with list of hotel offers
    """
    try:
        amadeus = AmadeusService()

        hotels = amadeus.search_hotels_by_geocode(
            latitude=request.latitude,
            longitude=request.longitude,
            check_in_date=request.check_in_date,
            check_out_date=request.check_out_date,
            adults=request.adults,
            radius=request.radius,
            max_results=request.max_results,
        )

        return {
            "ok": True,
            "hotels": hotels,
            "count": len(hotels),
        }

    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


# -------------------
# TRAVELER PROFILE ENDPOINTS
# -------------------

@router.post("/travelers")
def create_traveler_profile(
    user_id: str,  # In production, get from auth token
    request: TravelerProfileRequest,
    db: Session = Depends(get_db),
):
    """
    Create a new traveler profile.

    Args:
        user_id: User ID (from auth token in production)
        request: TravelerProfileRequest with profile data
        db: Database session

    Returns:
        Dict with created profile
    """
    # If setting as default, unset other defaults
    if request.is_default:
        db.query(TravelerProfile).filter(
            TravelerProfile.user_id == user_id,
            TravelerProfile.is_default == True,
        ).update({"is_default": False})

    # Create profile with encrypted PII fields
    profile = TravelerProfile(
        user_id=user_id,
        first_name=request.first_name,
        last_name=request.last_name,
        middle_name=request.middle_name,
        date_of_birth=encrypt_pii(request.date_of_birth),  # Encrypted
        gender=request.gender,
        email=encrypt_pii(request.email),  # Encrypted
        phone=encrypt_pii(request.phone),  # Encrypted
        passport_number=encrypt_pii(request.passport_number),  # Encrypted
        passport_country=request.passport_country,
        passport_expiry=request.passport_expiry,
        nationality=request.nationality,
        known_traveler_number=encrypt_pii(request.known_traveler_number),  # Encrypted
        redress_number=encrypt_pii(request.redress_number),  # Encrypted
        seat_preference=request.seat_preference,
        meal_preference=request.meal_preference,
        loyalty_programs=json.dumps(request.loyalty_programs) if request.loyalty_programs else None,
        is_default=request.is_default,
    )

    db.add(profile)
    db.commit()
    db.refresh(profile)

    return {
        "ok": True,
        "profile": {
            "id": profile.id,
            "first_name": profile.first_name,
            "last_name": profile.last_name,
            "date_of_birth": decrypt_pii(profile.date_of_birth),  # Decrypted for response
            "is_default": profile.is_default,
            "created_at": profile.created_at.isoformat(),
        },
    }


@router.get("/travelers/{user_id}")
def list_traveler_profiles(
    user_id: str,
    db: Session = Depends(get_db),
):
    """List all traveler profiles for a user"""
    profiles = (
        db.query(TravelerProfile)
        .filter(TravelerProfile.user_id == user_id)
        .order_by(TravelerProfile.is_default.desc(), TravelerProfile.created_at.desc())
        .all()
    )

    return {
        "profiles": [
            {
                "id": p.id,
                "first_name": p.first_name,
                "last_name": p.last_name,
                "middle_name": p.middle_name,
                "date_of_birth": decrypt_pii(p.date_of_birth),  # Decrypted
                "gender": p.gender,
                "passport_number": decrypt_pii(p.passport_number),  # Decrypted (masked in list)
                "is_default": p.is_default,
                "created_at": p.created_at.isoformat(),
            }
            for p in profiles
        ],
        "count": len(profiles),
    }


@router.get("/travelers/profile/{profile_id}")
def get_traveler_profile(
    profile_id: int,
    db: Session = Depends(get_db),
):
    """Get detailed traveler profile"""
    profile = db.query(TravelerProfile).filter(TravelerProfile.id == profile_id).first()

    if not profile:
        raise HTTPException(status_code=404, detail="Profile not found")

    return {
        "ok": True,
        "profile": {
            "id": profile.id,
            "user_id": profile.user_id,
            "first_name": profile.first_name,
            "last_name": profile.last_name,
            "middle_name": profile.middle_name,
            "date_of_birth": decrypt_pii(profile.date_of_birth),  # Decrypted
            "gender": profile.gender,
            "email": decrypt_pii(profile.email),  # Decrypted
            "phone": decrypt_pii(profile.phone),  # Decrypted
            "passport_number": decrypt_pii(profile.passport_number),  # Decrypted
            "passport_country": profile.passport_country,
            "passport_expiry": profile.passport_expiry,
            "nationality": profile.nationality,
            "known_traveler_number": decrypt_pii(profile.known_traveler_number),  # Decrypted
            "redress_number": decrypt_pii(profile.redress_number),  # Decrypted
            "seat_preference": profile.seat_preference,
            "meal_preference": profile.meal_preference,
            "loyalty_programs": json.loads(profile.loyalty_programs) if profile.loyalty_programs else [],
            "is_default": profile.is_default,
            "created_at": profile.created_at.isoformat(),
        },
    }


@router.delete("/travelers/profile/{profile_id}")
def delete_traveler_profile(
    profile_id: int,
    user_id: str,  # In production, get from auth token
    db: Session = Depends(get_db),
):
    """Delete a traveler profile"""
    profile = (
        db.query(TravelerProfile)
        .filter(
            TravelerProfile.id == profile_id,
            TravelerProfile.user_id == user_id,
        )
        .first()
    )

    if not profile:
        raise HTTPException(status_code=404, detail="Profile not found")

    db.delete(profile)
    db.commit()

    return {
        "ok": True,
        "message": "Profile deleted",
    }
