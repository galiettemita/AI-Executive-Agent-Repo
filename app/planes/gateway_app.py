from __future__ import annotations

"""
Gateway Plane entrypoint.

For now this reuses the existing FastAPI app wiring from `app.main` so we don't
break any current external endpoints while we incrementally move functionality
behind Brain/Hands internal APIs.
"""

from dotenv import load_dotenv

# Load env FIRST (before importing modules that read env vars)
load_dotenv()

from app.main import app as app  # noqa: E402

# Make it obvious in docs/telemetry which plane is serving traffic.
app.title = "Executive OS — Gateway"

