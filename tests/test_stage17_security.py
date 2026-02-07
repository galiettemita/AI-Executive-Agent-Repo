# tests/test_stage17_security.py

"""
Stage 17: Security Testing Suite

Tests for:
1. Travel booking flow (mock Amadeus)
2. GDPR data deletion compliance
3. PII encryption validation
4. Circuit breaker functionality
5. Rate limiting
"""

import pytest
import os
import uuid
import json
from datetime import datetime, timedelta
from unittest.mock import Mock, patch, MagicMock

# Set test environment variables BEFORE imports
os.environ.setdefault("JWT_SECRET", "test_jwt_secret_for_testing_only")
os.environ.setdefault("PII_ENCRYPTION_KEY", "dEkqDdDoZOe8lnV0fi9-SWicfi8UNtMvYnrqYH50mPU=")

from app.db.database import SessionLocal, Base, engine
from app.db.models import (
    User,
    TravelerProfile,
    PaymentMethod,
    Transaction,
    Proposal,
    Booking,
    Conversation,
    ChatMessage,
    OAuthToken,
)
from app.services.encryption_service import (
    EncryptionService,
    encrypt_pii,
    decrypt_pii,
    is_encrypted,
    encrypt_traveler_data,
    decrypt_traveler_data,
    generate_encryption_key,
    get_encryption_service,
    ENCRYPTED_FIELDS,
)
from app.core.config import settings

# Clear the encryption service cache to pick up test env vars
get_encryption_service.cache_clear()
from app.services.gdpr_service import GDPRService, delete_user_data, export_user_data
from app.services.circuit_breaker import (
    CircuitBreaker,
    CircuitBreakerError,
    CircuitState,
    get_circuit_breaker,
    reset_all_circuit_breakers,
)


# ==========================================
# FIXTURES
# ==========================================

@pytest.fixture(scope="module", autouse=True)
def setup_database():
    """Create all tables before running tests"""
    Base.metadata.create_all(bind=engine)
    yield


@pytest.fixture
def db():
    """Database session fixture"""
    session = SessionLocal()
    try:
        yield session
    finally:
        session.close()


@pytest.fixture
def test_user_id():
    """Generate unique test user ID"""
    return f"test_user_{uuid.uuid4().hex[:8]}"


@pytest.fixture
def create_test_user(db, test_user_id):
    """Create a test user with related data"""
    user = User(id=test_user_id)
    db.add(user)
    db.commit()
    return user


# ==========================================
# 1. PII ENCRYPTION TESTS
# ==========================================

class TestPIIEncryption:
    """Test PII encryption and decryption"""

    def test_encryption_key_generation(self):
        """Test that encryption key is generated correctly"""
        key = generate_encryption_key()
        assert key is not None
        assert len(key) == 44  # Fernet key is 44 chars base64
        print(f"Generated key: {key[:8]}...{key[-4:]}")

    def test_encrypt_decrypt_roundtrip(self):
        """Test that encrypt -> decrypt returns original value"""
        original = "AB123456789"  # Passport number

        encrypted = encrypt_pii(original)
        assert encrypted is not None
        assert encrypted != original
        assert is_encrypted(encrypted)

        decrypted = decrypt_pii(encrypted)
        assert decrypted == original
        print(f"Original: {original}")
        print(f"Encrypted: {encrypted[:20]}...")
        print(f"Decrypted: {decrypted}")

    def test_encrypt_none_returns_none(self):
        """Test that encrypting None returns None"""
        assert encrypt_pii(None) is None
        assert encrypt_pii("") is None

    def test_decrypt_none_returns_none(self):
        """Test that decrypting None returns None"""
        assert decrypt_pii(None) is None
        assert decrypt_pii("") is None

    def test_is_encrypted_detection(self):
        """Test encrypted value detection"""
        encrypted = encrypt_pii("test_value")
        assert is_encrypted(encrypted) is True
        assert is_encrypted("plain_text") is False
        assert is_encrypted(None) is False

    def test_encrypt_traveler_data(self):
        """Test encrypting traveler profile data"""
        data = {
            "first_name": "John",
            "last_name": "Doe",
            "date_of_birth": "1990-01-15",
            "passport_number": "AB123456",
            "email": "john@example.com",
            "phone": "+1234567890",
            "known_traveler_number": "12345678",
            "redress_number": "98765",
            "gender": "MALE",
        }

        encrypted_data = encrypt_traveler_data(data)

        # Check encrypted fields are encrypted
        for field in ENCRYPTED_FIELDS:
            if field in data and data[field]:
                assert is_encrypted(encrypted_data[field]), f"{field} should be encrypted"

        # Check non-encrypted fields remain unchanged
        assert encrypted_data["first_name"] == "John"
        assert encrypted_data["last_name"] == "Doe"
        assert encrypted_data["gender"] == "MALE"

        print("Encrypted fields:")
        for field in ENCRYPTED_FIELDS:
            if field in encrypted_data and encrypted_data[field]:
                print(f"  {field}: {encrypted_data[field][:25]}...")

    def test_decrypt_traveler_data(self):
        """Test decrypting traveler profile data"""
        original = {
            "first_name": "Jane",
            "last_name": "Smith",
            "date_of_birth": "1985-05-20",
            "passport_number": "XY987654",
            "email": "jane@example.com",
            "phone": "+9876543210",
        }

        encrypted = encrypt_traveler_data(original)
        decrypted = decrypt_traveler_data(encrypted)

        assert decrypted["date_of_birth"] == original["date_of_birth"]
        assert decrypted["passport_number"] == original["passport_number"]
        assert decrypted["email"] == original["email"]
        assert decrypted["phone"] == original["phone"]

    def test_double_encryption_prevention(self):
        """Test that data isn't encrypted twice"""
        original = "test_passport_123"
        encrypted_once = encrypt_pii(original)

        # encrypt_traveler_data should not re-encrypt
        data = {"passport_number": encrypted_once}
        encrypted_data = encrypt_traveler_data(data)

        # Should be the same (not double-encrypted)
        assert encrypted_data["passport_number"] == encrypted_once

        # Should decrypt correctly
        decrypted = decrypt_pii(encrypted_data["passport_number"])
        assert decrypted == original


# ==========================================
# 2. GDPR COMPLIANCE TESTS
# ==========================================

class TestGDPRCompliance:
    """Test GDPR data deletion and export"""

    def test_delete_user_data_dry_run(self, db, test_user_id, create_test_user):
        """Test dry run deletion (preview)"""
        # Create some test data
        profile = TravelerProfile(
            user_id=test_user_id,
            first_name="Test",
            last_name="User",
            date_of_birth="1990-01-01",
            gender="MALE",
        )
        db.add(profile)
        db.commit()

        # Run dry-run deletion
        result = delete_user_data(db, test_user_id, dry_run=True)

        assert result["success"] is True
        assert result["dry_run"] is True
        assert result["tables"]["traveler_profiles"]["count"] == 1

        # Verify data still exists
        profile_check = db.query(TravelerProfile).filter(
            TravelerProfile.user_id == test_user_id
        ).first()
        assert profile_check is not None

        print(f"Dry run result: {json.dumps(result, indent=2, default=str)}")

    def test_delete_user_data_actual(self, db):
        """Test actual user data deletion"""
        # Create a new user for this test
        user_id = f"gdpr_test_{uuid.uuid4().hex[:8]}"

        user = User(id=user_id)
        db.add(user)

        profile = TravelerProfile(
            user_id=user_id,
            first_name="Delete",
            last_name="Me",
            date_of_birth="1990-01-01",
            gender="FEMALE",
        )
        db.add(profile)

        conversation = Conversation(user_id=user_id)
        db.add(conversation)
        db.commit()

        # Verify data exists
        assert db.query(User).filter(User.id == user_id).first() is not None
        assert db.query(TravelerProfile).filter(TravelerProfile.user_id == user_id).first() is not None

        # Actually delete
        result = delete_user_data(db, user_id, dry_run=False)

        assert result["success"] is True
        assert result["dry_run"] is False

        # Verify data is deleted
        assert db.query(User).filter(User.id == user_id).first() is None
        assert db.query(TravelerProfile).filter(TravelerProfile.user_id == user_id).first() is None
        assert db.query(Conversation).filter(Conversation.user_id == user_id).first() is None

        print(f"Deleted user {user_id} successfully")
        print(f"Tables affected: {list(result['tables'].keys())}")

    def test_export_user_data(self, db):
        """Test user data export (data portability)"""
        # Create user with data
        user_id = f"export_test_{uuid.uuid4().hex[:8]}"

        user = User(id=user_id)
        db.add(user)

        profile = TravelerProfile(
            user_id=user_id,
            first_name="Export",
            last_name="Test",
            date_of_birth=encrypt_pii("1990-01-01"),
            gender="MALE",
            email=encrypt_pii("export@test.com"),
        )
        db.add(profile)
        db.commit()

        # Export data
        result = export_user_data(db, user_id)

        assert result["user_id"] == user_id
        assert "exported_at" in result
        assert "data" in result
        assert "users" in result["data"]
        assert "traveler_profiles" in result["data"]

        print(f"Exported data for user {user_id}:")
        print(f"  Tables exported: {list(result['data'].keys())}")

        # Clean up
        delete_user_data(db, user_id, dry_run=False)

    def test_transaction_anonymization(self, db):
        """Test that transactions are anonymized, not deleted"""
        user_id = f"anon_test_{uuid.uuid4().hex[:8]}"

        user = User(id=user_id)
        db.add(user)

        # Create a proposal first (required for transaction)
        proposal = Proposal(
            user_id=user_id,
            proposal_type="test",
            status="completed",
            payload_json="{}",
        )
        db.add(proposal)
        db.commit()

        transaction = Transaction(
            user_id=user_id,
            proposal_id=proposal.id,
            amount=100.00,
            currency="usd",
            status="succeeded",
            transaction_type="test",
        )
        db.add(transaction)
        db.commit()

        txn_id = transaction.id

        # Delete user data (should anonymize transactions)
        result = delete_user_data(
            db, user_id,
            dry_run=False,
            keep_anonymized_transactions=True
        )

        assert result["success"] is True
        assert result["tables"]["transactions"]["action"] == "anonymized"

        # Transaction should still exist but be anonymized
        txn = db.query(Transaction).filter(Transaction.id == txn_id).first()
        assert txn is not None
        assert txn.user_id.startswith("DELETED_")

        print(f"Transaction anonymized: {txn.user_id}")


# ==========================================
# 3. CIRCUIT BREAKER TESTS
# ==========================================

class TestCircuitBreaker:
    """Test circuit breaker functionality"""

    def setup_method(self):
        """Reset circuit breakers before each test"""
        reset_all_circuit_breakers()

    def test_circuit_breaker_initial_state(self):
        """Test circuit breaker starts closed"""
        breaker = CircuitBreaker("test_service")
        assert breaker.state == CircuitState.CLOSED
        assert breaker.is_closed is True
        assert breaker.is_open is False

    def test_circuit_breaker_opens_after_failures(self):
        """Test circuit opens after threshold failures"""
        breaker = CircuitBreaker("test_service", failure_threshold=3)

        @breaker
        def failing_function():
            raise Exception("Service error")

        # Fail 3 times
        for i in range(3):
            try:
                failing_function()
            except Exception:
                pass

        # Circuit should be open now
        assert breaker.state == CircuitState.OPEN
        assert breaker.is_open is True

        # Next call should fail fast
        with pytest.raises(CircuitBreakerError):
            failing_function()

        print(f"Circuit breaker opened after {breaker.config.failure_threshold} failures")

    def test_circuit_breaker_recovery(self):
        """Test circuit breaker recovery after timeout"""
        breaker = CircuitBreaker(
            "test_recovery",
            failure_threshold=2,
            recovery_timeout=0.1,  # 100ms for testing
            success_threshold=1,
        )

        call_count = 0

        @breaker
        def sometimes_fails():
            nonlocal call_count
            call_count += 1
            if call_count <= 2:
                raise Exception("Fail")
            return "success"

        # Trigger failures to open circuit
        for _ in range(2):
            try:
                sometimes_fails()
            except Exception:
                pass

        assert breaker.is_open is True

        # Wait for recovery timeout
        import time
        time.sleep(0.15)

        # Next call should be allowed (half-open)
        result = sometimes_fails()
        assert result == "success"
        assert breaker.is_closed is True

        print("Circuit breaker recovered successfully")

    def test_circuit_breaker_status(self):
        """Test circuit breaker status reporting"""
        breaker = get_circuit_breaker("status_test")

        status = breaker.get_status()

        assert status["name"] == "status_test"
        assert status["state"] == "closed"
        assert "config" in status
        assert status["config"]["failure_threshold"] == 5

        print(f"Status: {json.dumps(status, indent=2, default=str)}")

    def test_context_manager_usage(self):
        """Test circuit breaker as context manager"""
        breaker = CircuitBreaker("context_test", failure_threshold=2)

        # Successful call
        with breaker:
            result = "success"
        assert result == "success"

        # Failed calls
        for _ in range(2):
            try:
                with breaker:
                    raise ValueError("Error")
            except ValueError:
                pass

        # Circuit should be open
        with pytest.raises(CircuitBreakerError):
            with breaker:
                pass


# ==========================================
# 4. TRAVEL BOOKING FLOW TESTS (MOCKED)
# ==========================================

class TestTravelBookingFlow:
    """Test travel booking flow with mocked Amadeus"""

    @pytest.fixture
    def mock_amadeus_response(self):
        """Mock Amadeus flight search response"""
        return {
            "data": [{
                "id": "1",
                "price": {"total": "450.00", "currency": "USD"},
                "itineraries": [{
                    "duration": "PT5H30M",
                    "segments": [{
                        "departure": {"iataCode": "JFK", "at": "2024-03-15T08:00:00"},
                        "arrival": {"iataCode": "LAX", "at": "2024-03-15T11:30:00"},
                        "carrierCode": "AA",
                        "number": "123",
                        "duration": "PT5H30M",
                    }]
                }],
                "validatingAirlineCodes": ["AA"],
                "travelerPricings": [],
            }]
        }

    @patch("app.services.amadeus_service.Client")
    def test_flight_search_with_circuit_breaker(self, mock_client, mock_amadeus_response):
        """Test flight search uses circuit breaker correctly"""
        from app.services.amadeus_service import AmadeusService
        from app.services.circuit_breaker import amadeus_breaker

        # Reset circuit breaker
        amadeus_breaker.reset()

        # Setup mock
        mock_instance = Mock()
        mock_instance.shopping.flight_offers_search.get.return_value = Mock(
            data=mock_amadeus_response["data"]
        )
        mock_client.return_value = mock_instance

        # Call service
        with patch.dict(os.environ, {
            "AMADEUS_API_KEY": "test_key",
            "AMADEUS_API_SECRET": "test_secret"
        }):
            settings.AMADEUS_API_KEY = "test_key"
            settings.AMADEUS_API_SECRET = "test_secret"
            service = AmadeusService()
            offers = service.search_flights(
                origin="JFK",
                destination="LAX",
                departure_date="2024-03-15",
                adults=1,
            )

        assert len(offers) == 1
        assert offers[0]["price"] == 450.00
        assert amadeus_breaker.is_closed is True

        print("Flight search completed with circuit breaker protection")

    @patch("app.services.amadeus_service.Client")
    def test_flight_search_circuit_breaker_opens(self, mock_client):
        """Test circuit breaker opens on repeated failures"""
        from app.services.amadeus_service import AmadeusService
        from app.services.circuit_breaker import amadeus_breaker
        from amadeus import ResponseError

        # Reset circuit breaker
        amadeus_breaker.reset()

        # Setup mock to fail
        mock_instance = Mock()
        mock_instance.shopping.flight_offers_search.get.side_effect = ResponseError(
            Mock(status_code=500, result={"errors": [{"detail": "Server error"}]})
        )
        mock_client.return_value = mock_instance

        with patch.dict(os.environ, {
            "AMADEUS_API_KEY": "test_key",
            "AMADEUS_API_SECRET": "test_secret"
        }):
            settings.AMADEUS_API_KEY = "test_key"
            settings.AMADEUS_API_SECRET = "test_secret"
            service = AmadeusService()

            # Trigger failures
            for _ in range(5):
                try:
                    service.search_flights(
                        origin="JFK",
                        destination="LAX",
                        departure_date="2024-03-15",
                    )
                except ValueError:
                    pass

        assert amadeus_breaker.is_open is True
        print("Circuit breaker opened after repeated failures")


# ==========================================
# 5. INTEGRATION TESTS
# ==========================================

class TestSecurityIntegration:
    """Integration tests for security features"""

    def test_encrypted_traveler_profile_storage(self, db):
        """Test that traveler profiles are stored encrypted"""
        user_id = f"enc_profile_{uuid.uuid4().hex[:8]}"

        user = User(id=user_id)
        db.add(user)

        # Create profile with encrypted PII
        profile = TravelerProfile(
            user_id=user_id,
            first_name="Secure",
            last_name="User",
            date_of_birth=encrypt_pii("1990-06-15"),
            gender="MALE",
            passport_number=encrypt_pii("AB1234567"),
            email=encrypt_pii("secure@example.com"),
            phone=encrypt_pii("+1234567890"),
        )
        db.add(profile)
        db.commit()
        db.refresh(profile)

        # Verify stored values are encrypted
        assert is_encrypted(profile.date_of_birth)
        assert is_encrypted(profile.passport_number)
        assert is_encrypted(profile.email)
        assert is_encrypted(profile.phone)

        # Verify decryption works
        assert decrypt_pii(profile.date_of_birth) == "1990-06-15"
        assert decrypt_pii(profile.passport_number) == "AB1234567"

        print("Traveler profile stored with encrypted PII")

        # Clean up
        delete_user_data(db, user_id, dry_run=False)

    def test_full_security_flow(self, db):
        """Test complete security flow: create, encrypt, export, delete"""
        user_id = f"full_flow_{uuid.uuid4().hex[:8]}"

        # 1. Create user
        user = User(id=user_id)
        db.add(user)

        # 2. Create encrypted profile
        profile = TravelerProfile(
            user_id=user_id,
            first_name="Flow",
            last_name="Test",
            date_of_birth=encrypt_pii("1985-12-25"),
            gender="FEMALE",
            passport_number=encrypt_pii("XY9876543"),
        )
        db.add(profile)
        db.commit()

        # 3. Export data
        export = export_user_data(db, user_id)
        assert "traveler_profiles" in export["data"]
        print(f"Step 1-3: Created and exported user {user_id}")

        # 4. Verify encryption in export
        exported_profile = export["data"]["traveler_profiles"][0]
        assert is_encrypted(exported_profile["passport_number"])
        print("Step 4: Verified encryption in export")

        # 5. Delete data
        delete_result = delete_user_data(db, user_id, dry_run=False)
        assert delete_result["success"] is True
        print("Step 5: Deleted user data")

        # 6. Verify deletion
        assert db.query(User).filter(User.id == user_id).first() is None
        assert db.query(TravelerProfile).filter(TravelerProfile.user_id == user_id).first() is None
        print("Step 6: Verified complete deletion")

        print("\nFull security flow completed successfully!")


# ==========================================
# TEST RUNNER
# ==========================================

if __name__ == "__main__":
    print("\n" + "=" * 60)
    print("STAGE 17: SECURITY TESTING SUITE")
    print("=" * 60)

    pytest.main([__file__, "-v", "--tb=short"])
