import os
from pathlib import Path


def _setenv_once(key: str, value: str) -> None:
    if not os.getenv(key):
        os.environ[key] = value


# Ensure a writable SQLite DB for tests (avoid stale local .env paths)
TEST_DB_PATH = Path("/tmp/executive_ai_agent_test.db")
if TEST_DB_PATH.exists():
    try:
        TEST_DB_PATH.unlink()
    except Exception:
        pass

_setenv_once("DATABASE_URL", f"sqlite:///{TEST_DB_PATH}")
_setenv_once("ENV", "dev")
_setenv_once("JWT_SECRET", "test_jwt_secret_for_testing_only")
_setenv_once("PII_ENCRYPTION_KEY", "dEkqDdDoZOe8lnV0fi9-SWicfi8UNtMvYnrqYH50mPU=")
_setenv_once("STATE_SIGNING_SECRET", "test_state_signing_secret")
_setenv_once("OPENAI_API_KEY", "test_key")
_setenv_once("WARDROBE_LLM_ENABLED", "0")
_setenv_once("STRIPE_SECRET_KEY", "sk_test_dummy")
_setenv_once("STRIPE_PRICE_ID_STARTER", "price_dummy")
_setenv_once("CHECKOUT_SUCCESS_URL", "https://example.com/success")
_setenv_once("CHECKOUT_CANCEL_URL", "https://example.com/cancel")


def _fake_agent_response(message: str) -> dict:
    return {"assistant_message": message}


def _install_agent_stubs():
    from app.services import orchestrator

    def fake_run_agent(*args, **kwargs):
        return _fake_agent_response("Here are a few options to consider.")

    def fake_run_creative_agent(*args, **kwargs):
        return _fake_agent_response("Logo design ideas for your coffee shop: try a minimalist mark with warm colors.")

    def fake_run_wardrobe_agent(*args, **kwargs):
        return _fake_agent_response("Outfit idea: neutral top, dark jeans, and clean sneakers.")

    def fake_run_travel_agent(*args, **kwargs):
        return _fake_agent_response("Flight to Miami: what dates are you thinking?")

    def fake_run_food_agent(*args, **kwargs):
        return _fake_agent_response("Pizza delivery sounds good. Any dietary preferences?")

    orchestrator.run_agent = fake_run_agent
    orchestrator.run_creative_agent = fake_run_creative_agent
    orchestrator.run_wardrobe_agent = fake_run_wardrobe_agent
    orchestrator.run_travel_agent = fake_run_travel_agent
    orchestrator.run_food_agent = fake_run_food_agent


_install_agent_stubs()


def _ensure_tables():
    from app.db.database import Base, engine
    Base.metadata.create_all(bind=engine)


_ensure_tables()
