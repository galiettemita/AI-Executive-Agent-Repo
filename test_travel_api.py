#!/usr/bin/env python3
"""
Simple script to test travel booking API endpoints.
Run this with: python test_travel_api.py
"""

import httpx
import json
from datetime import datetime, timedelta

# Base URL - update if testing on Render
BASE_URL = "http://localhost:8000"

# Test user ID
TEST_USER_ID = "test_user_travel_123"


def print_section(title):
    """Print a section header"""
    print("\n" + "="*60)
    print(f"  {title}")
    print("="*60)


def test_flight_search():
    """Test flight search endpoint"""
    print_section("Testing Flight Search")

    departure_date = (datetime.now() + timedelta(days=30)).strftime("%Y-%m-%d")
    return_date = (datetime.now() + timedelta(days=37)).strftime("%Y-%m-%d")

    payload = {
        "origin": "JFK",
        "destination": "LAX",
        "departure_date": departure_date,
        "return_date": return_date,
        "adults": 1,
        "travel_class": "ECONOMY",
        "max_results": 3,
    }

    print(f"\n📍 Request: POST {BASE_URL}/travel/flights/search")
    print(f"📦 Payload: {json.dumps(payload, indent=2)}")

    try:
        with httpx.Client(timeout=30.0) as client:
            response = client.post(
                f"{BASE_URL}/travel/flights/search",
                json=payload,
            )

        print(f"\n✓ Status: {response.status_code}")

        if response.status_code == 200:
            data = response.json()
            print(f"✓ Found {data['count']} flight offers")

            if data['offers']:
                offer = data['offers'][0]
                print(f"\n🎫 Sample Offer:")
                print(f"   - Price: ${offer['price']} {offer['currency']}")
                print(f"   - Airline: {offer['validating_airline']}")
                print(f"   - Segments: {len(offer['itineraries'][0]['segments'])}")
                return offer
        else:
            print(f"✗ Error: {response.text}")
            return None

    except Exception as e:
        print(f"✗ Request failed: {e}")
        return None


def test_hotel_search():
    """Test hotel search endpoint"""
    print_section("Testing Hotel Search")

    check_in = (datetime.now() + timedelta(days=30)).strftime("%Y-%m-%d")
    check_out = (datetime.now() + timedelta(days=33)).strftime("%Y-%m-%d")

    payload = {
        "city_code": "NYC",
        "check_in_date": check_in,
        "check_out_date": check_out,
        "adults": 1,
        "max_results": 3,
    }

    print(f"\n📍 Request: POST {BASE_URL}/travel/hotels/search/city")
    print(f"📦 Payload: {json.dumps(payload, indent=2)}")

    try:
        with httpx.Client(timeout=30.0) as client:
            response = client.post(
                f"{BASE_URL}/travel/hotels/search/city",
                json=payload,
            )

        print(f"\n✓ Status: {response.status_code}")

        if response.status_code == 200:
            data = response.json()
            print(f"✓ Found {data['count']} hotels")

            if data['hotels']:
                hotel = data['hotels'][0]
                print(f"\n🏨 Sample Hotel:")
                print(f"   - Name: {hotel['name']}")
                print(f"   - Rating: {hotel.get('rating', 'N/A')} stars")
                if hotel['cheapest_offer']:
                    print(f"   - Price: ${hotel['cheapest_offer']['price']}")
                return hotel
        else:
            print(f"✗ Error: {response.text}")
            return None

    except Exception as e:
        print(f"✗ Request failed: {e}")
        return None


def test_airport_info():
    """Test airport info endpoint"""
    print_section("Testing Airport Information")

    airport_code = "JFK"
    print(f"\n📍 Request: GET {BASE_URL}/travel/airports/{airport_code}")

    try:
        with httpx.Client(timeout=10.0) as client:
            response = client.get(f"{BASE_URL}/travel/airports/{airport_code}")

        print(f"\n✓ Status: {response.status_code}")

        if response.status_code == 200:
            data = response.json()
            if data.get('airport'):
                airport = data['airport']
                print(f"\n✈️ Airport Info:")
                print(f"   - Code: {airport['iata_code']}")
                print(f"   - Name: {airport['name']}")
                print(f"   - City: {airport['city']}")
                print(f"   - Country: {airport['country']}")
        else:
            print(f"⚠ Not found (may be expected in test environment)")

    except Exception as e:
        print(f"✗ Request failed: {e}")


def test_create_traveler_profile():
    """Test creating a traveler profile"""
    print_section("Testing Traveler Profile Creation")

    payload = {
        "first_name": "Jane",
        "last_name": "Smith",
        "date_of_birth": "1985-05-20",
        "gender": "FEMALE",
        "email": "jane.smith@example.com",
        "phone": "5551234567",
        "passport_number": "987654321",
        "passport_country": "US",
        "passport_expiry": "2028-12-31",
        "nationality": "US",
        "known_traveler_number": "87654321",
        "seat_preference": "AISLE",
        "meal_preference": "VEGETARIAN",
        "is_default": True,
    }

    print(f"\n📍 Request: POST {BASE_URL}/travel/travelers?user_id={TEST_USER_ID}")
    print(f"📦 Payload: {json.dumps(payload, indent=2)}")

    try:
        with httpx.Client(timeout=10.0) as client:
            response = client.post(
                f"{BASE_URL}/travel/travelers",
                params={"user_id": TEST_USER_ID},
                json=payload,
            )

        print(f"\n✓ Status: {response.status_code}")

        if response.status_code == 200:
            data = response.json()
            profile = data['profile']
            print(f"\n👤 Profile Created:")
            print(f"   - ID: {profile['id']}")
            print(f"   - Name: {profile['first_name']} {profile['last_name']}")
            print(f"   - Default: {profile['is_default']}")
            return profile['id']
        else:
            print(f"✗ Error: {response.text}")
            return None

    except Exception as e:
        print(f"✗ Request failed: {e}")
        return None


def test_list_traveler_profiles():
    """Test listing traveler profiles"""
    print_section("Testing List Traveler Profiles")

    print(f"\n📍 Request: GET {BASE_URL}/travel/travelers/{TEST_USER_ID}")

    try:
        with httpx.Client(timeout=10.0) as client:
            response = client.get(f"{BASE_URL}/travel/travelers/{TEST_USER_ID}")

        print(f"\n✓ Status: {response.status_code}")

        if response.status_code == 200:
            data = response.json()
            print(f"✓ Found {data['count']} profile(s)")

            for profile in data['profiles']:
                print(f"\n   Profile ID {profile['id']}:")
                print(f"   - Name: {profile['first_name']} {profile['last_name']}")
                print(f"   - DOB: {profile['date_of_birth']}")
                print(f"   - Default: {profile['is_default']}")
        else:
            print(f"⚠ No profiles found (this is okay for first run)")

    except Exception as e:
        print(f"✗ Request failed: {e}")


def test_health_check():
    """Test that the API is running"""
    print_section("Testing API Health")

    print(f"\n📍 Request: GET {BASE_URL}/")

    try:
        with httpx.Client(timeout=5.0) as client:
            response = client.get(f"{BASE_URL}/")

        print(f"\n✓ Status: {response.status_code}")

        if response.status_code == 200:
            data = response.json()
            print(f"✓ Service: {data.get('service', 'Unknown')}")
            print(f"✓ API is running!")
            return True
        else:
            print(f"✗ Unexpected response: {response.text}")
            return False

    except Exception as e:
        print(f"✗ Cannot connect to API: {e}")
        print(f"\n💡 Make sure the API is running:")
        print(f"   cd backend && uvicorn app.main:app --reload")
        return False


def main():
    """Run all tests"""
    print("\n" + "🚀"*30)
    print("  TRAVEL BOOKING API TEST SUITE")
    print("🚀"*30)

    # Check if API is running
    if not test_health_check():
        print("\n❌ API is not running. Please start the server first.")
        return

    # Run tests
    test_airport_info()
    test_create_traveler_profile()
    test_list_traveler_profiles()

    # These tests make actual Amadeus API calls
    print("\n\n⚠️  The following tests will make real Amadeus API calls:")
    input("   Press Enter to continue or Ctrl+C to skip...")

    flight_offer = test_flight_search()
    hotel_offer = test_hotel_search()

    # Summary
    print_section("TEST SUMMARY")

    if flight_offer or hotel_offer:
        print("\n✅ Integration tests passed!")
        print("\n📋 Next Steps:")
        print("  1. Create a proposal with a flight/hotel offer")
        print("  2. Generate an approval token")
        print("  3. Execute the booking via POST /execution/execute")
        print("\n📖 API Documentation: http://localhost:8000/docs")
    else:
        print("\n⚠️  Some tests failed. Check your Amadeus credentials.")
        print("\n🔧 Required environment variables:")
        print("   - AMADEUS_API_KEY")
        print("   - AMADEUS_API_SECRET")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\n\n👋 Tests cancelled by user")
    except Exception as e:
        print(f"\n\n❌ Unexpected error: {e}")
