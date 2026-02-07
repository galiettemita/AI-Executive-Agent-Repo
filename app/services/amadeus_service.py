# app/services/amadeus_service.py

from __future__ import annotations

import json
import logging
from typing import Dict, List, Optional, Any
from datetime import datetime, date
from amadeus import Client, ResponseError

from app.services.circuit_breaker import amadeus_breaker, CircuitBreakerError
from app.core.config import settings

logger = logging.getLogger(__name__)


class AmadeusService:
    """Service for Amadeus travel API integration"""

    def __init__(self):
        """Initialize Amadeus client"""
        api_key = settings.AMADEUS_API_KEY
        api_secret = settings.AMADEUS_API_SECRET

        if not api_key or not api_secret:
            raise ValueError("AMADEUS_API_KEY and AMADEUS_API_SECRET must be set")

        self.client = Client(
            client_id=api_key,
            client_secret=api_secret,
        )

    # -------------------
    # FLIGHT OPERATIONS
    # -------------------

    def search_flights(
        self,
        origin: str,
        destination: str,
        departure_date: str,  # YYYY-MM-DD
        return_date: Optional[str] = None,  # YYYY-MM-DD
        adults: int = 1,
        travel_class: str = "ECONOMY",
        max_results: int = 5,
    ) -> List[Dict]:
        """
        Search for flight offers.

        Args:
            origin: Origin airport IATA code (e.g., "JFK")
            destination: Destination airport IATA code (e.g., "LAX")
            departure_date: Departure date in YYYY-MM-DD format
            return_date: Optional return date for round-trip
            adults: Number of adult passengers
            travel_class: ECONOMY, PREMIUM_ECONOMY, BUSINESS, or FIRST
            max_results: Maximum number of offers to return

        Returns:
            List of flight offers with pricing
        """
        try:
            params = {
                "originLocationCode": origin,
                "destinationLocationCode": destination,
                "departureDate": departure_date,
                "adults": adults,
                "travelClass": travel_class,
                "max": max_results,
            }

            if return_date:
                params["returnDate"] = return_date

            # Use circuit breaker for API call
            with amadeus_breaker:
                response = self.client.shopping.flight_offers_search.get(**params)

            # Parse and format results
            offers = []
            for offer in response.data:
                parsed_offer = self._parse_flight_offer(offer)
                offers.append(parsed_offer)

            return offers

        except CircuitBreakerError as e:
            logger.warning(f"Amadeus circuit breaker open: {e}")
            raise ValueError("Flight search service is temporarily unavailable. Please try again later.")
        except ResponseError as error:
            raise ValueError(f"Amadeus API error: {error}")

    def _parse_flight_offer(self, offer: Any) -> Dict:
        """Parse Amadeus flight offer into simplified format"""
        # Extract basic info
        price = float(offer["price"]["total"])
        currency = offer["price"]["currency"]

        # Extract itineraries
        itineraries = []
        for itinerary in offer["itineraries"]:
            segments = []
            for segment in itinerary["segments"]:
                segments.append({
                    "departure": {
                        "airport": segment["departure"]["iataCode"],
                        "terminal": segment["departure"].get("terminal"),
                        "time": segment["departure"]["at"],
                    },
                    "arrival": {
                        "airport": segment["arrival"]["iataCode"],
                        "terminal": segment["arrival"].get("terminal"),
                        "time": segment["arrival"]["at"],
                    },
                    "carrier": segment["carrierCode"],
                    "flight_number": segment["number"],
                    "aircraft": segment.get("aircraft", {}).get("code"),
                    "duration": segment["duration"],
                })

            itineraries.append({
                "duration": itinerary["duration"],
                "segments": segments,
            })

        return {
            "id": offer["id"],
            "price": price,
            "currency": currency,
            "itineraries": itineraries,
            "validating_airline": offer.get("validatingAirlineCodes", [None])[0],
            "traveler_pricings": offer.get("travelerPricings", []),
            "raw_offer": offer,  # Store full offer for booking
        }

    def book_flight(
        self,
        offer_id: str,
        offer_data: Dict,
        travelers: List[Dict],
        contact_email: str,
        contact_phone: str,
    ) -> Dict:
        """
        Book a flight using an offer.

        Args:
            offer_id: Offer ID from search results
            offer_data: Full offer data from search
            travelers: List of traveler information
            contact_email: Contact email
            contact_phone: Contact phone

        Returns:
            Dict with booking confirmation
        """
        try:
            # Format travelers for Amadeus
            formatted_travelers = []
            for i, traveler in enumerate(travelers):
                formatted_travelers.append({
                    "id": str(i + 1),
                    "dateOfBirth": traveler["date_of_birth"],  # YYYY-MM-DD
                    "name": {
                        "firstName": traveler["first_name"],
                        "lastName": traveler["last_name"],
                    },
                    "gender": traveler.get("gender", "MALE"),  # MALE or FEMALE
                    "contact": {
                        "emailAddress": contact_email,
                        "phones": [{
                            "deviceType": "MOBILE",
                            "countryCallingCode": "1",
                            "number": contact_phone.replace("-", "").replace(" ", ""),
                        }],
                    },
                    "documents": traveler.get("documents", []),
                })

            # Create flight order
            booking_data = {
                "data": {
                    "type": "flight-order",
                    "flightOffers": [offer_data["raw_offer"]],
                    "travelers": formatted_travelers,
                }
            }

            # Use circuit breaker for API call
            with amadeus_breaker:
                response = self.client.booking.flight_orders.post(booking_data)

            # Parse booking response
            order = response.data

            return {
                "booking_id": order["id"],
                "pnr": order.get("associatedRecords", [{}])[0].get("reference"),
                "status": "confirmed",
                "confirmation_number": order.get("associatedRecords", [{}])[0].get("reference"),
                "total_price": float(order["flightOffers"][0]["price"]["total"]),
                "currency": order["flightOffers"][0]["price"]["currency"],
                "tickets": order.get("ticketingAgreement", {}),
                "travelers": order.get("travelers", []),
                "raw_order": order,
            }

        except CircuitBreakerError as e:
            logger.warning(f"Amadeus circuit breaker open: {e}")
            raise ValueError("Flight booking service is temporarily unavailable. Please try again later.")
        except ResponseError as error:
            raise ValueError(f"Booking failed: {error}")

    def get_flight_price(
        self,
        offer_data: Dict,
    ) -> Dict:
        """
        Confirm pricing for a flight offer before booking.

        Args:
            offer_data: Flight offer data

        Returns:
            Dict with confirmed pricing
        """
        try:
            # Use circuit breaker for API call
            with amadeus_breaker:
                response = self.client.shopping.flight_offers.pricing.post(
                    offer_data["raw_offer"]
                )

            confirmed_offer = response.data["flightOffers"][0]

            return {
                "price": float(confirmed_offer["price"]["total"]),
                "currency": confirmed_offer["price"]["currency"],
                "confirmed": True,
                "offer_data": confirmed_offer,
            }

        except CircuitBreakerError as e:
            logger.warning(f"Amadeus circuit breaker open: {e}")
            raise ValueError("Price confirmation service is temporarily unavailable. Please try again later.")
        except ResponseError as error:
            raise ValueError(f"Price confirmation failed: {error}")

    # -------------------
    # HOTEL OPERATIONS
    # -------------------

    def search_hotels_by_city(
        self,
        city_code: str,  # IATA city code (e.g., "NYC")
        check_in_date: str,  # YYYY-MM-DD
        check_out_date: str,  # YYYY-MM-DD
        adults: int = 1,
        radius: int = 20,  # Radius in km
        radius_unit: str = "KM",
        max_results: int = 10,
    ) -> List[Dict]:
        """
        Search for hotels by city.

        Args:
            city_code: IATA city code
            check_in_date: Check-in date (YYYY-MM-DD)
            check_out_date: Check-out date (YYYY-MM-DD)
            adults: Number of adults
            radius: Search radius
            radius_unit: KM or MILE
            max_results: Maximum results

        Returns:
            List of hotel offers
        """
        try:
            # Use circuit breaker for API calls
            with amadeus_breaker:
                # First, get hotel list by city
                hotels_response = self.client.reference_data.locations.hotels.by_city.get(
                    cityCode=city_code
                )

            if not hotels_response.data:
                return []

            # Get hotel IDs (limit to max_results)
            hotel_ids = [hotel["hotelId"] for hotel in hotels_response.data[:max_results]]

            with amadeus_breaker:
                # Search for offers
                offers_response = self.client.shopping.hotel_offers_search.get(
                    hotelIds=",".join(hotel_ids),
                    checkInDate=check_in_date,
                    checkOutDate=check_out_date,
                    adults=adults,
                )

            # Parse results
            hotels = []
            for hotel_data in offers_response.data:
                hotel = self._parse_hotel_offer(hotel_data)
                hotels.append(hotel)

            return hotels

        except CircuitBreakerError as e:
            logger.warning(f"Amadeus circuit breaker open: {e}")
            raise ValueError("Hotel search service is temporarily unavailable. Please try again later.")
        except ResponseError as error:
            raise ValueError(f"Hotel search failed: {error}")

    def search_hotels_by_geocode(
        self,
        latitude: float,
        longitude: float,
        check_in_date: str,
        check_out_date: str,
        adults: int = 1,
        radius: int = 20,
        max_results: int = 10,
    ) -> List[Dict]:
        """
        Search for hotels by geographic coordinates.

        Args:
            latitude: Latitude
            longitude: Longitude
            check_in_date: Check-in date (YYYY-MM-DD)
            check_out_date: Check-out date (YYYY-MM-DD)
            adults: Number of adults
            radius: Search radius in km
            max_results: Maximum results

        Returns:
            List of hotel offers
        """
        try:
            # Use circuit breaker for API calls
            with amadeus_breaker:
                # Get hotels by geocode
                hotels_response = self.client.reference_data.locations.hotels.by_geocode.get(
                    latitude=latitude,
                    longitude=longitude,
                    radius=radius,
                )

            if not hotels_response.data:
                return []

            # Get hotel IDs
            hotel_ids = [hotel["hotelId"] for hotel in hotels_response.data[:max_results]]

            with amadeus_breaker:
                # Search for offers
                offers_response = self.client.shopping.hotel_offers_search.get(
                    hotelIds=",".join(hotel_ids),
                    checkInDate=check_in_date,
                    checkOutDate=check_out_date,
                    adults=adults,
                )

            # Parse results
            hotels = []
            for hotel_data in offers_response.data:
                hotel = self._parse_hotel_offer(hotel_data)
                hotels.append(hotel)

            return hotels

        except CircuitBreakerError as e:
            logger.warning(f"Amadeus circuit breaker open: {e}")
            raise ValueError("Hotel search service is temporarily unavailable. Please try again later.")
        except ResponseError as error:
            raise ValueError(f"Hotel search failed: {error}")

    def _parse_hotel_offer(self, hotel_data: Any) -> Dict:
        """Parse Amadeus hotel offer"""
        hotel = hotel_data["hotel"]
        offers = hotel_data.get("offers", [])

        # Get cheapest offer
        cheapest_offer = None
        if offers:
            cheapest_offer = min(offers, key=lambda x: float(x["price"]["total"]))

        return {
            "hotel_id": hotel["hotelId"],
            "name": hotel.get("name", "Unknown Hotel"),
            "rating": hotel.get("rating"),
            "location": {
                "latitude": hotel.get("latitude"),
                "longitude": hotel.get("longitude"),
                "address": hotel.get("address", {}),
            },
            "contact": hotel.get("contact", {}),
            "amenities": hotel.get("amenities", []),
            "cheapest_offer": {
                "id": cheapest_offer["id"] if cheapest_offer else None,
                "price": float(cheapest_offer["price"]["total"]) if cheapest_offer else None,
                "currency": cheapest_offer["price"]["currency"] if cheapest_offer else None,
                "room": cheapest_offer.get("room", {}) if cheapest_offer else {},
            } if cheapest_offer else None,
            "all_offers": offers,
            "raw_data": hotel_data,
        }

    def book_hotel(
        self,
        offer_id: str,
        offer_data: Dict,
        guests: List[Dict],
        contact_email: str,
        contact_phone: str,
        payment_card: Dict,
    ) -> Dict:
        """
        Book a hotel room.

        Args:
            offer_id: Offer ID
            offer_data: Full offer data
            guests: List of guest information
            contact_email: Contact email
            contact_phone: Contact phone
            payment_card: Payment card details

        Returns:
            Dict with booking confirmation
        """
        try:
            # Format guests
            formatted_guests = []
            for guest in guests:
                formatted_guests.append({
                    "name": {
                        "firstName": guest["first_name"],
                        "lastName": guest["last_name"],
                    },
                    "contact": {
                        "email": contact_email,
                        "phone": contact_phone,
                    },
                })

            # Prepare booking data
            booking_data = {
                "data": {
                    "offerId": offer_id,
                    "guests": formatted_guests,
                    "payments": [{
                        "method": "creditCard",
                        "card": payment_card,
                    }],
                }
            }

            # Use circuit breaker for API call
            with amadeus_breaker:
                response = self.client.booking.hotel_bookings.post(booking_data)

            # Parse response
            booking = response.data[0]

            return {
                "booking_id": booking["id"],
                "confirmation_number": booking.get("id"),
                "status": "confirmed",
                "hotel_id": booking["hotel"]["hotelId"],
                "hotel_name": booking["hotel"].get("name"),
                "check_in": booking.get("checkInDate"),
                "check_out": booking.get("checkOutDate"),
                "total_price": float(booking["price"]["total"]),
                "currency": booking["price"]["currency"],
                "raw_booking": booking,
            }

        except CircuitBreakerError as e:
            logger.warning(f"Amadeus circuit breaker open: {e}")
            raise ValueError("Hotel booking service is temporarily unavailable. Please try again later.")
        except ResponseError as error:
            raise ValueError(f"Hotel booking failed: {error}")

    # -------------------
    # UTILITY METHODS
    # -------------------

    def get_airport_info(self, iata_code: str) -> Dict:
        """
        Get airport information.

        Args:
            iata_code: Airport IATA code

        Returns:
            Dict with airport details
        """
        try:
            response = self.client.reference_data.locations.get(
                keyword=iata_code,
                subType="AIRPORT",
            )

            if response.data:
                airport = response.data[0]
                return {
                    "iata_code": airport["iataCode"],
                    "name": airport["name"],
                    "city": airport["address"]["cityName"],
                    "country": airport["address"]["countryName"],
                    "latitude": airport["geoCode"]["latitude"],
                    "longitude": airport["geoCode"]["longitude"],
                }

            return None

        except ResponseError as error:
            return None

    # -------------------
    # CANCELLATION METHODS
    # -------------------

    def cancel_flight_order(self, order_id: str) -> Dict:
        """
        Cancel a flight order.

        Note: Amadeus Self-Service API has limited cancellation support.
        This method attempts to delete the flight order.

        Args:
            order_id: The Amadeus flight order ID

        Returns:
            Dict with cancellation status
        """
        try:
            with amadeus_breaker:
                # Amadeus Flight Orders API - DELETE method
                response = self.client.booking.flight_order(order_id).delete()

            return {
                "success": True,
                "order_id": order_id,
                "status": "cancelled",
                "message": "Flight order cancelled successfully",
            }

        except CircuitBreakerError as e:
            logger.warning(f"Amadeus circuit breaker open: {e}")
            raise ValueError("Cancellation service is temporarily unavailable. Please try again later.")
        except ResponseError as error:
            # Check if it's a "not cancellable" error
            error_detail = str(error)
            if "UNABLE TO PROCESS" in error_detail.upper() or "NOT CANCELLABLE" in error_detail.upper():
                raise ValueError(
                    "This booking cannot be cancelled online. Please contact the airline directly."
                )
            raise ValueError(f"Cancellation failed: {error}")

    def get_flight_order(self, order_id: str) -> Dict:
        """
        Get flight order details.

        Args:
            order_id: The Amadeus flight order ID

        Returns:
            Dict with order details
        """
        try:
            with amadeus_breaker:
                response = self.client.booking.flight_order(order_id).get()

            order = response.data

            return {
                "order_id": order["id"],
                "status": order.get("type", "flight-order"),
                "travelers": order.get("travelers", []),
                "flight_offers": order.get("flightOffers", []),
                "raw_order": order,
            }

        except CircuitBreakerError as e:
            logger.warning(f"Amadeus circuit breaker open: {e}")
            raise ValueError("Service is temporarily unavailable. Please try again later.")
        except ResponseError as error:
            raise ValueError(f"Failed to retrieve order: {error}")

    def cancel_hotel_booking(self, booking_id: str) -> Dict:
        """
        Cancel a hotel booking.

        Note: Hotel cancellation support varies by property and rate.
        Some bookings may be non-refundable.

        Args:
            booking_id: The Amadeus hotel booking ID

        Returns:
            Dict with cancellation status
        """
        try:
            with amadeus_breaker:
                # Amadeus Hotel Bookings API - DELETE method
                response = self.client.booking.hotel_booking(booking_id).delete()

            return {
                "success": True,
                "booking_id": booking_id,
                "status": "cancelled",
                "message": "Hotel booking cancelled successfully",
            }

        except CircuitBreakerError as e:
            logger.warning(f"Amadeus circuit breaker open: {e}")
            raise ValueError("Cancellation service is temporarily unavailable. Please try again later.")
        except ResponseError as error:
            error_detail = str(error)
            if "NON-REFUNDABLE" in error_detail.upper():
                raise ValueError(
                    "This hotel booking is non-refundable and cannot be cancelled."
                )
            raise ValueError(f"Hotel cancellation failed: {error}")
