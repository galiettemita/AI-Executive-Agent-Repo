"""
Tests for Stage 6: Creative Studio + Wardrobe features
"""
import uuid
from fastapi.testclient import TestClient
from app.main import app
from app.services.intent import classify_intent, Intent
from app.services.preferences import (
    handle_wardrobe_onboarding_step,
    is_wardrobe_onboarding_complete,
)


# ===== INTENT CLASSIFICATION TESTS =====

def test_classify_wardrobe_intent():
    """Test that wardrobe-related messages are classified correctly"""
    assert classify_intent("what should I wear to a wedding?") == Intent.WARDROBE
    assert classify_intent("help me pick an outfit") == Intent.WARDROBE
    assert classify_intent("what outfit should I choose?") == Intent.WARDROBE
    assert classify_intent("what shoes go with this jacket?") == Intent.WARDROBE
    assert classify_intent("build me a capsule wardrobe") == Intent.WARDROBE


def test_classify_creative_intent():
    """Test that creative-related messages are classified correctly"""
    assert classify_intent("design a logo for my startup") == Intent.CREATIVE
    assert classify_intent("help me write an Instagram caption") == Intent.CREATIVE
    assert classify_intent("need a flyer for my event") == Intent.CREATIVE
    assert classify_intent("brand guidelines for my business") == Intent.CREATIVE
    assert classify_intent("website design direction") == Intent.CREATIVE


# ===== WARDROBE ONBOARDING TESTS =====

def test_wardrobe_onboarding_flow():
    """Test the complete wardrobe onboarding flow"""
    prefs = {}

    # Should not be complete initially
    assert not is_wardrobe_onboarding_complete(prefs)

    # Step 0: Start onboarding (trigger with any message)
    reply, prefs = handle_wardrobe_onboarding_step("I need wardrobe help", prefs)
    assert "vibe" in reply.lower() or "style" in reply.lower()
    assert prefs.get("wardrobe_onboarding_step") == 1

    # Step 1: Answer vibe question
    reply, prefs = handle_wardrobe_onboarding_step("streetwear", prefs)
    assert "sizes" in reply.lower()
    assert prefs.get("wardrobe_vibe") == "streetwear"
    assert prefs.get("wardrobe_onboarding_step") == 2

    # Step 2: Answer sizes question
    reply, prefs = handle_wardrobe_onboarding_step("shirt L, pants 34x32, shoes 11", prefs)
    assert "color" in reply.lower()
    assert prefs.get("wardrobe_sizes") == "shirt L, pants 34x32, shoes 11"
    assert prefs.get("wardrobe_onboarding_step") == 3

    # Step 3: Answer colors question
    reply, prefs = handle_wardrobe_onboarding_step("love earth tones, avoid bright colors", prefs)
    assert "budget" in reply.lower()
    assert prefs.get("wardrobe_colors") == "love earth tones, avoid bright colors"
    assert prefs.get("wardrobe_onboarding_step") == 4

    # Step 4: Answer budget question (final step)
    reply, prefs = handle_wardrobe_onboarding_step("$75-$200 per piece", prefs)
    assert "set" in reply.lower() or "ready" in reply.lower() or "all set" in reply.lower()
    assert prefs.get("wardrobe_budget") == "$75-$200 per piece"
    assert prefs.get("wardrobe_onboarding_complete") is True

    # Should now be complete
    assert is_wardrobe_onboarding_complete(prefs)


def test_wardrobe_onboarding_starts_automatically():
    """Test that wardrobe onboarding starts on first wardrobe intent"""
    prefs = {}

    # First call should start onboarding
    reply, prefs = handle_wardrobe_onboarding_step("help me with outfits", prefs)
    assert "vibe" in reply.lower() or "style" in reply.lower()
    assert prefs.get("wardrobe_onboarding_step") == 1


# ===== INTEGRATION TESTS =====

def test_wardrobe_onboarding_via_api():
    """Test wardrobe onboarding through the API"""
    client = TestClient(app)
    user_id = f"test_wardrobe_{uuid.uuid4().hex[:8]}"

    # Complete main onboarding first
    client.post("/agent/chat", json={"user_id": user_id, "message": "hi"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "minimal"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "$50-$100"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "evenings"})

    # First wardrobe message should trigger wardrobe onboarding
    resp = client.post(
        "/agent/chat",
        json={"user_id": user_id, "message": "what should I wear?"}
    )
    assert resp.status_code == 200
    data = resp.json()
    reply = data.get("reply", "")
    assert "vibe" in reply.lower() or "style" in reply.lower()

    # Complete wardrobe onboarding
    resp = client.post("/agent/chat", json={"user_id": user_id, "message": "classic"})
    resp = client.post("/agent/chat", json={"user_id": user_id, "message": "shirt M, pants 32x30, shoes 10"})
    resp = client.post("/agent/chat", json={"user_id": user_id, "message": "love navy, avoid neon"})
    resp = client.post("/agent/chat", json={"user_id": user_id, "message": "$100-$300"})

    # Final message should confirm wardrobe onboarding is complete
    assert resp.status_code == 200
    data = resp.json()
    reply = data.get("reply", "")
    # Should contain confirmation message from wardrobe onboarding completion
    assert ("set" in reply.lower() or "occasion" in reply.lower() or "help" in reply.lower())


def test_creative_mode_via_api():
    """Test creative agent through the API"""
    client = TestClient(app)
    user_id = f"test_creative_{uuid.uuid4().hex[:8]}"

    # Complete main onboarding
    client.post("/agent/chat", json={"user_id": user_id, "message": "hello"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "modern"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "$100"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "anytime"})

    # Creative request
    resp = client.post(
        "/agent/chat",
        json={"user_id": user_id, "message": "help me design a logo for my coffee shop"}
    )

    assert resp.status_code == 200
    data = resp.json()
    reply = data.get("reply", "")

    # Creative agent should respond (not empty)
    assert len(reply) > 0
    # Should ask clarifying questions or provide guidance
    assert any(word in reply.lower() for word in ["logo", "design", "coffee", "what", "vibe", "style", "color"])


def test_wardrobe_agent_responds_after_onboarding():
    """Test that wardrobe agent provides outfit advice after onboarding"""
    client = TestClient(app)
    user_id = f"test_wardrobe_{uuid.uuid4().hex[:8]}"

    # Complete both onboardings
    # Main onboarding
    client.post("/agent/chat", json={"user_id": user_id, "message": "hi"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "casual"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "$50-$150"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "flexible"})

    # Wardrobe onboarding
    client.post("/agent/chat", json={"user_id": user_id, "message": "outfit help"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "streetwear"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "M, 32x30, 10"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "dark colors"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "mid-range"})

    # Now ask for outfit advice
    resp = client.post(
        "/agent/chat",
        json={"user_id": user_id, "message": "what should I wear to a casual dinner?"}
    )

    assert resp.status_code == 200
    data = resp.json()
    reply = data.get("reply", "")

    # Should get outfit advice
    assert len(reply) > 0


def test_wardrobe_proposal_not_created_without_shopping_intent():
    """Test that wardrobe proposals aren't created unless user wants to shop"""
    client = TestClient(app)
    user_id = f"test_wardrobe_{uuid.uuid4().hex[:8]}"

    # Complete onboardings (abbreviated for test)
    client.post("/agent/chat", json={"user_id": user_id, "message": "start"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "minimal"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "$100"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "any"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "style advice"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "classic"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "L, 34, 11"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "neutral"})
    client.post("/agent/chat", json={"user_id": user_id, "message": "flexible"})

    # Ask for outfit advice (no shopping)
    resp = client.post(
        "/agent/chat",
        json={"user_id": user_id, "message": "what should I wear today?"}
    )

    assert resp.status_code == 200
    data = resp.json()
    reply = data.get("reply", "")

    # Should NOT contain proposal link
    assert "proposals/" not in reply
    assert "Approve:" not in reply


# ===== HELPER FUNCTION TESTS =====

def test_preferences_functions_exist():
    """Verify all preference functions are available"""
    from app.services.preferences import (
        get_preferences,
        update_preferences,
        is_onboarding_complete,
        is_wardrobe_onboarding_complete,
        handle_onboarding_step,
        handle_wardrobe_onboarding_step,
    )
    # If we got here without ImportError, functions exist
    assert True


def test_agent_functions_exist():
    """Verify all agent functions are available"""
    from app.services.wardrobe_agent import run_wardrobe_agent
    from app.services.creative_agent import run_creative_agent
    # If we got here without ImportError, functions exist
    assert True
