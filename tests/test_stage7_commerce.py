"""
Tests for Stage 7: Commerce Proposals (Food/Travel)
"""
import uuid
from fastapi.testclient import TestClient
from app.main import app
from app.services.intent import classify_intent, Intent


# ===== INTENT CLASSIFICATION TESTS =====

def test_classify_food_intent():
    """Test that food-related messages are classified correctly"""
    assert classify_intent("order pizza") == Intent.FOOD
    assert classify_intent("get me some sushi delivery") == Intent.FOOD
    assert classify_intent("pickup food from chipotle") == Intent.FOOD
    assert classify_intent("I'm hungry, what should I eat") == Intent.FOOD


def test_classify_travel_intent():
    """Test that travel-related messages are classified correctly"""
    assert classify_intent("book a flight to New York") == Intent.TRAVEL
    assert classify_intent("find me a hotel in Paris") == Intent.TRAVEL
    assert classify_intent("plan my trip to Japan") == Intent.TRAVEL
    assert classify_intent("need to rent a car") == Intent.TRAVEL


# ===== INTEGRATION TESTS =====

def test_food_agent_via_api():
    """Test food agent through the API"""
    client = TestClient(app)
    user_id = f"test_food_{uuid.uuid4().hex[:8]}"

    # Complete main onboarding
    client.post("/agent/chat", json={"user_id": user_id, "message": "hello"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "casual"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "$50"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "anytime"})

    # Food request
    resp = client.post(
        "/agent/chat",
        json={"user_id": user_id, "message": "I want to order pizza for delivery"}
    )

    assert resp.status_code == 200
    data = resp.json()
    reply = data.get("reply", "")

    # Should get food agent response OR upgrade prompt (since FOOD is premium)
    assert len(reply) > 0
    # Should either ask clarifying questions/provide suggestions OR prompt to upgrade
    assert any(word in reply.lower() for word in ["pizza", "delivery", "what", "where", "cuisine", "restaurant", "upgrade", "billing", "premium"])


def test_travel_agent_via_api():
    """Test travel agent through the API"""
    client = TestClient(app)
    user_id = f"test_travel_{uuid.uuid4().hex[:8]}"

    # Complete main onboarding
    client.post("/agent/chat", json={"user_id": user_id, "message": "hi"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "modern"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "$200"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "flexible"})

    # Travel request
    resp = client.post(
        "/agent/chat",
        json={"user_id": user_id, "message": "help me book a flight to Miami"}
    )

    assert resp.status_code == 200
    data = resp.json()
    reply = data.get("reply", "")

    # Should get travel agent response
    assert len(reply) > 0
    # Should ask clarifying questions or provide suggestions
    assert any(word in reply.lower() for word in ["flight", "miami", "dates", "when", "travel", "airline"])


def test_food_agent_no_proposal_without_confirmation():
    """Test that food proposals aren't created without order confirmation"""
    client = TestClient(app)
    user_id = f"test_food_{uuid.uuid4().hex[:8]}"

    # Complete onboarding
    client.post("/agent/chat", json={"user_id": user_id, "message": "start"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "minimal"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "$100"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "any"})

    # Ask about food (no confirmation to order)
    resp = client.post(
        "/agent/chat",
        json={"user_id": user_id, "message": "what's good for lunch?"}
    )

    assert resp.status_code == 200
    data = resp.json()
    reply = data.get("reply", "")

    # Should NOT contain proposal link
    assert "proposals/" not in reply
    assert "Approve:" not in reply


def test_travel_agent_no_proposal_without_confirmation():
    """Test that travel proposals aren't created without booking confirmation"""
    client = TestClient(app)
    user_id = f"test_travel_{uuid.uuid4().hex[:8]}"

    # Complete onboarding
    client.post("/agent/chat", json={"user_id": user_id, "message": "begin"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "casual"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "$150"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "evenings"})

    # Ask about travel (no confirmation to book)
    resp = client.post(
        "/agent/chat",
        json={"user_id": user_id, "message": "tell me about Hawaii"}
    )

    assert resp.status_code == 200
    data = resp.json()
    reply = data.get("reply", "")

    # Should NOT contain proposal link
    assert "proposals/" not in reply
    assert "Approve:" not in reply


# ===== HELPER FUNCTION TESTS =====

def test_food_agent_functions_exist():
    """Verify food agent functions are available"""
    from app.services.food_agent import run_food_agent
    assert True


def test_travel_agent_functions_exist():
    """Verify travel agent functions are available"""
    from app.services.travel_agent import run_travel_agent
    assert True


def test_shopping_agent_supports_cart_proposals():
    """Test that shopping agent can create purchase cart proposals"""
    from app.services.agent import run_agent
    from sqlalchemy.orm import Session
    
    # This is a unit test to verify the agent function exists and returns expected structure
    # Full integration test would require mocking OpenAI and DB
    assert callable(run_agent)
    
    # Verify the function signature
    import inspect
    sig = inspect.signature(run_agent)
    params = list(sig.parameters.keys())
    assert 'db' in params
    assert 'user_id' in params
    assert 'history' in params
    assert 'user_message' in params
