import os
import pytest
from openai import OpenAI

# Only run live OpenAI test when explicitly requested
if os.getenv("RUN_LIVE_TESTS") != "1":
    pytest.skip("Set RUN_LIVE_TESTS=1 to run live OpenAI test.", allow_module_level=True)

OPENAI_API_KEY = os.getenv("OPENAI_API_KEY")
if not OPENAI_API_KEY:
    pytest.skip("OPENAI_API_KEY not set; skipping live OpenAI test.", allow_module_level=True)

client = OpenAI(api_key=OPENAI_API_KEY)


def test_openai_api():
    resp = client.responses.create(
        model="gpt-4.1-mini",
        input="say 'api working' ."
    )

    assert resp.output_text
